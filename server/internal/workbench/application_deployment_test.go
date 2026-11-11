package workbench

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStaticDeploymentStagesSourceAndMakesNestedArtifactTraversable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("deployment scripts target POSIX servers")
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	if err := os.MkdirAll(filepath.Join(source, "build", "web"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "build", "web", "index.html"), []byte("website"), 0600); err != nil {
		t.Fatal(err)
	}
	app := DeploymentApplication{ID: deploymentTestID, SourceKind: "directory", Source: source, ArtifactPath: "build/web", BuildCommand: "test -f build/web/index.html"}
	config := ApplicationDeployment{Kind: "nginx", Directory: filepath.Join(directory, "deploy"), Domain: "example.test", HTTPS: "off", HealthPath: "/"}
	script, err := ApplicationDeploymentScript(deploymentTestID, app, config, ReleaseChange{ID: deploymentTestNode, Type: "app", Version: "v1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	preparation, _, ok := strings.Cut(script, "command -v nginx")
	if !ok {
		t.Fatal("missing nginx phase")
	}
	for attempt := 0; attempt < 2; attempt++ {
		command := exec.Command("sh", "-se")
		command.Stdin = strings.NewReader(preparation)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("source preparation failed: %v %s", err, output)
		}
	}
	root := filepath.Join(config.Directory, "releases", deploymentTestID)
	for _, dir := range []string{root, filepath.Join(root, "build"), filepath.Join(root, "build", "web")} {
		info, err := os.Stat(dir)
		if err != nil || info.Mode().Perm()&0001 == 0 {
			t.Fatalf("nginx cannot traverse %s: %v", dir, err)
		}
	}
	content, err := os.ReadFile(filepath.Join(root, "build", "web", "index.html"))
	if err != nil || string(content) != "website" {
		t.Fatalf("source content was not preserved: %v", err)
	}
}

const deploymentTestID = "019a6091-9363-7000-8000-000000000001"
const deploymentTestNode = "019a6091-9363-7000-8000-000000000002"

func TestApplicationReleaseDependenciesAndSourceValidation(t *testing.T) {
	release := ApplicationRelease{Title: "Existing application", Mode: "auto", Changes: []ReleaseChange{{ID: deploymentTestID, ApplicationID: deploymentTestNode, Type: "app", Name: "Website"}}}
	input := ReleaseOrderInput{Env: "staging", Delivery: &release}
	if err := input.Normalize(); err != nil {
		t.Fatal(err)
	}
	release.Changes = append(release.Changes, ReleaseChange{ID: deploymentTestNode, ApplicationID: deploymentTestNode, Type: "app", Name: "Duplicate website"})
	if err := release.Validate(); err == nil {
		t.Fatal("accepted conflicting versions for one application")
	}
	release.Changes = release.Changes[:1]
	release.Changes[0].After = deploymentTestID
	if err := release.Validate(); err == nil {
		t.Fatal("accepted cyclic dependency")
	}
	release.Changes[0].After = ""
	release.Changes[0].Type = "sql"
	if err := release.Validate(); err == nil {
		t.Fatal("accepted SQL in automatic upgrade mode")
	}
	config := EmptyProjectDeployment()
	config.Applications = []DeploymentApplication{{ID: deploymentTestID, Name: "Website", SourceKind: "git", Source: "https://example.test/repo.git", ArtifactPath: "../secret"}}
	if err := config.Validate(); err == nil {
		t.Fatal("accepted artifact path outside repository")
	}
	config.Applications[0].ArtifactPath = "dist"
	config.Applications[0].Source = "https://user:password@example.test/repo.git"
	if err := config.Validate(); err == nil {
		t.Fatal("accepted embedded repository secret")
	}
}

