package workbench

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/multica-ai/multica/server/internal/util"
)

type DeploymentResource struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Env          string `json:"env"`
	Host         string `json:"host"`
	Port         string `json:"port"`
	Username     string `json:"username"`
	HostKey      string `json:"host_key"`
	CredentialID string `json:"credential_id"`
	AuthMethod   string `json:"auth_method,omitempty"`
}

func (resource DeploymentResource) CredentialKind() string {
	if resource.AuthMethod == "password" {
		return "ssh-password"
	}
	return "ssh-key"
}

type ApplicationDeployment struct {
	NginxConfigDirectory string   `json:"nginx_config_directory,omitempty"`
	Kind                 string   `json:"kind"`
	Nodes                []string `json:"nodes"`
	Directory            string   `json:"directory"`
	Port                 string   `json:"port"`
	Domain               string   `json:"domain"`
	HTTPS                string   `json:"https"`
	CertificateID        string   `json:"certificate_id"`
	Strategy             string   `json:"strategy"`
	HealthPath           string   `json:"health_path"`
	Command              string   `json:"command"`
	Namespace            string   `json:"namespace"`
	SPA                  bool     `json:"spa"`
}

type DeploymentApplication struct {
	ID           string                           `json:"id"`
	Name         string                           `json:"name"`
	SourceKind   string                           `json:"source_kind"`
	Source       string                           `json:"source"`
	RepoID       string                           `json:"repo_id"`
	ArtifactPath string                           `json:"artifact_path"`
	BuildCommand string                           `json:"build_command"`
	Environments map[string]ApplicationDeployment `json:"environments"`
}

type ProjectDeploymentConfig struct {
	Mode         string                  `json:"mode"`
	Applications []DeploymentApplication `json:"applications"`
	Resources    []DeploymentResource    `json:"resources"`
}

func EmptyProjectDeployment() ProjectDeploymentConfig {
	return ProjectDeploymentConfig{Mode: "auto", Applications: []DeploymentApplication{}, Resources: []DeploymentResource{}}
}

func validDeploymentID(id string) bool { _, err := util.ParseUUID(id); return err == nil }

func (config *ProjectDeploymentConfig) Validate() error {
	if config.Applications == nil {
		config.Applications = []DeploymentApplication{}
	}
	if config.Resources == nil {
		config.Resources = []DeploymentResource{}
	}
	for i := range config.Applications {
		if config.Applications[i].Environments == nil {
			config.Applications[i].Environments = map[string]ApplicationDeployment{}
		}
	}
	if !slices.Contains([]string{"auto", "orchestrated"}, config.Mode) {
		return errors.New("invalid upgrade mode")
	}
	if len(config.Applications) > 100 || len(config.Resources) > 100 {
		return errors.New("at most 100 applications and resources are supported")
	}
	resources := map[string]DeploymentResource{}
	for i, resource := range config.Resources {
		if !slices.Contains([]string{"", "key", "password"}, resource.AuthMethod) {
			return errors.New("unsupported SSH login method")
		}
		if !validDeploymentID(resource.ID) || strings.TrimSpace(resource.Name) == "" || len(resource.Name) > 240 {
			return errors.New("resource identity and name are required")
		}
		if _, exists := resources[resource.ID]; exists {
			return errors.New("duplicate resource")
		}
		if !slices.Contains([]string{"staging", "prod"}, resource.Env) {
			return errors.New("invalid resource environment")
		}
		if resource.CredentialID != "" && !validDeploymentID(resource.CredentialID) {
			return errors.New("invalid credential reference")
		}
		target := resource.Target()
		if err := target.Validate(); err != nil {
			return err
		}
		resource.Host, resource.Port, resource.Username, resource.HostKey = target.Location["host"], target.Location["port"], target.Location["username"], target.Location["host_key"]
		config.Resources[i] = resource
		resources[resource.ID] = resource
	}
	apps := map[string]bool{}
	for _, app := range config.Applications {
		if !validDeploymentID(app.ID) || apps[app.ID] || strings.TrimSpace(app.Name) == "" || len(app.Name) > 240 {
			return errors.New("application identity and unique name are required")
		}
		apps[app.ID] = true
		if !slices.Contains([]string{"git", "image", "directory"}, app.SourceKind) {
			return errors.New("invalid application source")
		}
		if len(app.Source) > 2048 || len(app.BuildCommand) > 8192 {
			return errors.New("application source or build command is too long")
		}
		if app.SourceKind == "git" && app.Source != "" {
			u, err := url.Parse(app.Source)
			if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "ssh") || (u.User != nil && u.Scheme != "ssh") {
				return errors.New("use an HTTPS or SSH repository URL without embedded secrets")
			}
		}
		if app.ArtifactPath != "" && (path.IsAbs(app.ArtifactPath) || path.Clean(app.ArtifactPath) == ".." || strings.HasPrefix(path.Clean(app.ArtifactPath), "../")) {
			return errors.New("artifact directory must stay within the source directory")
		}
		if app.RepoID != "" && !validDeploymentID(app.RepoID) {
			return errors.New("invalid repository reference")
		}
		for env, deployment := range app.Environments {
			if !slices.Contains([]string{"staging", "prod"}, env) {
				return errors.New("invalid application environment")
			}
			if !slices.Contains([]string{"nginx", "docker", "process", "kubernetes"}, deployment.Kind) || !slices.Contains([]string{"off", "existing", "managed"}, deployment.HTTPS) || !slices.Contains([]string{"serial", "rolling", "batch"}, deployment.Strategy) {
				return errors.New("invalid deployment configuration")
			}
			if deployment.CertificateID != "" && !validDeploymentID(deployment.CertificateID) {
				return errors.New("invalid certificate reference")
			}
			seen := map[string]bool{}
			for _, node := range deployment.Nodes {
				resource, exists := resources[node]
				if !exists || resource.Env != env || seen[node] {
					return errors.New("deployment nodes must be unique resources in the selected environment")
				}
				seen[node] = true
			}
			if strings.ContainsAny(deployment.Domain, " /\\;{}\t\r\n\x00") || len(deployment.Domain) > 253 {
				return errors.New("enter a domain name without a scheme or path")
			}
			if deployment.NginxConfigDirectory != "" && (!path.IsAbs(deployment.NginxConfigDirectory) || path.Clean(deployment.NginxConfigDirectory) == "/" || strings.ContainsAny(deployment.NginxConfigDirectory, "\x00\r\n")) {
				return errors.New("Nginx configuration directory must be an absolute non-root path")
			}
			if deployment.Directory != "" && (!path.IsAbs(deployment.Directory) || path.Clean(deployment.Directory) == "/" || strings.ContainsAny(deployment.Directory, "\x00\r\n")) {
				return errors.New("deployment directory must be an absolute non-root path")
			}
		}
	}
	return nil
}

