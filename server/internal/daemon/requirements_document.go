package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/workbench"
)

func verifyRequirementFile(ctx context.Context, document *workbench.RequirementsDocument, task Task, cwd string) error {
	if err := document.Validate(strings.HasSuffix(document.Ref.Path, ".html")); err != nil {
		return err
	}
	directory := document.CheckoutPath
	if directory == "" {
		directory = cwd
	}
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(cwd, directory)
	}
	var resource *ProjectResourceData
	for i := range task.ProjectResources {
		if task.ProjectResources[i].ID == document.Ref.RepositoryID {
			resource = &task.ProjectResources[i]
			break
		}
	}
	if resource == nil {
		return fmt.Errorf("document repository is unavailable in this task")
	}
	var ref struct {
		URL       string `json:"url"`
		LocalPath string `json:"local_path"`
	}
	if json.Unmarshal(resource.ResourceRef, &ref) != nil {
		return fmt.Errorf("invalid repository configuration")
	}
	if resource.ResourceType == "local_directory" {
		expected, e := filepath.EvalSymlinks(ref.LocalPath)
		actual, a := filepath.EvalSymlinks(directory)
		if e != nil || a != nil {
			return fmt.Errorf("repository directory is unavailable")
		}
		// A worktree shares its object database with the configured checkout.
		common := func(dir string) string {
			out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(out))
		}
		if common(expected) == "" || common(expected) != common(actual) {
			return fmt.Errorf("document checkout does not match the project repository")
		}
	} else {
		remote, err := exec.CommandContext(ctx, "git", "-C", directory, "remote", "get-url", "origin").Output()
		if err != nil || ref.URL == "" || normalizeRequirementRepository(string(remote)) != normalizeRequirementRepository(ref.URL) {
			return fmt.Errorf("document checkout does not match the project repository URL")
		}
	}
	content, err := exec.CommandContext(ctx, "git", "-C", directory, "show", document.Ref.Commit+":"+document.Ref.Path).Output()
	if err != nil {
		return fmt.Errorf("cannot read the committed document; check the commit and path")
	}
	if string(content) != document.Content {
		return fmt.Errorf("committed document differs from the submitted content")
	}
	return nil
}

func normalizeRequirementRepository(raw string) string {
	raw = strings.TrimSpace(raw)
	if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
		raw = strings.ToLower(parsed.Host) + "/" + strings.TrimPrefix(parsed.Path, "/")
	} else if user, path, ok := strings.Cut(raw, ":"); ok && strings.Contains(user, "@") {
		_, host, _ := strings.Cut(user, "@")
		raw = strings.ToLower(host) + "/" + path
	}
	return strings.TrimSuffix(strings.TrimSuffix(raw, "/"), ".git")
}

func (d *Daemon) verifyRequirementDocuments(ctx context.Context, task Task, cwd string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	endpoint := "/api/daemon/tasks/" + task.ID + "/requirements-document"
	var response struct {
		Artifact *workbench.OneSpecArtifact `json:"artifact"`
	}
	if err := d.client.getJSON(ctx, endpoint, &response); err != nil {
		return err
	}
	if response.Artifact == nil || response.Artifact.Document == nil {
		return nil
	}
	document := response.Artifact.Document
	result := map[string]any{"version": response.Artifact.Version, "error": "", "prototype_error": ""}
	if err := verifyRequirementFile(ctx, document, task, cwd); err != nil {
		result["error"] = err.Error()
	}
	if document.Prototype != nil {
		if err := verifyRequirementFile(ctx, document.Prototype, task, cwd); err != nil {
			result["prototype_error"] = err.Error()
		}
	}
	return d.client.postJSONWithRetry(ctx, endpoint, result, nil, []time.Duration{0, 200 * time.Millisecond, time.Second})
}
