package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type ProjectRolesResponse = workbench.ProjectRoleConfiguration

func (h *Handler) GetProjectWorkbenchRoles(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	response, _, err := workbench.LoadProjectRoles(r.Context(), h.Queries, project)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) SaveProjectWorkbenchRoles(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	member, ok := h.requireWorkspaceRole(w, r, uuidToString(project.WorkspaceID), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	actorType, _ := h.resolveActor(r, uuidToString(member.UserID), uuidToString(project.WorkspaceID))
	if actorType != "member" {
		writeError(w, http.StatusForbidden, "only a member can change project roles")
		return
	}
	var input struct {
		Config           json.RawMessage `json:"config"`
		ExpectedRevision int64           `json:"expected_revision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input) != nil || input.ExpectedRevision < 1 {
		writeError(w, http.StatusBadRequest, "configuration and expected_revision are required")
		return
	}
	config, err := workbench.ParseProjectRoles(input.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, role := range workbench.ProjectRoleOrder {
		entry := config.Roles[role]
		if !entry.Enabled {
			continue
		}
		if entry.AgentID != nil {
			id, valid := parseUUIDOrBadRequest(w, *entry.AgentID, "agent_id")
			if !valid {
				return
			}
			agent, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{ID: id, WorkspaceID: project.WorkspaceID})
			if err != nil || agent.ArchivedAt.Valid {
				writeError(w, http.StatusBadRequest, "agent must be active in this workspace")
				return
			}
		}
		if entry.RuntimeID != nil {
			id, valid := parseUUIDOrBadRequest(w, *entry.RuntimeID, "runtime_id")
			if !valid {
				return
			}
			if _, err := h.Queries.GetAgentRuntimeForWorkspace(r.Context(), db.GetAgentRuntimeForWorkspaceParams{ID: id, WorkspaceID: project.WorkspaceID}); err != nil {
				writeError(w, http.StatusBadRequest, "runtime must belong to this workspace")
				return
			}
		}
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin role update")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err = q.LockProjectWorkbenchRoles(r.Context(), db.LockProjectWorkbenchRolesParams{WorkspaceID: project.WorkspaceID, ID: project.ID}); err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	before, resource, err := workbench.LoadProjectRoles(r.Context(), q, project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load project roles")
		return
	}
	if before.Revision != input.ExpectedRevision {
		writeError(w, http.StatusConflict, "project roles changed; reload before saving")
		return
	}
	response := ProjectRolesResponse{Config: config, Revision: before.Revision + 1}
	raw, _ := json.Marshal(response)
	if resource.ID.Valid {
		_, err = q.UpdateProjectResource(r.Context(), db.UpdateProjectResourceParams{ID: resource.ID, ResourceRef: raw, Label: resource.Label, Position: resource.Position})
	} else {
		_, err = q.CreateProjectResource(r.Context(), db.CreateProjectResourceParams{ProjectID: project.ID, WorkspaceID: project.WorkspaceID, ResourceType: "workbench_roles", ResourceRef: raw, Label: pgtype.Text{String: "Project team", Valid: true}, CreatedBy: member.UserID})
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save project roles")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, project.WorkspaceID, member.UserID, "project_roles_updated", map[string]any{"project_id": uuidToString(project.ID), "before": before.Config, "after": config, "revision": response.Revision}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record role audit")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit role update")
		return
	}
	updated, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: project.ID, WorkspaceID: project.WorkspaceID})
	if err == nil {
		response := projectToResponse(updated)
		response.IssueCount, response.DoneCount = h.loadProjectIssueStats(r.Context(), project.WorkspaceID, project.ID)
		response.ResourceCount = h.loadProjectResourceCount(r.Context(), project.ID)
		h.publish(protocol.EventProjectUpdated, uuidToString(project.WorkspaceID), "member", uuidToString(member.UserID), map[string]any{"project": response})
	}
	writeJSON(w, http.StatusOK, response)
}
