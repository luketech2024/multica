package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) CreateUserInput(w http.ResponseWriter, r *http.Request) {
	task, ws, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	h.createUserInput(w, r, task, ws, false)
}

func (h *Handler) CreateIssueUserInput(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	user, ok := requireUserID(w, r)
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, user, uuidToString(issue.WorkspaceID))
	task, ok := h.taskFromRequestHeader(r)
	if !ok || actorType != "agent" || uuidToString(task.AgentID) != actorID || task.IssueID != issue.ID {
		writeError(w, http.StatusForbidden, "questions require the active task agent")
		return
	}
	h.createUserInput(w, r, task, uuidToString(issue.WorkspaceID), true)
}

func (h *Handler) createUserInput(w http.ResponseWriter, r *http.Request, task db.AgentTaskQueue, ws string, platform bool) {
	var request protocol.UserInputRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&request) != nil {
		writeError(w, http.StatusBadRequest, "invalid user input request")
		return
	}
	if platform {
		request.Origin = "platform"
	} else {
		request.Origin = ""
	}
	if request.Validate() != nil || !task.IssueID.Valid {
		writeError(w, http.StatusBadRequest, "invalid user input request")
		return
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store question")
		return
	}
	defer tx.Rollback(r.Context())
	var status string
	if err = tx.QueryRow(r.Context(), "SELECT status FROM agent_task_queue WHERE id=$1 FOR UPDATE", task.ID).Scan(&status); err != nil || status != "running" {
		writeError(w, http.StatusConflict, "execution is no longer running")
		return
	}
	payload, _ := json.Marshal(request)
	row, err := h.Queries.WithTx(tx).CreateTaskUserInput(r.Context(), db.CreateTaskUserInputParams{TaskID: task.ID, WorkspaceID: parseUUID(ws), IssueID: task.IssueID, RequestKey: request.Key, Request: payload})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "question changed")
		return
	}
	if err == nil && platform && row.Status == "pending" {
		_, err = tx.Exec(r.Context(), `UPDATE task_user_input SET status='cancelled', updated_at=now()
			WHERE issue_id=$1 AND workspace_id=$2 AND status='pending' AND request->>'origin'='platform'
			AND id<>$3`, task.IssueID, parseUUID(ws), row.ID)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, http.StatusInternalServerError, "failed to store question")
		return
	}
	writeJSON(w, http.StatusOK, userInputToResponse(row))
}

func (h *Handler) GetUserInput(w http.ResponseWriter, r *http.Request) {
	task, ok := h.requireDaemonTaskAccess(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "requestId"), "request id")
	if !ok {
		return
	}
	row, err := h.Queries.GetTaskUserInput(r.Context(), db.GetTaskUserInputParams{ID: id, TaskID: task.ID})
	if err != nil {
		writeError(w, http.StatusNotFound, "question not found")
		return
	}
	var request protocol.UserInputRequest
	_ = json.Unmarshal(row.Request, &request)
	if task.Status != "running" && !(request.Origin == "platform" && task.Status == "completed") && (row.Status == "pending" || row.Status == "answered") {
		row.Status = "cancelled"
	}
	writeJSON(w, http.StatusOK, userInputToResponse(row))
}

