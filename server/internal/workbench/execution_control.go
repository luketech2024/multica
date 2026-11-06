package workbench

import (
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

type ExecutionPause struct {
	V              int      `json:"v"`
	Phase          string   `json:"phase"`
	PreviousStatus string   `json:"previous_status"`
	RequestedAt    string   `json:"requested_at"`
	RequestedBy    string   `json:"requested_by"`
	RunIDs         []string `json:"run_ids"`
	PendingRuns    []string `json:"pending_runs"`
	ResumeError    string   `json:"resume_error"`
}

func ExecutionPauseJSON(properties []byte) json.RawMessage {
	var values map[string]json.RawMessage
	if json.Unmarshal(properties, &values) != nil {
		return nil
	}
	return values["workbench_pause"]
}

func ParseExecutionPause(properties []byte) (*ExecutionPause, error) {
	raw := ExecutionPauseJSON(properties)
	if len(raw) == 0 {
		return nil, nil
	}
	var pause ExecutionPause
	if json.Unmarshal(raw, &pause) != nil || pause.V != 1 {
		return nil, errors.New("unsupported execution pause record")
	}
	for _, id := range slices.Concat(pause.RunIDs, pause.PendingRuns) {
		if _, err := uuid.Parse(id); err != nil {
			return nil, errors.New("invalid execution reference in pause record")
		}
	}
	return &pause, nil
}

func NewExecutionPause(criteriaRaw, properties []byte, status, actor string, runs []string, stopped bool, now time.Time) (*ExecutionPause, error) {
	var criteria Requirements
	isRequirement := json.Unmarshal(criteriaRaw, &criteria) == nil && criteria.V == 1
	var handoff Handoff
	isSubtask := json.Unmarshal(HandoffJSON(properties), &handoff) == nil && handoff.V == 2 && !handoff.Superseded
	if handoff.Superseded || (!isRequirement && !isSubtask) {
		return nil, errors.New("execution control requires a current workbench task")
	}
	if criteria.Change != nil || len(criteria.ResumeRuns) > 0 {
		return nil, errors.New("finish the requirements change before pausing execution")
	}
	if existing, err := ParseExecutionPause(properties); err != nil || existing != nil {
		return nil, errors.New("an execution pause is already active")
	}
	switch status {
	case "backlog", "todo", "in_progress", "in_review", "blocked":
	default:
		return nil, errors.New("this task has no executable state to pause")
	}
	phase := "stopping"
	if stopped {
		phase = "paused"
	}
	if runs == nil {
		runs = []string{}
	}
	return &ExecutionPause{V: 1, Phase: phase, PreviousStatus: status, RequestedAt: now.UTC().Format(time.RFC3339Nano), RequestedBy: actor,
		RunIDs: runs, PendingRuns: []string{}}, nil
}