func TestApplicationDeploymentScriptQuotesContentAndChecksResults(t *testing.T) {
	app := DeploymentApplication{ID: deploymentTestID, SourceKind: "directory", Source: "/srv/source", ArtifactPath: "."}
	config := ApplicationDeployment{Kind: "nginx", Directory: "/srv/site", Domain: "www.example.test", HTTPS: "off", HealthPath: "/"}
	change := ReleaseChange{ID: deploymentTestNode, Type: "app", Version: "v1"}
	script, err := ApplicationDeploymentScript(deploymentTestID, app, config, change, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"flock -n 9", "nginx -t", "nginx -s reload", "curl --fail", "cp -a"} {
		if !strings.Contains(script, required) {
			t.Fatalf("missing %q", required)
		}
	}
	if runtime.GOOS != "windows" {
		command := exec.Command("sh", "-n")
		command.Stdin = strings.NewReader(script)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("invalid script: %v %s", err, output)
		}
	}
	config.NginxConfigDirectory = "/etc/nginx/conf.d/config"
	custom, err := ApplicationDeploymentScript(deploymentTestID, app, config, change, nil)
	if err != nil || !strings.Contains(custom, "/etc/nginx/conf.d/config/multica-") {
		t.Fatal("custom Nginx configuration directory was ignored")
	}
	config.Domain = "example.test; touch /tmp/injected"
	if _, err := ApplicationDeploymentScript(deploymentTestID, app, config, change, nil); err == nil {
		t.Fatal("accepted domain shell injection")
	}
	config.Domain = "www.example.test"
	change.Type = "sql"
	change.Database = "prod; touch /tmp/injected"
	change.Body = "SELECT 1"
	change.Rehearsed = true
	if _, err := ApplicationDeploymentScript(deploymentTestID, app, config, change, nil); err == nil {
		t.Fatal("accepted database shell injection")
	}
	change.Database = "production"
	script, err = ApplicationDeploymentScript(deploymentTestID, app, config, change, nil)
	if err != nil || !strings.Contains(script, "--single-transaction") {
		t.Fatalf("SQL transaction missing: %v", err)
	}
}

func TestDeploymentHealthCheckWaitsForReloadButRejectsPersistentTLSFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX deployment health check")
	}
	app := DeploymentApplication{ID: deploymentTestID, SourceKind: "directory", Source: "/srv/source", ArtifactPath: "."}
	config := ApplicationDeployment{Kind: "nginx", Directory: "/srv/site", Domain: "example.test", HTTPS: "off", HealthPath: "/"}
	script, err := ApplicationDeploymentScript(deploymentTestID, app, config, ReleaseChange{ID: deploymentTestNode, Type: "app", Version: "v1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, health, found := strings.Cut(script, "attempt=0\n")
	if !found {
		t.Fatal("missing bounded health check")
	}
	for _, recover := range []bool{true, false} {
		directory := t.TempDir()
		count := filepath.Join(directory, "attempts")
		curl := "#!/bin/sh\nn=0\nif [ -f \"$HEALTH_COUNT\" ]; then n=$(cat \"$HEALTH_COUNT\"); fi\nn=$((n + 1))\nprintf '%s' \"$n\" > \"$HEALTH_COUNT\"\nif [ \"$HEALTH_RECOVER\" = true ] && [ \"$n\" -ge 2 ]; then exit 0; fi\nexit 60\n"
		if err := os.WriteFile(filepath.Join(directory, "curl"), []byte(curl), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "sleep"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("sh", "-ec", "attempt=0\n"+health)
		recovery := "false"
		if recover {
			recovery = "true"
		}
		command.Env = append(os.Environ(), "PATH="+directory+":"+os.Getenv("PATH"), "HEALTH_COUNT="+count, "HEALTH_RECOVER="+recovery)
		err := command.Run()
		attempts, readErr := os.ReadFile(count)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if recover && (err != nil || string(attempts) != "2") {
			t.Fatalf("transient TLS error did not recover: %v %s", err, attempts)
		}
		if !recover && (err == nil || string(attempts) != "10") {
			t.Fatalf("persistent TLS failure was ignored: %v %s", err, attempts)
		}
	}
}