func (resource DeploymentResource) Target() DeployTargetInput {
	var credential *string
	if resource.CredentialID != "" {
		credential = &resource.CredentialID
	}
	return DeployTargetInput{Label: resource.Name, Env: resource.Env, Kind: "docker-host", Channel: "ssh", CredentialID: credential, Location: map[string]string{"host": resource.Host, "port": resource.Port, "username": resource.Username, "host_key": resource.HostKey}}
}

type ReleaseChange struct {
	ArtifactDigest  string `json:"artifact_digest,omitempty"`
	ID              string `json:"id"`
	Type            string `json:"type"`
	Name            string `json:"name"`
	ApplicationID   string `json:"application_id"`
	Version         string `json:"version"`
	ResolvedVersion string `json:"resolved_version,omitempty"`
	After           string `json:"after"`
	Body            string `json:"body"`
	Path            string `json:"path"`
	Database        string `json:"database"`
	Rehearsed       bool   `json:"rehearsed"`
}

type ApplicationRelease struct {
	Title    string                   `json:"title"`
	Mode     string                   `json:"mode"`
	Changes  []ReleaseChange          `json:"changes"`
	Snapshot *ProjectDeploymentConfig `json:"snapshot,omitempty"`
	Results  []ReleaseStepResult      `json:"results,omitempty"`
}

type ReleaseStepResult struct {
	ID     string `json:"id"`
	NodeID string `json:"node_id"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	At     string `json:"at"`
}

func (release *ApplicationRelease) Validate() error {
	if strings.TrimSpace(release.Title) == "" || len(release.Title) > 240 {
		return errors.New("release title is required")
	}
	if !slices.Contains([]string{"auto", "orchestrated"}, release.Mode) || len(release.Changes) > 100 {
		return errors.New("invalid release mode or too many changes")
	}
	seen := map[string]bool{}
	applications := map[string]bool{}
	for _, change := range release.Changes {
		if !validDeploymentID(change.ID) || seen[change.ID] {
			return errors.New("invalid or duplicate change identity")
		}
		if change.After != "" && !seen[change.After] {
			return errors.New("a dependency must precede its change")
		}
		seen[change.ID] = true
		if !slices.Contains([]string{"app", "config", "sql"}, change.Type) || (release.Mode == "auto" && change.Type != "app") {
			return errors.New("automatic upgrade releases contain applications only")
		}
		if !validDeploymentID(change.ApplicationID) {
			return errors.New("select an application for each change")
		}
		if change.Type == "app" {
			if applications[change.ApplicationID] {
				return errors.New("each application can have only one target version in a release")
			}
			applications[change.ApplicationID] = true
		}
		if len(change.Body) > 65536 || len(change.Version) > 256 || len(change.Name) > 240 {
			return errors.New("change content is too long")
		}
	}
	return nil
}

func (release ApplicationRelease) Missing(config ProjectDeploymentConfig, env string) []string {
	missing := []string{}
	if len(release.Changes) == 0 {
		missing = append(missing, "add release content")
	}
	apps := map[string]DeploymentApplication{}
	for _, app := range config.Applications {
		apps[app.ID] = app
	}
	for _, change := range release.Changes {
		app, exists := apps[change.ApplicationID]
		deployment, configured := app.Environments[env]
		if !exists || !configured || len(deployment.Nodes) == 0 {
			missing = append(missing, fmt.Sprintf("%s: configure deployment", change.Name))
			continue
		}
		if change.Type == "app" && (strings.TrimSpace(change.Version) == "" || strings.TrimSpace(app.Source) == "") {
			missing = append(missing, app.Name+": source and version are required")
		}
		if deployment.Domain == "" || deployment.Directory == "" {
			missing = append(missing, app.Name+": domain and deployment directory are required")
		}
		if deployment.HTTPS == "existing" && deployment.CertificateID == "" {
			missing = append(missing, app.Name+": choose a certificate")
		}
		if change.Type == "sql" && (!change.Rehearsed || strings.TrimSpace(change.Body) == "" || change.Database == "") {
			missing = append(missing, change.Name+": SQL, database and rehearsal confirmation are required")
		}
		if change.Type == "config" && (strings.TrimSpace(change.Body) == "" || !path.IsAbs(change.Path)) {
			missing = append(missing, change.Name+": configuration content and absolute target path are required")
		}
	}
	return missing
}
