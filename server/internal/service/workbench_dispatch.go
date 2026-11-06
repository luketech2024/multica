package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// DispatchWorkbenchPlans recovers initial dispatch from persisted plans. A run,
// including a failed or cancelled run, leaves retries to the execution lifecycle.
func (s *TaskService) DispatchWorkbenchPlans(ctx context.Context) error {
	cursor := pgtype.UUID{Valid: true}
	var failures []error
	for {
		candidates, err := s.Queries.ListWorkbenchDispatchCandidates(ctx, cursor)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, candidate := range candidates {
			cursor = candidate.ID
			if err := s.dispatchWorkbenchIssue(ctx, candidate); err != nil {
				failures = append(failures, fmt.Errorf("dispatch workbench issue %s: %w", util.UUIDToString(candidate.ID), err))
			}
		}
		if len(candidates) < 100 {
			return errors.Join(failures...)
		}
	}
}

func (s *TaskService) dispatchWorkbenchIssue(ctx context.Context, candidate db.Issue) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	issue, err := q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: candidate.ID, WorkspaceID: candidate.WorkspaceID})
	if err != nil {
		return err
	}
	if issue.Status != "todo" || issue.AssigneeType.String != "agent" {
		return nil
	}
	_, err = q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: issue.CreatorID, WorkspaceID: issue.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	hasRun, err := q.WorkbenchIssueHasRun(ctx, issue.ID)
	if err != nil || hasRun {
		return err
	}
	agent, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: issue.AssigneeID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return err
	}
	if agent.ArchivedAt.Valid || !agent.RuntimeID.Valid || issue.CreatorType != "member" ||
		!CanMemberInvokeAgent(ctx, q, agent, issue.CreatorID, issue.WorkspaceID) {
		return nil
	}
	// The transaction-bound service has no external listeners or daemon wakeup.
	// Only the committed row is published by the owning service below.
	writer := &TaskService{Queries: q, Bus: events.New(), FeatureFlags: s.FeatureFlags, Composio: s.Composio}
	task, err := writer.EnqueueTaskForIssue(ctx, issue)
	if err != nil {
		var constraint *pgconn.PgError
		if errors.As(err, &constraint) && constraint.ConstraintName == "workbench_execution_gate" {
			return nil
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, task)
	s.NotifyTaskEnqueued(ctx, task)
	return nil
}
