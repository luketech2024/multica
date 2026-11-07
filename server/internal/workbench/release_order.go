package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
)

type ReleaseTaskInput struct {
	ID               string `json:"id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type ReleaseOrderInput struct {
	ProjectID string             `json:"project_id"`
	Env       string             `json:"env"`
	RequestID string             `json:"request_id"`
	Tasks     []ReleaseTaskInput `json:"tasks"`
	Tags      map[string]string  `json:"tags"`
}

type ReleaseTaskSnapshot struct {
	ID         string       `json:"id"`
	Identifier string       `json:"identifier"`
	Title      string       `json:"title"`
	Revision   int64        `json:"revision"`
	Criteria   Requirements `json:"acceptance_criteria"`
}

type ReleaseRepositorySnapshot struct {
	RepoID string         `json:"repo_id"`
	Tag    string         `json:"tag"`
	Merges []ReleaseMerge `json:"merges"`
}

type ReleaseManifest struct {
	Tasks        []ReleaseTaskSnapshot       `json:"tasks"`
	Repositories []ReleaseRepositorySnapshot `json:"repositories"`
}

var releaseTag = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func (input *ReleaseOrderInput) Normalize() error {
	if input.Env != "staging" && input.Env != "prod" {
		return errors.New("release environment must be staging or prod")
	}
	if len(input.Tasks) == 0 || len(input.Tasks) > 100 {
		return errors.New("select 1 to 100 accepted tasks")
	}
	slices.SortFunc(input.Tasks, func(a, b ReleaseTaskInput) int { return strings.Compare(a.ID, b.ID) })
	for i, task := range input.Tasks {
		if task.ExpectedRevision < 1 || (i > 0 && input.Tasks[i-1].ID == task.ID) {
			return errors.New("task revisions are required and duplicate tasks are not allowed")
		}
	}
	if len(input.Tags) > 100 {
		return errors.New("too many repository tags")
	}
	for _, tag := range input.Tags {
		if !releaseTag.MatchString(tag) || strings.Contains(tag, "..") || strings.HasSuffix(tag, ".") || strings.HasSuffix(tag, ".lock") {
			return errors.New("repository tags must use 1 to 64 letters, numbers, dots, underscores or hyphens and form a valid Git tag")
		}
	}
	if input.Tags == nil {
		input.Tags = map[string]string{}
	}
	return nil
}

func (input ReleaseOrderInput) Digest() string {
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
