package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// VerifyRequirementsDocument accepts verification only from the task's daemon.
func (h *Handler) VerifyRequirementsDocument(w http.ResponseWriter, r *http.Request) {
	task, ws, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	if task.Status != "running" {
		writeError(w, 409, "execution is no longer running")
		return
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin document verification")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	row, err := q.LockOneSpecFlow(r.Context(), db.LockOneSpecFlowParams{WorkspaceID: parseUUID(ws), IssueID: task.IssueID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 200, map[string]any{"artifact": nil})
		return
	}
	if err != nil {
		writeError(w, 500, "failed to load document")
		return
	}
	flow, err := parseOneSpecFlow(row)
	if err != nil {
		writeError(w, 500, "invalid workflow")
		return
	}
	artifact := flow.Latest("requirements")
	if artifact == nil || artifact.SourceTaskID != uuidToString(task.ID) || artifact.Document == nil {
		writeJSON(w, 200, map[string]any{"artifact": nil})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"artifact": artifact})
		return
	}
	var input struct {
		Version        int    `json:"version"`
		Error          string `json:"error"`
		PrototypeError string `json:"prototype_error"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || input.Version != artifact.Version || flow.Phase != "requirements" {
		writeError(w, 409, "document version changed")
		return
	}
	artifact.Document.Verified, artifact.Document.Error = input.Error == "", input.Error
	if artifact.Document.Prototype != nil {
		artifact.Document.Prototype.Verified, artifact.Document.Prototype.Error = input.PrototypeError == "", input.PrototypeError
	}
	raw, _ := json.Marshal(flow)
	_, err = q.UpdateOneSpecFlow(r.Context(), db.UpdateOneSpecFlowParams{State: raw, WorkspaceID: parseUUID(ws), IssueID: task.IssueID, ExpectedRevision: row.Revision})
	if err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, 409, "failed to save verification")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