func (h *Handler) ResolveUserInput(w http.ResponseWriter, r *http.Request) {
	task, ok := h.requireDaemonTaskAccess(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "requestId"), "request id")
	if !ok {
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || (input.Status != "resolved" && input.Status != "cancelled") {
		writeError(w, http.StatusBadRequest, "invalid resolution")
		return
	}
	if err := h.Queries.ResolveTaskUserInput(r.Context(), db.ResolveTaskUserInputParams{ID: id, TaskID: task.ID, Status: input.Status}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve question")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) ListUserInputs(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	rows, err := h.Queries.ListTaskUserInputs(r.Context(), db.ListTaskUserInputsParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load questions")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, userInputToResponse(row))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) AnswerUserInput(w http.ResponseWriter, r *http.Request) {
	r = h.withWakeupActor(r)
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	user, ok := requireUserID(w, r)
	if !ok {
		return
	}
	actor, _ := h.resolveActor(r, user, uuidToString(issue.WorkspaceID))
	if actor != "member" {
		writeError(w, http.StatusForbidden, "only a member can answer")
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "requestId"), "request id")
	if !ok {
		return
	}
	row, err := h.Queries.GetIssueUserInput(r.Context(), db.GetIssueUserInputParams{ID: id, WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
	if err != nil {
		writeError(w, http.StatusNotFound, "question not found")
		return
	}
	task, err := h.Queries.GetAgentTask(r.Context(), row.TaskID)
	if err != nil {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}
	agent, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{ID: task.AgentID, WorkspaceID: issue.WorkspaceID})
	if err != nil || !h.canInvokeAgent(r.Context(), agent, "member", user, user, uuidToString(issue.WorkspaceID)) {
		writeError(w, http.StatusForbidden, "cannot answer this agent")
		return
	}
	var response protocol.UserInputResponse
	var request protocol.UserInputRequest
	if json.Unmarshal(row.Request, &request) != nil || json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&response) != nil || request.ValidateResponse(response) != nil {
		writeError(w, http.StatusBadRequest, "invalid answers")
		return
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to answer question")
		return
	}
	defer tx.Rollback(r.Context())
	var status string
	if err = tx.QueryRow(r.Context(), "SELECT status FROM agent_task_queue WHERE id=$1 FOR UPDATE", task.ID).Scan(&status); err != nil || (status != "running" && !(request.Origin == "platform" && status == "completed")) {
		writeError(w, http.StatusConflict, "execution is no longer running")
		return
	}
	payload, _ := json.Marshal(response)
	q := h.Queries.WithTx(tx)
	updated, err := q.AnswerTaskUserInput(r.Context(), db.AnswerTaskUserInputParams{ID: row.ID, Response: payload, AnsweredBy: parseUUID(user)})
	firstAnswer := err == nil
	var created db.CreateCommentRow
	var suppressed []pgtype.UUID
	if errors.Is(err, pgx.ErrNoRows) {
		// A retry after a lost response must not answer the request twice.
		current, loadErr := q.GetTaskUserInput(r.Context(), db.GetTaskUserInputParams{ID: row.ID, TaskID: task.ID})
		var prior protocol.UserInputResponse
		_ = json.Unmarshal(current.Response, &prior)
		priorJSON, _ := json.Marshal(prior)
		if loadErr == nil && (current.Status == "answered" || current.Status == "resolved") && string(priorJSON) == string(payload) && current.AnsweredBy == parseUUID(user) {
			updated = current
			err = nil
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "question is no longer awaiting an answer")
		return
	}
	if err == nil && firstAnswer {
		var content strings.Builder
		for _, question := range request.Questions {
			content.WriteString(question.Question + "\n" + response.Answers[question.ID].Answers[0] + "\n\n")
		}
		if request.Origin != "platform" {
			suppressed = []pgtype.UUID{task.AgentID}
		}
		created, err = q.CreateComment(r.Context(), db.CreateCommentParams{
			IssueID: issue.ID, WorkspaceID: issue.WorkspaceID, AuthorType: "member", AuthorID: parseUUID(user),
			Content: strings.TrimSpace(content.String()), Type: "comment", SuppressedAgentIds: suppressed,
		})
		if err == nil {
			var saved map[string]any
			_ = json.Unmarshal(updated.Response, &saved)
			saved["discussion_comment_id"] = uuidToString(created.ID)
			updated.Response, _ = json.Marshal(saved)
			_, err = tx.Exec(r.Context(), "UPDATE task_user_input SET response=$2 WHERE id=$1", row.ID, updated.Response)
		}
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, http.StatusInternalServerError, "failed to answer question")
		return
	}
	if firstAnswer {
		comment := created.Comment()
		resp := commentToResponse(comment, nil, nil)
		resp.IssueRevision = created.IssueRevision
		h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), "member", user, map[string]any{"comment": resp, "issue_title": issue.Title, "issue_revision": created.IssueRevision})
		h.triggerTasksForComment(r.Context(), issue, comment, nil, "member", user, user, suppressed, commentSteerRequest{})
	}
	writeJSON(w, http.StatusOK, userInputToResponse(updated))
}

func userInputToResponse(row db.TaskUserInput) map[string]any {
	return map[string]any{"id": uuidToString(row.ID), "task_id": uuidToString(row.TaskID), "issue_id": uuidToString(row.IssueID), "status": row.Status, "request": json.RawMessage(row.Request), "answered_by": uuidToString(row.AnsweredBy), "response": json.RawMessage(row.Response), "created_at": timestampToPtr(row.CreatedAt)}
}
