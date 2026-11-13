package workbench

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ExecutionPlanRequest struct {
	ExpectedRevision int64      `json:"expected_revision"`
	Version          int        `json:"version"`
	Key              string     `json:"idempotency_key"`
	Tasks            []PlanTask `json:"tasks"`
}

func ParseExecutionPlanRequest(raw []byte) (ExecutionPlanRequest, error) {
	var input ExecutionPlanRequest
	if err := json.Unmarshal(raw, &input); err != nil {
		var field *json.UnmarshalTypeError
		if errors.As(err, &field) {
			if field.Field == "tasks.expected" {
				return input, errors.New(`tasks[].expected must be an array of strings, for example ["deployment report"], not a single string`)
			}
			return input, fmt.Errorf("invalid execution plan field %s: expected %s, got %s", field.Field, field.Type, field.Value)
		}
		return input, fmt.Errorf("invalid execution plan JSON: %w", err)
	}
	if input.ExpectedRevision < 1 {
		return input, errors.New("expected_revision must be a positive integer from the current issue")
	}
	if input.Version < 1 {
		return input, errors.New("version must be the positive requirement version")
	}
	if strings.TrimSpace(input.Key) == "" || len(input.Key) > 100 {
		return input, errors.New("idempotency_key must contain 1 to 100 bytes")
	}
	if len(input.Tasks) == 0 || len(input.Tasks) > 100 {
		return input, errors.New("tasks must be a top-level array of 1 to 100 subtasks; do not wrap fields in plan")
	}
	for i, task := range input.Tasks {
		if len(task.Expected) == 0 {
			return input, fmt.Errorf("tasks[%d].expected must contain at least one deliverable string", i)
		}
	}
	return input, nil
}
