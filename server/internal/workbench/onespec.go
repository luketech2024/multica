package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
)

const OneSpecVersion = "1.1.0"

type OneSpecArtifactRef struct {
	RepositoryID string `json:"repository_id"`
	Path         string `json:"path"`
	Commit       string `json:"commit"`
	Digest       string `json:"digest"`
}

type OneSpecArtifact struct {
	Document           *RequirementsDocument `json:"document,omitempty"`
	SourceTaskID       string                `json:"source_task_id,omitempty"`
	DraftRevision      int64                 `json:"draft_revision,omitempty"`
	DeliveryDigest     string                `json:"delivery_digest,omitempty"`
	ReleaseRevision    int64                 `json:"release_revision,omitempty"`
	Stage              string                `json:"stage"`
	Version            int                   `json:"version"`
	Summary            string                `json:"summary"`
	Refs               []OneSpecArtifactRef  `json:"refs"`
	Tasks              []PlanTask            `json:"tasks,omitempty"`
	RequirementVersion int                   `json:"requirement_version"`
	SubmittedBy        string                `json:"submitted_by"`
	CreatedAt          string                `json:"created_at"`
}

type OneSpecDecision struct {
	Stage     string `json:"stage"`
	Version   int    `json:"version"`
	Approved  bool   `json:"approved"`
	Reason    string `json:"reason"`
	ActorID   string `json:"actor_id"`
	CreatedAt string `json:"created_at"`
}

type OneSpecGitOperation struct {
	Worktree        string `json:"worktree,omitempty"`
	Key             string `json:"key"`
	RepositoryID    string `json:"repository_id"`
	Kind            string `json:"kind"`
	Branch          string `json:"branch"`
	BaseCommit      string `json:"base_commit"`
	PublishedCommit string `json:"published_commit,omitempty"`
	MergedCommit    string `json:"merged_commit,omitempty"`
	Status          string `json:"status"`
	Summary         string `json:"summary"`
}

type OneSpecFlow struct {
	Generation           int                      `json:"generation"`
	ReleaseID            string                   `json:"release_id,omitempty"`
	V                    int                      `json:"v"`
	Kind                 string                   `json:"kind"`
	Phase                string                   `json:"phase"`
	Status               string                   `json:"status"`
	RuntimeID            string                   `json:"runtime_id"`
	OverviewRepositoryID string                   `json:"overview_repository_id"`
	ApproverID           string                   `json:"approver_id"`
	RuleVersion          string                   `json:"rule_version"`
	Team                 ProjectRoleConfiguration `json:"team"`
	Artifacts            []OneSpecArtifact        `json:"artifacts"`
	Decisions            []OneSpecDecision        `json:"decisions"`
	GitOperations        []OneSpecGitOperation    `json:"git_operations"`
}

func OneSpecStages(kind string) []string {
	switch kind {
	case "feature":
		return []string{"requirements", "design", "plan", "branch", "implementation", "testing", "archive", "archiving", "acceptance"}
	case "change", "bugfix":
		return []string{"requirements", "plan", "branch", "implementation", "testing", "archive", "archiving", "acceptance"}
	case "release":
		return []string{"requirements", "release_preparation", "deployment", "merge", "merging", "acceptance"}
	case "documentation":
		return []string{"requirements", "branch", "documentation", "testing", "merge", "merging", "acceptance"}
	}
	return nil
}

func OneSpecStageRole(stage string) string {
	switch stage {
	case "requirements":
		return "product"
	case "testing":
		return "test"
	case "release_preparation", "deployment":
		return "ops"
	case "documentation":
		return "docs"
	case "branch", "merge", "merging":
		return "git"
	case "acceptance", "complete":
		return "sched"
	default:
		return "dev"
	}
}

func (f OneSpecFlow) Latest(stage string) *OneSpecArtifact {
	for i := len(f.Artifacts) - 1; i >= 0; i-- {
		if f.Artifacts[i].Stage == stage {
			return &f.Artifacts[i]
		}
	}
	return nil
}

func (f OneSpecFlow) Approved(stage string) bool {
	artifact := f.Latest(stage)
	if artifact == nil {
		return false
	}
	for i := len(f.Decisions) - 1; i >= 0; i-- {
		d := f.Decisions[i]
		if d.Stage == stage && d.Version == artifact.Version {
			return d.Approved
		}
	}
	return false
}

var oneSpecHash = regexp.MustCompile(`^[a-fA-F0-9]{40,64}$`)

