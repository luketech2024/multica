package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) resumeWorkbenchPause(r *http.Request, issue db.Issue, memberID pgtype.UUID) (db.Issue, error) {
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		return db.Issue{}, err
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if err = q.LockWorkbenchResume(r.Context(), uuidToString(issue.ID)); err != nil {
		return db.Issue{}, err
	}
	current, err := q.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return db.Issue{}, err
	}
	pause, err := workbench.ParseExecutionPause(current.Properties)
	if err != nil {
		return db.Issue{}, err
	}
	if pause == nil {
		return current, nil
	}
	if pause.Phase != "resuming" || current.Status != pause.PreviousStatus {
		return db.Issue{}, errors.New("this execution pause is not ready to resume")
	}
	pending, message, err := h.resumeWorkbenchRuns(r, q, current, pause.PendingRuns, memberID)
	if err != nil {
		return db.Issue{}, err
	}
	pause.PendingRuns, pause.ResumeError = pending, message
	if len(pending) == 0 {
		pause = nil
	}
	raw, _ := json.Marshal(pause)
	after, err := q.UpdateWorkbenchPause(r.Context(), db.UpdateWorkbenchPauseParams{ID: current.ID, WorkspaceID: current.WorkspaceID, ExpectedRevision: current.Revision, Pause: raw, Status: current.Status})
	if err != nil {
		return db.Issue{}, err
	}
	activity, err := recordExecutionControlActivity(r, q, current, after, "member", memberID)
	if err != nil {
		return db.Issue{}, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return db.Issue{}, err
	}
	h.publishRequirementsActivity(activity)
	return after, nil
}

// The source run link also recovers a resume committed before its acknowledgement.
func (h *Handler) resumeWorkbenchRuns(r *http.Request, q *db.Queries, issue db.Issue, runIDs []string, memberID pgtype.UUID) ([]string, string, error) {
	workspaceID, actorID := uuidToString(issue.WorkspaceID), uuidToString(memberID)
	canInvoke := func(agent db.Agent) bool {
		return h.canInvokeAgent(r.Context(), agent, "member", actorID, actorID, workspaceID)
	}
	pending := []string{}
	message := ""
	for _, id := range runIDs {
		sourceID := parseUUID(id)
		_, err := q.FindWorkbenchResumedRun(r.Context(), sourceID)
		if err == nil {
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, "", err
		}
		source, err := q.GetAgentTaskInWorkspace(r.Context(), db.GetAgentTaskInWorkspaceParams{ID: sourceID, WorkspaceID: issue.WorkspaceID})
		if err != nil {
			return nil, "", err
		}
		if _, err = h.TaskService.RerunIssue(r.Context(), source.IssueID, source.ID, pgtype.UUID{}, memberID, false, canInvoke); err != nil {
			pending = append(pending, id)
			message = err.Error()
		}
	}
	return pending, message, nil
}
