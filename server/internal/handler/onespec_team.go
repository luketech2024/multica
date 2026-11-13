package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) ProvisionOneSpecTeam(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	member, ok := h.requireWorkspaceRole(w, r, uuidToString(project.WorkspaceID), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	actor, _ := h.resolveActor(r, uuidToString(member.UserID), uuidToString(project.WorkspaceID))
	if actor != "member" {
		writeError(w, 403, "only a member can initialize the team")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin team setup")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if err = q.LockOneSpecTeam(r.Context(), project.WorkspaceID); err != nil {
		writeError(w, 500, "failed to lock team")
		return
	}
	settings, err := q.GetOneSpecProject(r.Context(), db.GetOneSpecProjectParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if err != nil || !settings.Enabled {
		writeError(w, 409, "enable OneSpec in this project before preparing its team")
		return
	}
	setup, err := service.ResolveOneSpecTeam(r.Context(), q, project.WorkspaceID, member.UserID, &project, true)
	if err != nil {
		writeError(w, 500, "failed to prepare OneSpec team")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit team setup")
		return
	}
	writeJSON(w, 200, setup.Team)
}

func (h *Handler) ProvisionOneSpecWorkspace(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")
	member, ok := h.requireWorkspaceRole(w, r, id, "workspace not found", "owner", "admin")
	if !ok {
		return
	}
	actor, _ := h.resolveActor(r, uuidToString(member.UserID), id)
	if actor != "member" {
		writeError(w, 403, "only a member can initialize the team")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin team setup")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	setup, err := service.ResolveOneSpecTeam(r.Context(), q, member.WorkspaceID, member.UserID, nil, true)
	if err != nil {
		writeError(w, 500, "failed to prepare OneSpec team")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, "onespec_workspace_initialized", map[string]any{"status": setup.Status}); err != nil {
		writeError(w, 500, "failed to audit team setup")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit team setup")
		return
	}
	h.GetOneSpecWorkspace(w, r)
}
