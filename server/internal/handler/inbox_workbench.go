package handler

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func applyInboxWorkbench(item *InboxItemResponse, issue db.Issue) {
	criteria, err := workbench.ParseRequirements(issue.AcceptanceCriteria)
	item.Workbench = (err == nil && criteria.Team != nil) || workbench.HandoffJSON(issue.Properties) != nil
	item.WorkbenchAction = ""
	if !item.Workbench || issue.AssigneeType.String != "member" || uuidToString(issue.AssigneeID) != item.RecipientID {
		return
	}
	switch issue.Status {
	case "in_review":
		item.WorkbenchAction = "delivery"
	case "blocked":
		item.WorkbenchAction = "blocked"
	case "todo", "paused":
		if err == nil && !issue.ParentIssueID.Valid && len(criteria.Items) > 0 && criteria.Gate.Spec == nil &&
			(criteria.Change == nil || criteria.Change.Phase == "editing") {
			item.WorkbenchAction = "requirements"
		}
	}
}

func (h *Handler) enrichInboxWorkbench(ctx context.Context, workspaceID pgtype.UUID, items []InboxItemResponse) error {
	ids := make([]pgtype.UUID, 0, len(items))
	for _, item := range items {
		if item.IssueID != nil {
			ids = append(ids, parseUUID(*item.IssueID))
		}
	}
	if len(ids) == 0 {
		return nil
	}
	issues, err := h.Queries.ListInboxWorkbenchIssues(ctx, db.ListInboxWorkbenchIssuesParams{WorkspaceID: workspaceID, IssueIds: ids})
	if err != nil {
		return err
	}
	byID := make(map[string]db.Issue, len(issues))
	for _, issue := range issues {
		byID[uuidToString(issue.ID)] = issue
	}
	for index := range items {
		if items[index].IssueID == nil {
			continue
		}
		if issue, exists := byID[*items[index].IssueID]; exists {
			applyInboxWorkbench(&items[index], issue)
		}
	}
	return nil
}
