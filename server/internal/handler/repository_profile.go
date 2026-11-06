package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) ListRepositoryStacks(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member", "viewer"); !ok {
		return
	}
	catalog, err := workbench.RepositoryStacks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load repository stack catalog")
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

func (h *Handler) GetRepositoryProfile(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "resourceId"), "resourceId")
	if !ok {
		return
	}
	resource, err := h.Queries.GetProjectResourceInWorkspace(r.Context(), db.GetProjectResourceInWorkspaceParams{ID: id, WorkspaceID: project.WorkspaceID})
	if err != nil || resource.ProjectID != project.ID || (resource.ResourceType != "github_repo" && resource.ResourceType != "local_directory") {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	profile, err := workbench.ReadRepositoryProfile(resource.ResourceRef)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *Handler) SaveRepositoryProfile(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusForbidden, "only a member can change a repository profile")
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "resourceId"), "resourceId")
	if !ok {
		return
	}
	var input workbench.RepositoryProfileInput
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&input) != nil || input.ExpectedRevision < 0 {
		writeError(w, http.StatusBadRequest, "invalid repository profile request")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin repository update")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	resource, err := q.GetProjectResourceInWorkspaceForUpdate(r.Context(), db.GetProjectResourceInWorkspaceForUpdateParams{ID: id, WorkspaceID: project.WorkspaceID})
	if err != nil || resource.ProjectID != project.ID || (resource.ResourceType != "github_repo" && resource.ResourceType != "local_directory") {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	before, err := workbench.ReadRepositoryProfile(resource.ResourceRef)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if before.Revision != input.ExpectedRevision {
		writeError(w, http.StatusConflict, "repository profile changed; reload before saving")
		return
	}
	after, err := workbench.BuildRepositoryProfile(input, before)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ref, err := workbench.WriteRepositoryProfile(resource.ResourceRef, after)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode repository profile")
		return
	}
	updated, err := q.UpdateProjectResource(r.Context(), db.UpdateProjectResourceParams{ID: resource.ID, ResourceRef: ref, Label: resource.Label, Position: resource.Position})
	if err != nil {
		writeError(w, http.StatusConflict, "failed to save repository profile")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, project.WorkspaceID, member.UserID, "repository_profile_updated", map[string]any{"project_id": uuidToString(project.ID), "repo_id": uuidToString(resource.ID), "before": before, "after": after}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record repository profile audit")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit repository profile")
		return
	}
	h.publish(protocol.EventProjectResourceUpdated, uuidToString(project.WorkspaceID), "member", uuidToString(member.UserID), map[string]any{"resource": projectResourceToResponse(updated), "project_id": uuidToString(project.ID)})
	writeJSON(w, http.StatusOK, after)
}
