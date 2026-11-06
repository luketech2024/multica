package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Native reruns retain source sessions and branches. The persisted pending list
// and source linkage let a retry recover after a run was created but before the
// issue acknowledgement was saved.
func (h *Handler) resumeWorkbenchRequirements(r *http.Request, issue db.Issue, memberID pgtype.UUID) (db.Issue, error) {
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		return db.Issue{}, err
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if err := q.LockWorkbenchResume(r.Context(), uuidToString(issue.ID)); err != nil {
		return db.Issue{}, err
	}
	current, err := q.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return db.Issue{}, err
	}
	criteria, err := workbench.ParseRequirements(current.AcceptanceCriteria)
	if err != nil {
		return db.Issue{}, err
	}
	if len(criteria.ResumeRuns) == 0 {
		return current, nil
	}
	if current.Status != "in_progress" || criteria.Change != nil {
		return db.Issue{}, errors.New("this task is not ready to resume")
	}
	pending, message, err := h.resumeWorkbenchRuns(r, q, current, criteria.ResumeRuns, memberID)
	if err != nil {
		return db.Issue{}, err
	}
	criteria.ResumeRuns, criteria.ResumeError = pending, message
	raw, err := json.Marshal(criteria)
	if err != nil {
		return db.Issue{}, err
	}
	after, err := q.UpdateIssue(r.Context(), db.UpdateIssueParams{
		ID: current.ID, AcceptanceCriteria: raw, ExpectedRevision: pgtype.Int8{Int64: current.Revision, Valid: true},
		AssigneeType: current.AssigneeType, AssigneeID: current.AssigneeID,
		ProjectID: current.ProjectID, ParentIssueID: current.ParentIssueID,
		StartDate: current.StartDate, DueDate: current.DueDate, Stage: current.Stage,
		DuplicateOfIssueID: current.DuplicateOfIssueID,
	})
	if err != nil {
		return db.Issue{}, err
	}
	activity, err := recordRequirementsActivity(r.Context(), q, current, after)
	if err != nil {
		return db.Issue{}, err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return db.Issue{}, err
	}
	h.publishRequirementsActivity(activity)
	return after, nil
}
