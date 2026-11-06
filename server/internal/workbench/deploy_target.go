package workbench

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

type DeployTargetInput struct {
	RepoID           string            `json:"repo_id"`
	Label            string            `json:"label"`
	Env              string            `json:"env"`
	Kind             string            `json:"kind"`
	Channel          string            `json:"channel"`
	RuntimeID        *string           `json:"runtime_id"`
	CredentialID     *string           `json:"credential_id"`
	Location         map[string]string `json:"location"`
	ExpectedRevision int64             `json:"expected_revision"`
}

var targetLocationFields = map[string][]string{
	"docker-host":     {"host", "image", "dir", "health_url"},
	"service-process": {"host", "dir", "start_command", "health_url"},
	"weapp-ci":        {"appid", "robot"},
	"static-site":     {"bucket", "domain"},
	"npm-package":     {"registry"},
}

// Incomplete configurations can be saved, but cannot satisfy a release gate.
func (target *DeployTargetInput) Validate() error {
	target.Label = strings.TrimSpace(target.Label)
	if target.Label == "" || len([]rune(target.Label)) > 120 {
		return errors.New("target label must contain 1 to 120 characters")
	}
	if !slices.Contains([]string{"preview", "staging", "prod"}, target.Env) {
		return errors.New("unsupported target environment")
	}
	fields, ok := targetLocationFields[target.Kind]
	if !ok {
		return errors.New("unsupported deployment kind")
	}
	if target.Kind == "docker-host" || target.Kind == "service-process" {
		if target.Channel != "runtime" && target.Channel != "ssh" {
			return errors.New("host deployments require runtime or ssh channel")
		}
	} else if target.Channel != "platform" {
		return errors.New("this deployment kind requires the platform channel")
	}
	if target.Channel != "runtime" && target.RuntimeID != nil {
		return errors.New("runtime_id is only valid for the runtime channel")
	}
	if target.Location == nil {
		target.Location = map[string]string{}
	}
	for key, value := range target.Location {
		if !slices.Contains(fields, key) && key != "domain" {
			return fmt.Errorf("unsupported location field %q", key)
		}
		value = strings.TrimSpace(value)
		if len(value) > 2048 {
			return fmt.Errorf("location field %s exceeds 2048 bytes", key)
		}
		if (key == "health_url" || key == "registry") && value != "" {
			u, err := url.Parse(value)
			if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return fmt.Errorf("%s must be an HTTP(S) URL without embedded credentials", key)
			}
		}
		target.Location[key] = value
	}
	return nil
}

func (target DeployTargetInput) MissingFields() []string {
	missing := []string{}
	for _, field := range targetLocationFields[target.Kind] {
		if target.Location[field] == "" {
			missing = append(missing, field)
		}
	}
	if target.Channel == "runtime" {
		if target.RuntimeID == nil {
			missing = append(missing, "runtime_id")
		}
	} else if target.CredentialID == nil {
		missing = append(missing, "credential_id")
	}
	return missing
}

func (target DeployTargetInput) CredentialKind() string {
	if target.Channel == "ssh" {
		return "ssh-key"
	}
	return map[string]string{
		"weapp-ci":    "weapp-ci-key",
		"static-site": "cloud-ak",
		"npm-package": "registry-token",
	}[target.Kind]
}