func (f *OneSpecFlow) Submit(a OneSpecArtifact, actor string, now time.Time) error {
	if a.Stage != f.Phase || f.Status == "completed" {
		return errors.New("submit an artifact for the current stage")
	}
	if strings.TrimSpace(a.Summary) == "" || len([]rune(a.Summary)) > 4000 {
		return errors.New("provide a summary of 1 to 4000 characters")
	}
	if len(a.Refs) == 0 || len(a.Refs) > 100 {
		return errors.New("provide 1 to 100 versioned artifact references")
	}
	if a.Stage == "requirements" && f.RuleVersion == OneSpecVersion && a.Document == nil {
		return errors.New("submit the readable PRD document with requirements")
	}
	if a.Document != nil {
		if a.Stage != "requirements" {
			return errors.New("requirement documents belong to the requirements stage")
		}
		if err := a.Document.Validate(false); err != nil {
			return err
		}
		for _, document := range []*RequirementsDocument{a.Document, a.Document.Prototype} {
			if document == nil {
				continue
			}
			found := false
			for _, ref := range a.Refs {
				if ref == document.Ref {
					found = true
					break
				}
			}
			if !found {
				return errors.New("include each document in the artifact references")
			}
			document.Verified, document.Error = false, ""
		}
	}
	for _, ref := range a.Refs {
		if ref.RepositoryID == "" || ref.Path == "" || strings.HasPrefix(ref.Path, "/") || strings.Contains(ref.Path, "\\") || strings.Contains(ref.Path, ":") || path.Clean(ref.Path) != ref.Path || ref.Path == ".." || strings.HasPrefix(ref.Path, "../") || !oneSpecHash.MatchString(ref.Commit) || !oneSpecHash.MatchString(ref.Digest) {
			return errors.New("artifact requires a repository, relative path, commit and content digest")
		}
	}
	a.Version = 1
	if previous := f.Latest(a.Stage); previous != nil {
		a.Version = previous.Version + 1
	}
	a.SubmittedBy, a.CreatedAt = actor, now.UTC().Format(time.RFC3339Nano)
	f.Artifacts = append(f.Artifacts, a)
	f.Status = "ready"
	if OneSpecNeedsConfirmation(f.Phase) {
		f.Status = "awaiting_confirmation"
	}
	return nil
}

func (f *OneSpecFlow) Decide(stage string, version int, approved bool, reason, actor string, now time.Time) error {
	a := f.Latest(stage)
	if stage != f.Phase || a == nil || a.Version != version {
		return errors.New("artifact version changed; reload before confirming")
	}
	if approved && !a.DocumentReady() {
		return errors.New("wait for the runtime to verify the committed requirement documents")
	}
	if f.Status != "awaiting_confirmation" {
		return errors.New("this stage is not waiting for confirmation")
	}
	if !approved && strings.TrimSpace(reason) == "" {
		return errors.New("provide a reason for returning this stage")
	}
	f.Decisions = append(f.Decisions, OneSpecDecision{Stage: stage, Version: version, Approved: approved, Reason: reason, ActorID: actor, CreatedAt: now.UTC().Format(time.RFC3339Nano)})
	f.Status = "preparing"
	if approved {
		f.Status = "ready"
	}
	return nil
}

func (f *OneSpecFlow) Advance() error {
	if f.Status != "ready" || (OneSpecNeedsConfirmation(f.Phase) && !f.Approved(f.Phase)) {
		return errors.New("confirm the current artifact before continuing")
	}
	stages := OneSpecStages(f.Kind)
	for i, stage := range stages {
		if stage != f.Phase {
			continue
		}
		f.Generation++
		f.Phase, f.Status = "complete", "completed"
		if i+1 < len(stages) {
			f.Phase, f.Status = stages[i+1], "preparing"
		}
		return nil
	}
	return errors.New("unknown workflow stage")
}

func OneSpecBranch(kind, taskID, description, date string) (string, error) {
	var name string
	switch kind {
	case "development":
		name = "release_" + taskID
	case "hotfix":
		name = "hotfix_" + date + taskID
	case "documentation":
		name = "docs_" + description
	case "upstream":
		name = "main_" + date
	default:
		return "", errors.New("unknown branch operation")
	}
	if (kind == "development" || kind == "hotfix") && taskID == "" {
		return "", errors.New("task identifier is required")
	}
	if kind == "documentation" && strings.TrimSpace(description) == "" {
		return "", errors.New("document branch description is required")
	}
	if kind == "hotfix" || kind == "upstream" {
		if _, err := time.Parse("20060102", date); err != nil {
			return "", errors.New("date must use YYYYMMDD")
		}
	}
	if strings.ContainsAny(name, " ~^:?*[\\\x00\r\n\t") || strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".lock") || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid branch name %q", name)
	}
	return name, nil
}

