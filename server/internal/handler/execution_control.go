package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func (h *Handler) ControlIssueExecution(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusForbidden, "only a member can control execution")
		return
	}
	var input struct {
		Action           string `json:"action"`
		ExpectedRevision int64  `json:"expected_revision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || input.ExpectedRevision < 1 {
		writeError(w, http.StatusBadRequest, "an action and expected_revision are required")
		return
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin execution control")
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
	pause, err := workbench.ParseExecutionPause(current.Properties)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	status := current.Status
	var stoppedRuns []db.AgentTaskQueue
	switch input.Action {
	case "pause":
		// Validate before requesting any executor stop.
		if _, err = workbench.NewExecutionPause(current.AcceptanceCriteria, current.Properties, current.Status, actorID, nil, true, time.Now()); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		stoppedRuns, err = q.CancelWorkbenchChangeRuns(r.Context(), db.CancelWorkbenchChangeRunsParams{IssueID: current.ID, WorkspaceID: current.WorkspaceID, ActorID: member.UserID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to request execution stop")
			return
		}
		ids := make([]string, 0, len(stoppedRuns))
		allStopped := true
		for _, run := range stoppedRuns {
			var result struct {
				State string `json:"workbench_stop_state"`
			}
			_ = json.Unmarshal(run.Result, &result)
			ids = append(ids, uuidToString(run.ID))
			allStopped = allStopped && result.State == "stopped"
			if err = q.MarkPiTaskProcessCleanupPending(r.Context(), run.ID); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to persist stop request")
				return
			}
		}
		pause, err = workbench.NewExecutionPause(current.AcceptanceCriteria, current.Properties, current.Status, actorID, ids, allStopped, time.Now())
		if err == nil {
			err = service.SettleTerminalTaskState(r.Context(), q, stoppedRuns...)
		}
		status = "paused"
		if err == nil {
			_, err = tx.Exec(r.Context(), `UPDATE task_user_input SET status='cancelled',updated_at=now()
				WHERE workspace_id=$1 AND status='pending' AND (issue_id=$2 OR issue_id IN (SELECT id FROM issue WHERE parent_issue_id=$2 AND workspace_id=$1))`, current.WorkspaceID, current.ID)
		}
	case "reconcile", "resume":
		if pause == nil {
			writeError(w, http.StatusConflict, "no execution pause is active")
			return
		}
		if pause.Phase == "resuming" && input.Action == "resume" {
			break
		}
		if current.Status != "paused" {
			writeError(w, http.StatusConflict, "task is no longer paused")
			return
		}
		ids := make([]pgtype.UUID, 0, len(pause.RunIDs))
		for _, id := range pause.RunIDs {
			ids = append(ids, parseUUID(id))
		}
		stopped, readErr := q.WorkbenchRunsStopped(r.Context(), db.WorkbenchRunsStoppedParams{RunIds: ids, WorkspaceID: current.WorkspaceID})
		if readErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to read stop acknowledgements")
			return
		}
		if input.Action == "resume" {
			if !stopped || pause.Phase != "paused" {
				writeError(w, http.StatusConflict, "confirm all execution has stopped before resuming")
				return
			}
			status, pause.Phase, pause.PendingRuns = pause.PreviousStatus, "resuming", pause.RunIDs
		} else if stopped {
			pause.Phase = "paused"
		} else {
			unknown, readErr := q.WorkbenchRunsUnconfirmed(r.Context(), db.WorkbenchRunsUnconfirmedParams{RunIds: ids, WorkspaceID: current.WorkspaceID})
			if readErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to reconcile stop results")
				return
			}
			if unknown {
				pause.Phase = "unknown"
			}
		}
	default:
		err = errors.New("unsupported execution action")
	}
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	raw, _ := json.Marshal(pause)
	after, err := q.UpdateWorkbenchPause(r.Context(), db.UpdateWorkbenchPauseParams{ID: current.ID, WorkspaceID: current.WorkspaceID, ExpectedRevision: current.Revision, Pause: raw, Status: status})
	if err != nil {
		writeError(w, http.StatusConflict, "execution control could not be applied")
		return
	}
	activity, err := recordExecutionControlActivity(r, q, current, after, actorType, member.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record execution control")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit execution control")
		return
	}
	h.broadcastCancelledWakeups(r.Context(), current.WorkspaceID, stoppedRuns)
	h.publishRequirementsActivity(activity)
	if input.Action == "resume" {
		resumed, resumeErr := h.resumeWorkbenchPause(r, after, member.UserID)
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

func recordExecutionControlActivity(r *http.Request, q *db.Queries, before, after db.Issue, actor string, member pgtype.UUID) (db.ActivityLog, error) {
	details, _ := json.Marshal(map[string]any{"from": before.Status, "to": after.Status, "pause": workbench.ExecutionPauseJSON(after.Properties), "revision": after.Revision})
	return q.CreateActivity(r.Context(), db.CreateActivityParams{ID: dbid.NewV7(), WorkspaceID: after.WorkspaceID, IssueID: after.ID, ActorType: strToText(actor), ActorID: member, Action: "execution_control", Details: details})
}
