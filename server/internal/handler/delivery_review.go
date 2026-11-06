package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type deliveryReviewState struct {
	Revision int64                       `json:"revision"`
	Blockers []workbench.DeliveryBlocker `json:"blockers"`
	Checks   []workbench.DeliveryCheck   `json:"checks"`
}

func readDeliveryReview(ctx context.Context, q *db.Queries, issue db.Issue, criteria workbench.Requirements) (deliveryReviewState, error) {
	children, err := q.ListWorkbenchPlanChildren(ctx, db.ListWorkbenchPlanChildrenParams{ParentID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return deliveryReviewState{}, err
	}
	tasks := make([]workbench.DeliveryTask, 0, len(children))
	for _, child := range children {
		var handoff workbench.Handoff
		if json.Unmarshal(workbench.HandoffJSON(child.Properties), &handoff) != nil {
			continue
		}
		tasks = append(tasks, workbench.DeliveryTask{ID: uuidToString(child.ID), Revision: child.Revision, Status: child.Status, Handoff: handoff})
	}
	active, err := q.WorkbenchPlanHasActiveRuns(ctx, db.WorkbenchPlanHasActiveRunsParams{ParentID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return deliveryReviewState{}, err
	}
	blockers := workbench.DeliveryBlockers(criteria, tasks, active, issue.Description.String)
	titles := make(map[string]string, len(children))
	for _, child := range children {
		titles[uuidToString(child.ID)] = child.Title
	}
	for index := range blockers {
		blockers[index].Title = titles[blockers[index].IssueID]
	}
	return deliveryReviewState{Revision: issue.Revision, Blockers: blockers, Checks: workbench.DeliveryChecks(criteria)}, nil
}

func (h *Handler) ReviewIssueDelivery(w http.ResponseWriter, r *http.Request) {
	r = h.withWakeupActor(r)
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var input struct {
		ExpectedRevision int64    `json:"expected_revision"`
		Action           string   `json:"action"`
		Reason           string   `json:"reason"`
		Checks           []string `json:"checks"`
	}
	actorType, actorID := "", ""
	if r.Method == http.MethodPost {
		member, ok := h.requireWorkspaceRole(w, r, uuidToString(issue.WorkspaceID), "workspace not found", "owner", "admin", "member")
		if !ok {
			return
		}
		actorType, actorID = h.resolveActor(r, uuidToString(member.UserID), uuidToString(issue.WorkspaceID))
		if actorType != "member" {
			writeError(w, http.StatusForbidden, "only the assigned member can review delivery")
			return
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&input) != nil || input.ExpectedRevision < 1 {
			writeError(w, http.StatusBadRequest, "a review action and expected_revision are required")
			return
		}
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin delivery review")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	current, err := q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	criteria, err := workbench.ParseRequirements(current.AcceptanceCriteria)
	if err != nil || !current.ProjectID.Valid || current.ParentIssueID.Valid || criteria.Team == nil {
		writeError(w, http.StatusConflict, "delivery review requires a project parent task")
		return
	}
	state, err := readDeliveryReview(r.Context(), q, current, criteria)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read delivery evidence")
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, state)
		return
	}
	if current.AssigneeType.String != "member" || uuidToString(current.AssigneeID) != actorID {
		writeError(w, http.StatusForbidden, "only the assigned member can review delivery")
		return
	}
	if current.Revision != input.ExpectedRevision {
		writeRevisionConflict(w, "issue", current.ID, input.ExpectedRevision, current.Revision)
		return
	}
	criteria, status, err := workbench.ReviewDelivery(criteria, current.Status, input.Action, input.Reason, actorID, input.Checks, state.Blockers, time.Now())
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err := assertIssueStatusStillActive(r.Context(), q, current.WorkspaceID, status); err != nil {
		writeError(w, http.StatusConflict, "the target task status is unavailable")
		return
	}
	raw, err := json.Marshal(criteria)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode acceptance decision")
		return
	}
	after, err := q.UpdateIssue(r.Context(), db.UpdateIssueParams{
		ID: current.ID, AcceptanceCriteria: raw, Status: strToText(status), ExpectedRevision: pgtype.Int8{Int64: current.Revision, Valid: true},
		AssigneeType: current.AssigneeType, AssigneeID: current.AssigneeID, ProjectID: current.ProjectID, ParentIssueID: current.ParentIssueID,
		StartDate: current.StartDate, DueDate: current.DueDate, Stage: current.Stage, DuplicateOfIssueID: current.DuplicateOfIssueID,
	})
	if err != nil {
		if !writeIssueStatusRaceError(w, err) {
			writeError(w, http.StatusConflict, "delivery review could not be saved")
		}
		return
	}
	activity, err := recordRequirementsActivity(r.Context(), q, current, after)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record delivery review audit")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit delivery review")
		return
	}
	h.publishRequirementsActivity(activity)
	h.publishIssueSnapshots(r.Context(), []db.Issue{after}, actorType, actorID)
	response := issueToResponse(after, h.getIssuePrefix(r.Context(), after.WorkspaceID))
	h.fillStatusCategory(r.Context(), after.WorkspaceID, &response)
	writeJSON(w, http.StatusOK, response)
}
