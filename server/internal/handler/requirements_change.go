package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) ChangeIssueRequirements(w http.ResponseWriter, r *http.Request) {
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
	if actorType != "member" {
		writeError(w, http.StatusForbidden, "only a member can request requirements changes")
		return
	}
	var input struct {
		Action           string `json:"action"`
		Reason           string `json:"reason"`
		ExpectedRevision int64  `json:"expected_revision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input) != nil || input.ExpectedRevision < 1 {
		writeError(w, http.StatusBadRequest, "an action and expected_revision are required")
		return
	}
	if input.Action == "resume" {
		if issue.Revision != input.ExpectedRevision {
			writeRevisionConflict(w, "issue", issue.ID, input.ExpectedRevision, issue.Revision)
			return
		}
		after, err := h.resumeWorkbenchRequirements(r, issue, member.UserID)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		h.publishIssueSnapshots(r.Context(), []db.Issue{after}, actorType, actorID)
		response := issueToResponse(after, h.getIssuePrefix(r.Context(), after.WorkspaceID))
		h.fillStatusCategory(r.Context(), after.WorkspaceID, &response)
		writeJSON(w, http.StatusOK, response)
		return
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin requirements change")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if err := assertIssueStatusStillActive(r.Context(), q, issue.WorkspaceID, "paused"); err != nil {
		writeError(w, http.StatusConflict, "paused status is unavailable")
		return
	}
	current, err := q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}
	if current.Revision != input.ExpectedRevision {
		writeRevisionConflict(w, "issue", current.ID, input.ExpectedRevision, current.Revision)
		return
	}
	var updated []byte
	var cancelled []db.AgentTaskQueue
	status := "paused"
	if input.Action == "start" {
		cancelled, err = q.CancelWorkbenchChangeRuns(r.Context(), db.CancelWorkbenchChangeRunsParams{IssueID: current.ID, WorkspaceID: current.WorkspaceID, ActorID: member.UserID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to stop affected runs")
			return
		}
		runIDs := make([]string, 0, len(cancelled))
		allStopped := true
		for _, run := range cancelled {
			var result struct {
				State string `json:"workbench_stop_state"`
			}
			_ = json.Unmarshal(run.Result, &result)
			runIDs = append(runIDs, uuidToString(run.ID))
			allStopped = allStopped && result.State == "stopped"
			if err = q.MarkPiTaskProcessCleanupPending(r.Context(), run.ID); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to record stop request")
				return
			}
		}
		updated, err = workbench.StartRequirementsChange(current.AcceptanceCriteria, current.Status, input.Reason, actorID, runIDs, time.Now())
		if err == nil && allStopped && len(runIDs) > 0 {
			updated, _, err = workbench.ChangeRequirements(updated, "reconcile", true)
		}
		if err == nil {
			err = service.SettleTerminalTaskState(r.Context(), q, cancelled...)
		}
	} else {
		criteria, parseErr := workbench.ParseRequirements(current.AcceptanceCriteria)
		if parseErr != nil || criteria.Change == nil || current.Status != "paused" {
			writeError(w, http.StatusConflict, "no paused requirements change is active")
			return
		}
		runIDs := make([]pgtype.UUID, 0, len(criteria.Change.RunIDs))
		for _, id := range criteria.Change.RunIDs {
			runIDs = append(runIDs, parseUUID(id))
		}
		stopped, readErr := q.WorkbenchRunsStopped(r.Context(), db.WorkbenchRunsStoppedParams{RunIds: runIDs, WorkspaceID: current.WorkspaceID})
		if readErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to read stop acknowledgements")
			return
		}
		updated, status, err = workbench.ChangeRequirements(current.AcceptanceCriteria, input.Action, stopped)
		if input.Action == "reconcile" && !stopped {
			unknown, readErr := q.WorkbenchRunsUnconfirmed(r.Context(), db.WorkbenchRunsUnconfirmedParams{RunIds: runIDs, WorkspaceID: current.WorkspaceID})
			if readErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to reconcile stop results")
				return
			}
			if unknown {
				criteria.Change.Phase = "unknown"
				updated, err = json.Marshal(criteria)
				status = "paused"
			}
		}
	}
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	after, err := q.UpdateIssue(r.Context(), db.UpdateIssueParams{
		ID: current.ID, AcceptanceCriteria: updated, Status: pgtype.Text{String: status, Valid: true},
		ExpectedRevision: pgtype.Int8{Int64: current.Revision, Valid: true},
		AssigneeType:     current.AssigneeType, AssigneeID: current.AssigneeID,
		ProjectID: current.ProjectID, ParentIssueID: current.ParentIssueID,
		StartDate: current.StartDate, DueDate: current.DueDate, Stage: current.Stage,
		DuplicateOfIssueID: current.DuplicateOfIssueID,
	})
	if err != nil {
		if !writeIssueStatusRaceError(w, err) {
			writeError(w, http.StatusConflict, "requirements change could not be applied")
		}
		return
	}
	activity, err := recordRequirementsActivity(r.Context(), q, current, after)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record requirements change")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit requirements change")
		return
	}
	h.broadcastCancelledWakeups(r.Context(), current.WorkspaceID, cancelled)
	h.publishRequirementsActivity(activity)
	if input.Action == "cancel" {
		resumed, resumeErr := h.resumeWorkbenchRequirements(r, after, member.UserID)
		if resumeErr == nil {
			after = resumed
		} else if latest, readErr := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: after.ID, WorkspaceID: after.WorkspaceID}); readErr == nil {
			after = latest
		}
	}
	h.publishIssueSnapshots(r.Context(), []db.Issue{after}, actorType, actorID)
	response := issueToResponse(after, h.getIssuePrefix(r.Context(), after.WorkspaceID))
	h.fillStatusCategory(r.Context(), after.WorkspaceID, &response)
	writeJSON(w, http.StatusOK, response)
}
