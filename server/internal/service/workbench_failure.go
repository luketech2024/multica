package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

func taskRetryAttemptCeiling(reason string, task db.AgentTaskQueue) int32 {
	var context struct {
		WorkbenchMaxAttempts int32 `json:"workbench_max_attempts"`
	}
	if json.Unmarshal(task.Context, &context) == nil && context.WorkbenchMaxAttempts > 0 {
		return context.WorkbenchMaxAttempts
	}
	return retryAttemptCeiling(reason, task.MaxAttempts)
}

type workbenchFailure struct {
	before   db.Issue
	after    db.Issue
	activity db.ActivityLog
}

// The terminal run, blocked state and audit share the caller's transaction.
func blockFailedWorkbenchIssue(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) (*workbenchFailure, error) {
	if !task.IssueID.Valid || task.Status != "failed" {
		return nil, nil
	}
	issue, err := q.GetIssue(ctx, task.IssueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if workbench.HandoffJSON(issue.Properties) == nil {
		return nil, nil
	}
	issue, err = q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return nil, err
	}
	var handoff workbench.Handoff
	if json.Unmarshal(workbench.HandoffJSON(issue.Properties), &handoff) != nil || handoff.Superseded ||
		(issue.Status != "todo" && issue.Status != "in_progress") {
		return nil, nil
	}
	parent, err := q.GetIssue(ctx, issue.ParentIssueID)
	if err != nil {
		return nil, err
	}
	criteria, err := workbench.ParseRequirements(parent.AcceptanceCriteria)
	if err != nil || criteria.Change != nil || criteria.NeedsResplit || parent.Status == "paused" || parent.Status == "cancelled" {
		return nil, err
	}
	active, err := q.HasActiveTaskForIssue(ctx, issue.ID)
	if err != nil || active {
		return nil, err
	}
	after, err := q.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID, SourceTaskID: task.ID, Status: "blocked"})
	if err != nil {
		return nil, err
	}
	details, _ := json.Marshal(map[string]any{"from": issue.Status, "to": "blocked", "task_id": util.UUIDToString(task.ID), "reason": redact.Text(task.Error.String), "attempt": task.Attempt, "max_attempts": task.MaxAttempts})
	activity, err := q.CreateActivity(ctx, db.CreateActivityParams{ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID, IssueID: issue.ID,
		ActorType: util.StrToText("system"), Action: "status_changed", Details: details})
	if err != nil {
		return nil, err
	}
	return &workbenchFailure{before: issue, after: after, activity: activity}, nil
}

func (s *TaskService) publishWorkbenchFailure(ctx context.Context, change *workbenchFailure) {
	if change == nil {
		return
	}
	s.broadcastIssueUpdated(ctx, change.after, change.before.Status)
	activity := change.activity
	s.Bus.Publish(events.Event{Type: protocol.EventActivityCreated, WorkspaceID: util.UUIDToString(activity.WorkspaceID), ActorType: "system",
		Payload: map[string]any{"issue_id": util.UUIDToString(activity.IssueID), "entry": map[string]any{
			"type": "activity", "id": util.UUIDToString(activity.ID), "actor_type": "system", "actor_id": "",
			"action": activity.Action, "details": json.RawMessage(activity.Details), "created_at": util.TimestampToString(activity.CreatedAt),
		}},
	})
}
