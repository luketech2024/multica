package workbench

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type RequirementBaseline struct {
	Version int      `json:"version"`
	Items   []Item   `json:"items"`
	Bounds  []string `json:"bounds"`
	Gate    Gates    `json:"gate"`
}

type RequirementChange struct {
	Phase          string              `json:"phase"`
	Reason         string              `json:"reason"`
	RequestedAt    string              `json:"requested_at"`
	RequestedBy    string              `json:"requested_by"`
	PreviousStatus string              `json:"previous_status"`
	Baseline       RequirementBaseline `json:"baseline"`
	RunIDs         []string            `json:"run_ids"`
}

// StartRequirementsChange freezes the confirmed baseline before execution stops.
func StartRequirementsChange(raw []byte, status, reason, actor string, runIDs []string, now time.Time) ([]byte, error) {
	criteria, err := ParseRequirements(raw)
	if err != nil {
		return nil, err
	}
	if criteria.Change != nil {
		return nil, errors.New("a requirements change is already active")
	}
	if len(criteria.ResumeRuns) > 0 {
		return nil, errors.New("finish resuming the previous execution before another requirements change")
	}
	if criteria.Gate.Spec == nil || status != "in_progress" {
		return nil, errors.New("requirements changes require a confirmed task in progress")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 2000 {
		return nil, errors.New("a change reason of 1 to 2000 characters is required")
	}
	if runIDs == nil {
		runIDs = []string{}
	}
	phase := "stopping"
	if len(runIDs) == 0 {
		phase = "paused"
	}
	criteria.Change = &RequirementChange{
		Phase: phase, Reason: reason, RequestedAt: now.UTC().Format(time.RFC3339Nano), RequestedBy: actor,
		PreviousStatus: status, RunIDs: runIDs,
		Baseline: RequirementBaseline{Version: criteria.Version, Items: criteria.Items, Bounds: criteria.Bounds, Gate: criteria.Gate},
	}
	return json.Marshal(criteria)
}

// ChangeRequirements preserves the original execution snapshot until the new
// version is confirmed. The caller verifies run acknowledgements under its lock.
func ChangeRequirements(raw []byte, action string, stopped bool) ([]byte, string, error) {
	criteria, err := ParseRequirements(raw)
	if err != nil {
		return nil, "", err
	}
	change := criteria.Change
	if change == nil {
		return nil, "", errors.New("no requirements change is active")
	}
	status := "paused"
	switch action {
	case "reconcile":
		if change.Phase != "stopping" && change.Phase != "unknown" {
			return nil, "", errors.New("this change does not need stop reconciliation")
		}
		if !stopped {
			return nil, "", errors.New("executors have not confirmed that all affected runs stopped")
		}
		change.Phase = "paused"
	case "edit":
		if change.Phase != "paused" || !stopped {
			return nil, "", errors.New("wait for stop confirmation before editing requirements")
		}
		change.Phase = "editing"
		criteria.Version++
		criteria.Gate = Gates{}
		for index := range criteria.Items {
			criteria.Items[index].Checked = nil
			criteria.Items[index].Covers = []string{}
		}
	case "cancel":
		if !stopped || (change.Phase != "paused" && change.Phase != "editing") {
			return nil, "", errors.New("wait for stop reconciliation before restoring execution")
		}
		criteria.Items, criteria.Bounds, criteria.Gate = change.Baseline.Items, change.Baseline.Bounds, change.Baseline.Gate
		criteria.ResumeRuns = change.RunIDs
		criteria.ResumeError = ""
		status = change.PreviousStatus
		criteria.Change = nil
	default:
		return nil, "", errors.New("unknown requirements change action")
	}
	updated, err := json.Marshal(criteria)
	return updated, status, err
}
