package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	inboxwriter "github.com/multica-ai/multica/server/internal/inbox"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// ReconcileWorkbenchReviews recovers evidence aggregation after restart or a
// late executor acknowledgement. Parent locking serializes concurrent workers.
func (s *TaskService) ReconcileWorkbenchReviews(ctx context.Context) error {
	cursor := pgtype.UUID{Valid: true}
	var failures []error
	for {
		issues, err := s.Queries.ListWorkbenchReviewCandidates(ctx, cursor)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, issue := range issues {
			cursor = issue.ID
			if err := s.reconcileWorkbenchReview(ctx, issue); err != nil {
				failures = append(failures, fmt.Errorf("review workbench issue %s: %w", util.UUIDToString(issue.ID), err))
			}
		}
		if len(issues) < 100 {
			return errors.Join(failures...)
		}
	}
}

func (s *TaskService) reconcileWorkbenchReview(ctx context.Context, candidate db.Issue) error {
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
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(issue.Properties, &properties); err != nil {
		return err
	}
	if issue.ParentIssueID.Valid || !issue.ProjectID.Valid || (issue.Status != "todo" && issue.Status != "in_progress" && issue.Status != "in_review") || properties["workbench_pause"] != nil {
		return nil
	}
	criteria, err := workbench.ParseRequirements(issue.AcceptanceCriteria)
	if err != nil {
		return err
	}
	if issue.Status == "todo" && !issue.AssigneeID.Valid {
		return nil
	}
	children, err := q.ListWorkbenchPlanChildren(ctx, db.ListWorkbenchPlanChildrenParams{ParentID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return err
	}
	tasks := make([]workbench.DeliveryTask, 0, len(children))
	for _, child := range children {
		var handoff workbench.Handoff
		if json.Unmarshal(workbench.HandoffJSON(child.Properties), &handoff) == nil {
			tasks = append(tasks, workbench.DeliveryTask{ID: util.UUIDToString(child.ID), Title: child.Title, Description: child.Description.String, Revision: child.Revision, Status: child.Status, Handoff: handoff})
		}
	}
	active, err := q.WorkbenchPlanHasActiveRuns(ctx, db.WorkbenchPlanHasActiveRunsParams{ParentID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return err
	}
	status, reason := workbench.DeliveryTransition(criteria, tasks, active, issue.Description.String, issue.Status)
	if status == "" {
		return nil
	}
	assignee, assigneeType := issue.AssigneeID, issue.AssigneeType
	if status != "in_progress" && (issue.AssigneeType.String != "member" || !assignee.Valid) {
		assignee, err = util.ParseUUID(criteria.Gate.Spec.PassedBy)
		if err != nil {
			return nil
		}
		assigneeType = util.StrToText("member")
	}
	if status != "in_progress" {
		member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: assignee, WorkspaceID: issue.WorkspaceID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if member.Role != "owner" && member.Role != "admin" && member.Role != "member" {
			return nil
		}
	}
	if issue.Status == status && issue.AssigneeType.String == "member" && issue.AssigneeID == assignee {
		return nil
	}
	after, err := q.UpdateIssue(ctx, db.UpdateIssueParams{
		ID: issue.ID, Status: util.StrToText(status), ExpectedRevision: pgtype.Int8{Int64: issue.Revision, Valid: true},
		AssigneeType: assigneeType, AssigneeID: assignee, ProjectID: issue.ProjectID, ParentIssueID: issue.ParentIssueID,
		StartDate: issue.StartDate, DueDate: issue.DueDate, Stage: issue.Stage, DuplicateOfIssueID: issue.DuplicateOfIssueID,
	})
	if err != nil {
		return err
	}
	details, _ := json.Marshal(map[string]string{
		"from": issue.Status, "to": status, "reason": reason, "source": "workbench_delivery",
		"previous_assignee_type": issue.AssigneeType.String, "previous_assignee_id": util.UUIDToString(issue.AssigneeID),
		"assignee_id": util.UUIDToString(assignee), "requirement_version": fmt.Sprint(criteria.Version),
	})
	activity, err := q.CreateActivity(ctx, db.CreateActivityParams{ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID, IssueID: issue.ID,
		ActorType: util.StrToText("system"), Action: "status_changed", Details: details})
	if err != nil {
		return err
	}
	notificationType := "review_requested"
	if status == "blocked" {
		notificationType = "agent_blocked"
	}
	var item db.InboxItem
	var inserted bool
	if status != "in_progress" {
		item, inserted, err = inboxwriter.CreateIfEligible(ctx, q, db.CreateInboxItemParams{
			ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, RecipientType: "member", RecipientID: assignee,
			Type: notificationType, Severity: "action_required", Title: issue.Title, ActorType: util.StrToText("system"), Details: details,
		})
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.broadcastIssueUpdated(ctx, after, issue.Status)
	wakeups := &IssueWakeupService{Tasks: s}
	wakeups.publishWakeupActivities(wakeupActivity{row: activity, actorType: "system", details: details})
	if inserted {
		wakeups.publishSystemInbox(item, after.Status)
	}
	return nil
}
