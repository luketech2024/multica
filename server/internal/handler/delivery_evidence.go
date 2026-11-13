package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) SaveDeliveryEvidence(w http.ResponseWriter, r *http.Request) {
	r = h.withWakeupActor(r)
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	member, ok := h.requireWorkspaceRole(w, r, uuidToString(issue.WorkspaceID), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, uuidToString(member.UserID), uuidToString(issue.WorkspaceID))
	var input struct {
		ExpectedRevision int64                      `json:"expected_revision"`
		Evidence         workbench.DeliveryEvidence `json:"evidence"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&input) != nil || input.ExpectedRevision < 1 || input.Evidence.IssueRevision < 1 {
		writeError(w, http.StatusBadRequest, "parent and subtask revisions and delivery evidence are required")
		return
	}
	childID, ok := parseUUIDOrBadRequest(w, input.Evidence.IssueID, "evidence.issue_id")
	if !ok {
		return
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin delivery evidence update")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	current, err := q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if current.Revision != input.ExpectedRevision {
		writeRevisionConflict(w, "issue", current.ID, input.ExpectedRevision, current.Revision)
		return
	}
	if current.ParentIssueID.Valid || !current.ProjectID.Valid || (current.Status != "in_progress" && current.Status != "in_review" && current.Status != "blocked") {
		writeError(w, http.StatusConflict, "delivery evidence requires an executing or reviewable parent task")
		return
	}
	child, err := q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: childID, WorkspaceID: current.WorkspaceID})
	if err != nil || child.ParentIssueID != current.ID || child.ProjectID != current.ProjectID {
		writeError(w, http.StatusNotFound, "planned subtask not found")
		return
	}
	if actorType == "agent" && (child.AssigneeType.String != "agent" || uuidToString(child.AssigneeID) != actorID) {
		writeError(w, http.StatusForbidden, "agents can only report evidence for their assigned subtask")
		return
	}
	if child.Revision != input.Evidence.IssueRevision {
		writeRevisionConflict(w, "issue", child.ID, input.Evidence.IssueRevision, child.Revision)
		return
	}
	if child.Status != "done" && child.Status != "in_progress" && child.Status != "in_review" {
		writeError(w, http.StatusConflict, "delivery requires a started or completed subtask")
		return
	}
	active, err := q.WorkbenchPlanHasOtherActiveRuns(r.Context(), db.WorkbenchPlanHasOtherActiveRunsParams{ParentID: child.ID, WorkspaceID: current.WorkspaceID, SourceTaskID: h.workbenchSourceTaskID(r, child, actorType, actorID)})
	if err != nil || active {
		writeError(w, http.StatusConflict, "wait for subtask execution to settle before recording evidence")
		return
	}
	criteria, err := workbench.ParseRequirements(current.AcceptanceCriteria)
	var handoff workbench.Handoff
	if err != nil || json.Unmarshal(workbench.HandoffJSON(child.Properties), &handoff) != nil {
		writeError(w, http.StatusConflict, "the current execution plan is unavailable")
		return
	}
	evidence := input.Evidence
	if err := workbench.ValidateDeliveryEvidence(criteria, handoff, evidence); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	evidence.Version, evidence.Role, evidence.RepoID = criteria.Version, handoff.Role, handoff.RepoID
	evidence.DescriptionHash = workbench.DeliveryDescriptionHash(current.Description.String)
	evidence.RecordedAt, evidence.RecordedBy, evidence.ActorType = time.Now().UTC().Format(time.RFC3339Nano), actorID, actorType
	// Commit completion and its evidence together so the recorded revision remains current.
	if child.Status != "done" {
		child, err = q.UpdateIssue(r.Context(), db.UpdateIssueParams{
			ID: child.ID, Status: strToText("done"), ExpectedRevision: pgtype.Int8{Int64: child.Revision, Valid: true},
			AssigneeType: child.AssigneeType, AssigneeID: child.AssigneeID, ProjectID: child.ProjectID, ParentIssueID: child.ParentIssueID,
			StartDate: child.StartDate, DueDate: child.DueDate, Stage: child.Stage, DuplicateOfIssueID: child.DuplicateOfIssueID,
		})
		if err != nil {
			writeError(w, http.StatusConflict, "subtask completion could not be saved")
			return
		}
	}
	evidence.IssueRevision = child.Revision
	evidence.TaskHash = workbench.DeliveryTaskHash(workbench.DeliveryTask{Title: child.Title, Description: child.Description.String, Handoff: handoff})
	if criteria.Delivery == nil {
		criteria.Delivery = map[string]workbench.DeliveryEvidence{}
	}
	criteria.Delivery[evidence.IssueID] = evidence
	raw, err := json.Marshal(criteria)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode delivery evidence")
		return
	}
	after, err := q.UpdateIssue(r.Context(), db.UpdateIssueParams{
		ID: current.ID, AcceptanceCriteria: raw, ExpectedRevision: pgtype.Int8{Int64: current.Revision, Valid: true},
		AssigneeType: current.AssigneeType, AssigneeID: current.AssigneeID, ProjectID: current.ProjectID, ParentIssueID: current.ParentIssueID,
		StartDate: current.StartDate, DueDate: current.DueDate, Stage: current.Stage, DuplicateOfIssueID: current.DuplicateOfIssueID,
	})
	if err != nil {
		writeError(w, http.StatusConflict, "delivery evidence could not be saved")
		return
	}
	activity, err := recordRequirementsActivity(r.Context(), q, current, after)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record delivery evidence audit")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit delivery evidence")
		return
	}
	h.publishRequirementsActivity(activity)
	h.publishIssueSnapshots(r.Context(), []db.Issue{child, after}, actorType, actorID)
	response := issueToResponse(after, h.getIssuePrefix(r.Context(), after.WorkspaceID))
	h.fillStatusCategory(r.Context(), after.WorkspaceID, &response)
	writeJSON(w, http.StatusOK, response)
}
