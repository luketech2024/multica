package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/workbench"
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
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "onespec-team:"+uuidToString(project.WorkspaceID)); err != nil {
		writeError(w, 500, "failed to lock team")
		return
	}
	if _, err = q.LockProjectWorkbenchRoles(r.Context(), db.LockProjectWorkbenchRolesParams{WorkspaceID: project.WorkspaceID, ID: project.ID}); err != nil {
		writeError(w, 404, "project not found")
		return
	}
	config, err := q.GetOneSpecProject(r.Context(), db.GetOneSpecProjectParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 500, "failed to load project configuration")
		return
	}
	runtimeID := config.RuntimeID
	if !runtimeID.Valid {
		defaults, e := q.GetOneSpecWorkspace(r.Context(), project.WorkspaceID)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			writeError(w, 500, "failed to load defaults")
			return
		}
		runtimeID = defaults.DefaultRuntimeID
	}
	if !runtimeID.Valid {
		writeError(w, 409, "configure a default or project runtime first")
		return
	}
	runtime, err := q.LockAgentRuntimeForAccessChange(r.Context(), runtimeID)
	if err != nil || runtime.WorkspaceID != project.WorkspaceID || !canUseRuntimeForAgent(member, runtime) {
		writeError(w, 403, "runtime is inaccessible")
		return
	}
	before, resource, err := workbench.LoadProjectRoles(r.Context(), q, project)
	if err != nil {
		writeError(w, 500, "failed to read roles")
		return
	}
	originalConfig, _ := json.Marshal(before.Config)
	for _, role := range service.OneSpecRoles {
		key := pgtype.Text{String: "onespec:" + role, Valid: true}
		agent, e := q.GetOneSpecAgent(r.Context(), db.GetOneSpecAgentParams{WorkspaceID: project.WorkspaceID, RuntimeID: runtimeID, SystemKey: key})
		if errors.Is(e, pgx.ErrNoRows) {
			agent, e = q.CreateSystemUserAgent(r.Context(), db.CreateSystemUserAgentParams{WorkspaceID: project.WorkspaceID, Name: service.OneSpecRoleNames[role] + " · " + uuidToString(runtimeID)[:8], Description: service.OneSpecRoleNames[role], RuntimeMode: runtime.RuntimeMode, RuntimeID: runtimeID, Visibility: "workspace", PermissionMode: "public_to", MaxConcurrentTasks: 1, OwnerID: member.UserID, SystemKey: key})
			if e == nil {
				e = replaceInvocationTargetsWithQueries(r.Context(), q, agent.ID, member.UserID, []targetSpec{{targetType: invocationTargetWorkspace, targetID: project.WorkspaceID}})
			}
		}
		if e != nil {
			writeError(w, 409, "could not initialize a platform agent")
			return
		}
		bound, e := q.ListAgentSkills(r.Context(), agent.ID)
		if e != nil {
			writeError(w, 500, "failed to inspect agent skills")
			return
		}
		names := make([]service.AgentSkillData, 0, len(bound))
		for _, skill := range bound {
			names = append(names, service.AgentSkillData{Name: skill.Name})
		}
		if e = service.ValidateOneSpecSkillNames(names); e != nil {
			writeError(w, 409, e.Error())
			return
		}
		before.Config.Roles[role] = workbench.ProjectRole{Enabled: true, AgentID: oneSpecID(agent.ID), RuntimeID: oneSpecID(runtimeID)}
	}
	currentConfig, _ := json.Marshal(before.Config)
	if bytes.Equal(originalConfig, currentConfig) {
		if err = tx.Commit(r.Context()); err != nil {
			writeError(w, 500, "failed to commit team")
			return
		}
		writeJSON(w, 200, before)
		return
	}
	before.Revision++
	raw, _ := json.Marshal(before)
	if resource.ID.Valid {
		_, err = q.UpdateProjectResource(r.Context(), db.UpdateProjectResourceParams{ID: resource.ID, ResourceRef: raw, Label: resource.Label, Position: resource.Position})
	} else {
		_, err = q.CreateProjectResource(r.Context(), db.CreateProjectResourceParams{ProjectID: project.ID, WorkspaceID: project.WorkspaceID, ResourceType: "workbench_roles", ResourceRef: raw, Label: strToText("OneSpec team"), CreatedBy: member.UserID})
	}
	if err != nil {
		writeError(w, 500, "failed to save team")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, project.WorkspaceID, member.UserID, "onespec_team_initialized", map[string]any{"project_id": uuidToString(project.ID), "runtime_id": uuidToString(runtimeID)}); err != nil {
		writeError(w, 500, "failed to audit team")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit team")
		return
	}
	writeJSON(w, 200, before)
}
