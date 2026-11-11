package workbench

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

var deploymentDomain = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?$`)
var deploymentName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// ExecuteDeploymentScript sends the script over the authenticated SSH channel.
// Command output can contain application secrets and is deliberately not persisted.
func ExecuteDeploymentScript(ctx context.Context, resource DeploymentResource, key []byte, script string) error {
	return executeDeploymentSSH(ctx, resource, key, script, io.Discard)
}

func executeDeploymentSSH(ctx context.Context, resource DeploymentResource, key []byte, script string, output io.Writer) error {
	return runDeploymentSSH(ctx, resource, key, "sh -se", strings.NewReader(script), output)
}

func runDeploymentSSH(ctx context.Context, resource DeploymentResource, key []byte, command string, input io.Reader, output io.Writer) error {
	var authentication ssh.AuthMethod
	switch resource.AuthMethod {
	case "password":
		authentication = ssh.Password(string(key))
	case "", "key":
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return errors.New("SSH private key cannot be read")
		}
		authentication = ssh.PublicKeys(signer)
	default:
		return errors.New("unsupported SSH login method")
	}
	address := net.JoinHostPort(resource.Host, resource.Port)
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return errors.New("SSH server is unreachable")
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	clientConfig := &ssh.ClientConfig{User: resource.Username, Auth: []ssh.AuthMethod{authentication}, HostKeyCallback: func(_ string, _ net.Addr, publicKey ssh.PublicKey) error {
		if ssh.FingerprintSHA256(publicKey) != resource.HostKey {
			return errors.New("SSH host fingerprint mismatch")
		}
		return nil
	}}
	clientConnection, channels, requests, err := ssh.NewClientConn(connection, address, clientConfig)
	if err != nil {
		return errors.New("SSH authentication or host fingerprint verification failed")
	}
	client := ssh.NewClient(clientConnection, channels, requests)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return errors.New("SSH session could not be opened")
	}
	defer session.Close()
	session.Stdin = input
	session.Stdout = output
	session.Stderr = io.Discard
	if err = session.Run(command); err != nil {
		var status *ssh.ExitError
		if errors.As(err, &status) {
			return fmt.Errorf("remote command exited with status %d; inspect the deployment server", status.ExitStatus())
		}
		return errors.New("SSH connection interrupted; verify the server state before retrying")
	}
	return nil
}

// CheckApplicationDeploymentTools reports missing executables before changing the target.
func CheckApplicationDeploymentTools(ctx context.Context, resource DeploymentResource, key []byte, app DeploymentApplication, config ApplicationDeployment, change ReleaseChange) error {
	tools := []string{"sh", "flock"}
	if change.Type == "sql" {
		tools = append(tools, "psql", "base64")
	}
	if change.Type == "config" {
		tools = append(tools, "base64")
	}
	if change.Type == "app" {
		tools = append(tools, "curl")
		if app.SourceKind == "git" {
			tools = append(tools, "tar", "sha256sum")
		}
		if config.Kind == "nginx" || config.Kind == "docker" {
			tools = append(tools, "nginx", "base64")
		}
		if config.Kind == "docker" {
			tools = append(tools, "docker")
		}
		if config.HTTPS == "managed" {
			tools = append(tools, "certbot")
		}
	}
	output := &deploymentOutput{}
	script := "for tool in " + strings.Join(tools, " ") + "; do command -v \"$tool\" >/dev/null 2>&1 || printf '%s\\n' \"$tool\"; done\n"
	if err := executeDeploymentSSH(ctx, resource, key, script, output); err != nil {
		return err
	}
	missing := strings.Fields(output.String())
	if len(missing) > 0 {
		return fmt.Errorf("deployment server is missing required tools: %s", strings.Join(missing, ", "))
	}
	return nil
}

type deploymentOutput struct{ bytes.Buffer }

func (output *deploymentOutput) Write(value []byte) (int, error) {
	length := len(value)
	remaining := 4096 - output.Len()
	if remaining > 0 {
		_, _ = output.Buffer.Write(value[:min(remaining, length)])
	}
	return length, nil
}

func ResolveApplicationVersion(ctx context.Context, resource DeploymentResource, key []byte, app DeploymentApplication, version string) (string, error) {
	if app.SourceKind == "directory" {
		return version, nil
	}
	if app.SourceKind == "git" && regexp.MustCompile(`^[a-fA-F0-9]{40}$`).MatchString(version) {
		return strings.ToLower(version), nil
	}
	if app.SourceKind == "image" && regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(version) {
		return version, nil
	}
	var script string
	if app.SourceKind == "image" {
		image := app.Source + ":" + version
		if strings.HasPrefix(image, "-") || strings.ContainsAny(image, " \r\n\t") {
			return "", errors.New("invalid image version")
		}
		script = "set -e\ndocker pull " + sshShellQuote(image) + " >/dev/null\ndocker image inspect --format '{{json .RepoDigests}}' " + sshShellQuote(image)
	} else if app.SourceKind != "git" {
		return "", errors.New("unsupported application source")
	}
	output := &deploymentOutput{}
	if app.SourceKind == "git" {
		refs := []string{"refs/tags/" + version, "refs/tags/" + version + "^{}", "refs/heads/" + version}
		if strings.HasPrefix(version, "refs/") {
			refs = []string{version, version + "^{}"}
		}
		raw, err := localReleaseGit(ctx, "", append([]string{"ls-remote", "--exit-code", "--", app.Source}, refs...)...)
		if err != nil {
			return "", errors.New("could not resolve the Git version locally; verify the branch or tag and repository credentials on the build machine")
		}
		_, _ = output.Write(raw)
	} else if err := executeDeploymentSSH(ctx, resource, key, script, output); err != nil {
		return "", err
	}
	if app.SourceKind == "image" {
		var digests []string
		if json.Unmarshal(output.Bytes(), &digests) != nil || len(digests) == 0 {
			return "", errors.New("registry did not return an immutable image digest")
		}
		_, digest, found := strings.Cut(digests[0], "@")
		if !found || !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(digest) {
			return "", errors.New("invalid registry digest")
		}
		return digest, nil
	}
	resolved := ""
	for _, line := range strings.Split(output.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if !regexp.MustCompile(`^[a-fA-F0-9]{40}$`).MatchString(fields[0]) {
			return "", errors.New("invalid repository commit")
		}
		if strings.HasSuffix(fields[1], "^{}") {
			return fields[0], nil
		}
		if resolved != "" && resolved != fields[0] {
			return "", errors.New("ambiguous version; use a fully qualified repository ref")
		}
		resolved = fields[0]
	}
	if resolved == "" {
		return "", errors.New("repository version was not found")
	}
	return resolved, nil
}

func deploymentWriteFile(file, content string) string {
	return "printf '%s' " + sshShellQuote(base64.StdEncoding.EncodeToString([]byte(content))) + " | base64 -d > " + sshShellQuote(file) + "\n"
}

func ApplicationDeploymentScript(releaseID string, app DeploymentApplication, config ApplicationDeployment, change ReleaseChange, certificate *CertificateUpload) (string, error) {
	if change.ResolvedVersion != "" {
		change.Version = change.ResolvedVersion
	}
	if !validDeploymentID(releaseID) || !validDeploymentID(app.ID) || !validDeploymentID(change.ID) {
		return "", errors.New("invalid deployment identity")
	}
	if !path.IsAbs(config.Directory) || path.Clean(config.Directory) == "/" {
		return "", errors.New("an absolute non-root deployment directory is required")
	}
	if !deploymentDomain.MatchString(config.Domain) {
		return "", errors.New("a valid domain is required")
	}
	if !strings.HasPrefix(config.HealthPath, "/") || strings.ContainsAny(config.HealthPath, "\r\n\x00") {
		return "", errors.New("health path must start with a slash")
	}
	var script strings.Builder
	script.WriteString("set -eu\numask 077\ncommand -v flock >/dev/null\nmkdir -p " + sshShellQuote(config.Directory) + "\nexec 9>" + sshShellQuote(path.Join(config.Directory, ".multica.lock")) + "\nflock -n 9\n")
	if change.Type == "sql" {
		if !deploymentName.MatchString(change.Database) || !change.Rehearsed || strings.TrimSpace(change.Body) == "" {
			return "", errors.New("SQL requires a PostgreSQL service name and rehearsal confirmation")
		}
		script.WriteString("command -v psql >/dev/null\n")
		script.WriteString("printf '%s' " + sshShellQuote(base64.StdEncoding.EncodeToString([]byte(change.Body))) + " | base64 -d | psql " + sshShellQuote("service="+change.Database) + " -X --set ON_ERROR_STOP=1 --single-transaction -f -\n")
		return script.String(), nil
	}
	if change.Type == "config" {
		if !path.IsAbs(change.Path) || strings.TrimSpace(change.Body) == "" {
			return "", errors.New("configuration content and absolute destination are required")
		}
		temporary := change.Path + ".multica-" + releaseID
		script.WriteString("test -d " + sshShellQuote(path.Dir(change.Path)) + "\n")
		script.WriteString(deploymentWriteFile(temporary, change.Body))
		script.WriteString("chmod 600 " + sshShellQuote(temporary) + "\nmv -f " + sshShellQuote(temporary) + " " + sshShellQuote(change.Path) + "\n")
		return script.String(), nil
	}
	if strings.TrimSpace(change.Version) == "" {
		return "", errors.New("application version is required")
	}
	script.WriteString("command -v curl >/dev/null\n")
	root := path.Join(config.Directory, "releases", releaseID)
	if app.SourceKind != "image" {
		script.WriteString("mkdir -p " + sshShellQuote(path.Dir(root)) + "\n")
		if app.SourceKind == "git" {
			script.WriteString("test -d " + sshShellQuote(root) + "\ntest -f " + sshShellQuote(path.Join(root, ".multica-artifact")) + "\ncd " + sshShellQuote(root) + "\n")
			if change.ArtifactDigest != "" {
				script.WriteString("test \"$(cat .multica-artifact)\" = " + sshShellQuote(change.ArtifactDigest) + "\n")
			}
		} else if app.SourceKind == "directory" {
			source := path.Clean(app.Source)
			if !path.IsAbs(source) || source == "/" || source == path.Clean(config.Directory) || strings.HasPrefix(path.Clean(config.Directory), source+"/") {
				return "", errors.New("source must be an absolute directory separate from deployment output")
			}
			script.WriteString("if [ ! -d " + sshShellQuote(root) + " ]; then\nstaging=$(mktemp -d " + sshShellQuote(root+".XXXXXX") + ")\ntrap 'rm -rf -- \"$staging\"' EXIT\ncp -a " + sshShellQuote(source+"/.") + " \"$staging/\"\nmv -- \"$staging\" " + sshShellQuote(root) + "\ntrap - EXIT\nfi\ncd " + sshShellQuote(root) + "\n")
		} else {
			return "", errors.New("unsupported source")
		}
		if app.SourceKind != "git" && app.BuildCommand != "" {
			script.WriteString("sh -ec " + sshShellQuote(app.BuildCommand) + "\n")
		}
	}
	artifactPath := path.Clean(app.ArtifactPath)
	if app.SourceKind == "git" {
		artifactPath = "."
	}
	if artifactPath == "" {
		artifactPath = "."
	}
	if path.IsAbs(artifactPath) || artifactPath == ".." || strings.HasPrefix(artifactPath, "../") {
		return "", errors.New("artifact path must stay inside the source")
	}
	if config.Kind == "nginx" {
		if app.SourceKind == "image" {
			return "", errors.New("static websites require a repository or directory source")
		}
		script.WriteString("test -d " + sshShellQuote(path.Join(root, artifactPath)) + "\nchmod o+x " + sshShellQuote(config.Directory) + " " + sshShellQuote(path.Dir(root)) + "\nchmod -R a+rX " + sshShellQuote(path.Join(root, artifactPath)) + "\n")
		for directory := path.Dir(path.Join(root, artifactPath)); strings.HasPrefix(directory, root); directory = path.Dir(directory) {
			script.WriteString("chmod a+x " + sshShellQuote(directory) + "\n")
		}
	} else if config.Kind == "docker" {
		port, err := strconv.Atoi(config.Port)
		if err != nil || port < 1 || port > 65535 {
			return "", errors.New("container port must be between 1 and 65535")
		}
		if app.SourceKind != "image" {
			return "", errors.New("Docker deployment requires an existing image source")
		}
		image := app.Source + ":" + change.Version
		if strings.HasPrefix(change.Version, "sha256:") {
			image = app.Source + "@" + change.Version
		}
		if strings.ContainsAny(image, " \t\r\n\x00") || strings.HasPrefix(image, "-") {
			return "", errors.New("invalid container image")
		}
		container := "multica-" + app.ID
		script.WriteString("command -v docker >/dev/null\nmkdir -p " + sshShellQuote(root) + "\n")
		imageFile := path.Join(root, ".multica-image")
		script.WriteString("if [ ! -f " + sshShellQuote(imageFile) + " ]; then docker pull " + sshShellQuote(image) + "; docker image inspect --format '{{.Id}}' " + sshShellQuote(image) + " > " + sshShellQuote(imageFile) + "; fi\n")
		script.WriteString("if docker container inspect " + sshShellQuote(container) + " >/dev/null 2>&1; then docker stop " + sshShellQuote(container) + "; docker rm " + sshShellQuote(container) + "; fi\n")
		script.WriteString("docker run -d --restart unless-stopped --name " + sshShellQuote(container) + " -p " + sshShellQuote("127.0.0.1:"+config.Port+":"+config.Port) + " \"$(cat " + sshShellQuote(imageFile) + ")\"\n")
	} else if config.Kind == "process" || config.Kind == "kubernetes" {
		if strings.TrimSpace(config.Command) == "" {
			return "", errors.New("configure the application's start or upgrade command")
		}
		script.WriteString("export MULTICA_VERSION=" + sshShellQuote(change.Version) + "\nexport MULTICA_SOURCE=" + sshShellQuote(app.Source) + "\nexport MULTICA_NAMESPACE=" + sshShellQuote(config.Namespace) + "\n")
		if app.SourceKind != "image" {
			script.WriteString("cd " + sshShellQuote(root) + "\n")
		}
		script.WriteString("sh -ec " + sshShellQuote(config.Command) + "\n")
	} else {
		return "", errors.New("unsupported deployment method")
	}
	if config.Kind == "nginx" || config.Kind == "docker" {
		nginx := &bytes.Buffer{}
		fmt.Fprintf(nginx, "server {\n listen 80;\n server_name %s;\n", config.Domain)
		if config.HTTPS == "existing" {
			if certificate == nil {
				return "", errors.New("select an existing certificate")
			}
			certDir := path.Join(config.Directory, ".certificates", config.CertificateID)
			script.WriteString("mkdir -p " + sshShellQuote(certDir) + "\nchmod 700 " + sshShellQuote(certDir) + "\n")
			script.WriteString(deploymentWriteFile(path.Join(certDir, "fullchain.pem"), certificate.Certificate) + deploymentWriteFile(path.Join(certDir, "private.key"), certificate.PrivateKey))
			fmt.Fprintf(nginx, " listen 443 ssl;\n ssl_certificate %s;\n ssl_certificate_key %s;\n", nginxQuote(path.Join(certDir, "fullchain.pem")), nginxQuote(path.Join(certDir, "private.key")))
		}
		if config.Kind == "nginx" {
			nginx.WriteString(" location ~ /\\.(?!well-known/) { deny all; }\n")
			fmt.Fprintf(nginx, " root %s;\n index index.html;\n location / { try_files $uri $uri/ ", nginxQuote(path.Join(root, artifactPath)))
			if config.SPA {
				nginx.WriteString("/index.html; }")
			} else {
				nginx.WriteString("=404; }")
			}
		} else {
			fmt.Fprintf(nginx, " location / { proxy_pass http://127.0.0.1:%s; proxy_set_header Host $host; }", config.Port)
		}
		nginx.WriteString("\n}\n")
		configDirectory := config.NginxConfigDirectory
		if configDirectory == "" {
			configDirectory = "/etc/nginx/conf.d"
		}
		if !path.IsAbs(configDirectory) || path.Clean(configDirectory) == "/" {
			return "", errors.New("invalid Nginx configuration directory")
		}
		configFile := path.Join(configDirectory, "multica-"+app.ID+".conf")
		backup := path.Join(root, ".multica-nginx-backup")
		script.WriteString("command -v nginx >/dev/null\nmkdir -p " + sshShellQuote(root) + "\nif [ -f " + sshShellQuote(configFile) + " ]; then cp " + sshShellQuote(configFile) + " " + sshShellQuote(backup) + "; fi\n")
		script.WriteString(deploymentWriteFile(configFile, nginx.String()))
		script.WriteString("if ! nginx -t; then if [ -f " + sshShellQuote(backup) + " ]; then cp " + sshShellQuote(backup) + " " + sshShellQuote(configFile) + "; else rm -f " + sshShellQuote(configFile) + "; fi; exit 1; fi\nnginx -s reload\n")
		if config.HTTPS == "managed" {
			script.WriteString("command -v certbot >/dev/null\ncertbot --nginx --non-interactive --agree-tos --register-unsafely-without-email -d " + sshShellQuote(config.Domain) + "\n")
		}
	}
	scheme := "http"
	if config.HTTPS != "off" {
		scheme = "https"
	}
	resolve := ""
	if config.Kind == "nginx" || config.Kind == "docker" {
		port := "80"
		if scheme == "https" {
			port = "443"
		}
		resolve = " --resolve " + sshShellQuote(config.Domain+":"+port+":127.0.0.1")
	}
	script.WriteString("attempt=0\nuntil curl --fail --silent --show-error --max-time 10" + resolve + " " + sshShellQuote(scheme+"://"+config.Domain+config.HealthPath) + " >/dev/null; do\nattempt=$((attempt + 1))\nif [ \"$attempt\" -ge 10 ]; then exit 1; fi\nsleep 2\ndone\n")
	return script.String(), nil
}

func nginxQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`).Replace(value) + `"`
}