func OneSpecNeedsConfirmation(stage string) bool {
	switch stage {
	case "requirements", "design", "plan", "archive", "deployment", "merge", "acceptance":
		return true
	}
	return false
}

// Reopen preserves history while making every downstream confirmation obsolete.
func (f *OneSpecFlow) Reopen(stage string) error {
	stages := OneSpecStages(f.Kind)
	start, current := -1, -1
	for i, item := range stages {
		if item == stage {
			start = i
		}
		if item == f.Phase {
			current = i
		}
	}
	if f.Phase == "complete" {
		current = len(stages)
	}
	if start < 0 || start > current {
		return errors.New("select the current or an earlier stage")
	}
	for _, item := range stages[start:] {
		if previous := f.Latest(item); previous != nil {
			f.Artifacts = append(f.Artifacts, OneSpecArtifact{Stage: item, Version: previous.Version + 1, Summary: "", Refs: []OneSpecArtifactRef{}})
		}
	}
	f.Generation++
	f.Phase, f.Status = stage, "preparing"
	return nil
}

func (f *OneSpecFlow) RecordGit(op OneSpecGitOperation, taskID string) error {
	if !oneSpecHash.MatchString(op.BaseCommit) {
		return errors.New("record the full base commit")
	}
	if op.Status != "prepared" && op.Status != "completed" && op.Status != "failed" {
		return errors.New("invalid Git operation status")
	}
	switch op.Kind {
	case "development":
		branch, err := OneSpecBranch(op.Kind, taskID, "", "")
		if err != nil || op.Branch != branch {
			return errors.New("development branch must use release_{task_id}")
		}
	case "hotfix":
		if len(op.Branch) < 15 {
			return errors.New("invalid hotfix branch")
		}
		branch, err := OneSpecBranch(op.Kind, taskID, "", op.Branch[7:15])
		if err != nil || op.Branch != branch {
			return errors.New("hotfix branch must use hotfix_{YYYYMMDD}{task_id}")
		}
	case "documentation":
		if !strings.HasPrefix(op.Branch, "docs_") {
			return errors.New("documentation branch must start with docs_")
		}
		if _, err := OneSpecBranch(op.Kind, "", strings.TrimPrefix(op.Branch, "docs_"), ""); err != nil {
			return err
		}
	case "upstream":
		if !strings.HasPrefix(op.Branch, "main_") {
			return errors.New("upstream branch must start with main_")
		}
		if _, err := OneSpecBranch(op.Kind, "", "", strings.TrimPrefix(op.Branch, "main_")); err != nil {
			return err
		}
	case "release_merge":
		if op.Branch != "release" || !oneSpecHash.MatchString(op.PublishedCommit) {
			return errors.New("release merge requires the published commit and release target")
		}
		deployment := f.Latest("deployment")
		if deployment == nil || deployment.Summary == "" || !f.Approved("deployment") {
			return errors.New("record confirmed deployment evidence before merging a release")
		}
	default:
		return errors.New("unknown Git operation kind")
	}
	if op.MergedCommit != "" && !f.Approved("merge") {
		return errors.New("confirm the integration proposal before recording a merge")
	}
	if op.MergedCommit != "" && !oneSpecHash.MatchString(op.MergedCommit) {
		return errors.New("invalid merge commit")
	}
	if op.Status == "completed" && (op.Kind == "release_merge" || op.Kind == "upstream") && op.MergedCommit == "" {
		return errors.New("completed integration requires its merge commit")
	}
	for i, previous := range f.GitOperations {
		if previous.Key != op.Key {
			continue
		}
		if previous.RepositoryID != op.RepositoryID || previous.Kind != op.Kind || previous.Branch != op.Branch || previous.BaseCommit != op.BaseCommit || previous.Worktree != op.Worktree {
			return errors.New("operation key already belongs to a different operation")
		}
		if previous.Status == "completed" && previous != op {
			return errors.New("completed operations are immutable")
		}
		f.GitOperations[i] = op
		return nil
	}
	f.GitOperations = append(f.GitOperations, op)
	return nil
}

func (f OneSpecFlow) ValidatePriorVersions(version int) error {
	for _, stage := range OneSpecStages(f.Kind) {
		if stage == f.Phase {
			break
		}
		artifact := f.Latest(stage)
		if artifact == nil || artifact.Summary == "" || artifact.RequirementVersion != version {
			return fmt.Errorf("reopen %s and regenerate evidence for the current requirements", stage)
		}
	}
	return nil
}

func OneSpecDeliveryDigest(criteria Requirements) string {
	raw, _ := json.Marshal(criteria.Delivery)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
