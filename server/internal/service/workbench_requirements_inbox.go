package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	inboxwriter "github.com/multica-ai/multica/server/internal/inbox"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (s *TaskService) ReconcileWorkbenchRequirements(ctx context.Context) error {
	cursor := pgtype.UUID{Valid: true}
	var failures []error
	for {
		issues, err := s.Queries.ListWorkbenchRequirementCandidates(ctx, cursor)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, issue := range issues {
			cursor = issue.ID
			if err := s.reconcileWorkbenchRequirementNotice(ctx, issue); err != nil {
				failures = append(failures, fmt.Errorf("notify workbench requirement %s: %w", util.UUIDToString(issue.ID), err))
			}
		}
		if len(issues) < 100 {
			return errors.Join(failures...)
		}
	}
}

func (s *TaskService) reconcileWorkbenchRequirementNotice(ctx context.Context, candidate db.Issue) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	issue, err := q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: candidate.ID, WorkspaceID: candidate.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	criteria, err := workbench.ParseRequirements(issue.AcceptanceCriteria)
	if err != nil {
		return err
	}
	needsConfirmation := workbench.RequirementsNeedConfirmation(criteria, issue.Status, issue.ParentIssueID.Valid) && issue.AssigneeType.String == "member"
	if needsConfirmation {
		member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: issue.AssigneeID, WorkspaceID: issue.WorkspaceID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		needsConfirmation = err == nil && (member.Role == "owner" || member.Role == "admin" || member.Role == "member")
	}
	notices, err := q.ListWorkbenchRequirementNotices(ctx, db.ListWorkbenchRequirementNoticesParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
	if err != nil {
		return err
	}
	version := fmt.Sprint(criteria.Version)
	alreadyNotified := false
	var handled []db.InboxItem
	for _, notice := range notices {
		var details map[string]string
		if err := json.Unmarshal(notice.Details, &details); err != nil {
			return err
		}
		current := needsConfirmation && notice.RecipientID == issue.AssigneeID && details["requirement_version"] == version
		alreadyNotified = alreadyNotified || current
		if !current && !notice.Read {
			read, err := q.MarkInboxRead(ctx, notice.ID)
			if err != nil {
				return err
			}
			handled = append(handled, read)
		}
	}
	var created db.InboxItem
	var inserted bool
	if needsConfirmation && !alreadyNotified {
		details, _ := json.Marshal(map[string]string{"source": "workbench_requirements", "reason": "requirements_ready", "requirement_version": version})
		created, inserted, err = inboxwriter.CreateIfEligible(ctx, q, db.CreateInboxItemParams{
			ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, RecipientType: "member", RecipientID: issue.AssigneeID,
			Type: "review_requested", Severity: "action_required", Title: issue.Title, ActorType: util.StrToText("system"), Details: details,
		})
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if inserted {
		(&IssueWakeupService{Tasks: s}).publishSystemInbox(created, issue.Status)
	}
	if s.Bus != nil {
		for _, notice := range handled {
			s.Bus.Publish(events.Event{Type: protocol.EventInboxRead, WorkspaceID: util.UUIDToString(issue.WorkspaceID), ActorType: "system",
				Payload: map[string]any{"item_id": util.UUIDToString(notice.ID), "recipient_id": util.UUIDToString(notice.RecipientID)}})
		}
	}
	return nil
}
