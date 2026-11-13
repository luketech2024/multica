package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

type DeliveryReport struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type DeliveryChanges struct {
	Files     int `json:"files"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

// DeliveryEvidence records attributed reports, not an inferred result from task status.
type DeliveryEvidence struct {
	TaskHash        string           `json:"task_hash,omitempty"`
	DescriptionHash string           `json:"description_hash"`
	IssueID         string           `json:"issue_id"`
	IssueRevision   int64            `json:"issue_revision"`
	Version         int              `json:"version"`
	Role            string           `json:"role"`
	RepoID          string           `json:"repo_id"`
	Commit          string           `json:"commit"`
	Passed          bool             `json:"passed"`
	Summary         string           `json:"summary"`
	Reports         []DeliveryReport `json:"reports"`
	Changes         *DeliveryChanges `json:"changes"`
	PreviewURL      string           `json:"preview_url"`
	RecordedAt      string           `json:"recorded_at"`
	RecordedBy      string           `json:"recorded_by"`
	ActorType       string           `json:"actor_type"`
}

var sourceCommit = regexp.MustCompile(`^[a-fA-F0-9]{40}([a-fA-F0-9]{24})?$`)

func DeliveryDescriptionHash(description string) string {
	digest := sha256.Sum256([]byte(description))
	return hex.EncodeToString(digest[:])
}

// Comments advance the issue revision without changing the delivery scope.
func DeliveryTaskHash(task DeliveryTask) string {
	content, _ := json.Marshal(struct {
		Title       string
		Description string
		Handoff     Handoff
	}{task.Title, task.Description, task.Handoff})
	return DeliveryDescriptionHash(string(content))
}

func validEvidenceURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && len(value) <= 2048 && parsed.Hostname() != "" && parsed.User == nil && (parsed.Scheme == "https" || parsed.Scheme == "http")
}

func ValidateDeliveryEvidence(criteria Requirements, handoff Handoff, evidence DeliveryEvidence) error {
	if criteria.Gate.Spec == nil || criteria.Change != nil || criteria.NeedsResplit || criteria.Team == nil || criteria.Plan == nil || criteria.Plan.Version != criteria.Version {
		return errors.New("delivery evidence requires a confirmed current execution plan")
	}
	if !slices.Contains(criteria.Plan.Issues, evidence.IssueID) || handoff.V != 2 || handoff.Superseded || handoff.RequirementVersion != criteria.Version {
		return errors.New("delivery evidence must belong to a current planned subtask")
	}
	if !criteria.Team.Config.Roles[handoff.Role].Enabled {
		return errors.New("the delivery role is not enabled for this task")
	}
	if strings.TrimSpace(evidence.Summary) == "" || len([]rune(evidence.Summary)) > 2000 {
		return errors.New("a delivery conclusion of 1 to 2000 characters is required")
	}
	if handoff.RepoID != "" && !sourceCommit.MatchString(evidence.Commit) {
		return errors.New("repository evidence requires a full source commit hash")
	}
	if len(evidence.Reports) == 0 || len(evidence.Reports) > 100 {
		return errors.New("delivery requires 1 to 100 report links")
	}
	reports := make(map[string]bool, len(evidence.Reports))
	for _, report := range evidence.Reports {
		if strings.TrimSpace(report.Name) == "" || len([]rune(report.Name)) > 200 || !validEvidenceURL(report.URL) || reports[report.Name] {
			return errors.New("report names must be unique and links must be HTTP(S) URLs without credentials")
		}
		reports[report.Name] = true
	}
	for _, expected := range handoff.Expected {
		if !reports[expected] {
			return fmt.Errorf("missing expected delivery report: %s", expected)
		}
	}
	if evidence.Changes != nil && (evidence.Changes.Files < 0 || evidence.Changes.Additions < 0 || evidence.Changes.Deletions < 0) {
		return errors.New("change statistics cannot be negative")
	}
	if evidence.PreviewURL != "" && !validEvidenceURL(evidence.PreviewURL) {
		return errors.New("preview verification requires an HTTP(S) URL without credentials")
	}
	if handoff.Role == "dev" && evidence.Passed && (evidence.Changes == nil || evidence.PreviewURL == "") {
		return errors.New("passing development evidence requires change statistics and a verified preview URL")
	}
	return nil
}
