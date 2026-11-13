package handler

import (
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

type oneSpecConfiguration struct {
	Enabled              bool                      `json:"enabled"`
	DefaultRuntimeID     *string                   `json:"default_runtime_id"`
	RuntimeID            *string                   `json:"runtime_id"`
	EffectiveRuntimeID   *string                   `json:"effective_runtime_id"`
	OverviewRepositoryID *string                   `json:"overview_repository_id"`
	Revision             int64                     `json:"revision"`
	ExpectedRevision     int64                     `json:"expected_revision,omitempty"`
	Setup                *service.OneSpecTeamSetup `json:"setup,omitempty"`
	RuntimeSource        string                    `json:"runtime_source"`
}

func oneSpecID(id pgtype.UUID) *string {
	if !id.Valid {
		return nil
	}
	value := uuidToString(id)
	return &value
}

func (h *Handler) GetOneSpecWorkspace(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")
	if _, ok := h.requireWorkspaceRole(w, r, id, "workspace not found", "owner", "admin", "member"); !ok {
		return
	}
	ws, ok := parseUUIDOrBadRequest(w, id, "workspace id")
	if !ok {
		return
	}
	row, err := h.Queries.GetOneSpecWorkspace(r.Context(), ws)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 500, "failed to read OneSpec settings")
		return
	}
	setup, err := service.ResolveOneSpecTeam(r.Context(), h.Queries, ws, parseUUID(r.Header.Get("X-User-ID")), nil, false)
	if err != nil {
		writeError(w, 500, "failed to read OneSpec team")
		return
	}
	writeJSON(w, 200, oneSpecConfiguration{DefaultRuntimeID: oneSpecID(row.DefaultRuntimeID), EffectiveRuntimeID: oneSpecID(row.DefaultRuntimeID), Revision: row.Revision, Setup: &setup, RuntimeSource: "workspace"})
}

func (h *Handler) SaveOneSpecWorkspace(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")
	member, ok := h.requireWorkspaceRole(w, r, id, "workspace not found", "owner", "admin")
	if !ok {
		return
	}
	actor, _ := h.resolveActor(r, uuidToString(member.UserID), id)
	if actor != "member" {
		writeError(w, 403, "only members can configure OneSpec")
		return
	}
	var input oneSpecConfiguration
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input) != nil || input.ExpectedRevision < 0 {
		writeError(w, 400, "invalid OneSpec settings")
		return
	}
	ws := member.WorkspaceID
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin settings update")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if err = q.LockOneSpecTeam(r.Context(), ws); err != nil {
		writeError(w, 500, "failed to lock settings")
		return
	}
	current, err := q.GetOneSpecWorkspace(r.Context(), ws)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 500, "failed to read settings")
		return
	}
	if current.Revision != input.ExpectedRevision {
		writeError(w, 409, "settings changed; reload before saving")
		return
	}
	var runtimeID pgtype.UUID
	if input.DefaultRuntimeID != nil {
		runtimeID, ok = parseUUIDOrBadRequest(w, *input.DefaultRuntimeID, "default_runtime_id")
		if !ok {
			return
		}
		runtime, e := q.LockAgentRuntimeForAccessChange(r.Context(), runtimeID)
		if e != nil || runtime.WorkspaceID != ws || !canUseRuntimeForAgent(member, runtime) {
			writeError(w, 403, "runtime is unavailable or inaccessible")
			return
		}
	}
	saved, err := q.SaveOneSpecWorkspace(r.Context(), db.SaveOneSpecWorkspaceParams{WorkspaceID: ws, DefaultRuntimeID: runtimeID})
	if err != nil {
		writeError(w, 500, "failed to save settings")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, ws, member.UserID, "onespec_workspace_updated", map[string]any{"revision": saved.Revision}); err != nil {
		writeError(w, 500, "failed to audit settings")
		return
	}
	if _, err = service.ResolveOneSpecTeam(r.Context(), q, ws, member.UserID, nil, true); err != nil {
		writeError(w, 500, "failed to prepare OneSpec team")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit settings")
		return
	}
	h.GetOneSpecWorkspace(w, r)
}

func (h *Handler) GetOneSpecProject(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	row, err := h.Queries.GetOneSpecProject(r.Context(), db.GetOneSpecProjectParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 500, "failed to read OneSpec project")
		return
	}
	defaults, err := h.Queries.GetOneSpecWorkspace(r.Context(), project.WorkspaceID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 500, "failed to read OneSpec defaults")
		return
	}
	effective := row.RuntimeID
	if !effective.Valid {
		effective = defaults.DefaultRuntimeID
	}
	setup, err := service.ResolveOneSpecTeam(r.Context(), h.Queries, project.WorkspaceID, parseUUID(r.Header.Get("X-User-ID")), &project, false)
	if err != nil {
		writeError(w, 500, "failed to read OneSpec team")
		return
	}
	source := "workspace"
	if row.RuntimeID.Valid {
		source = "project"
	}
	writeJSON(w, 200, oneSpecConfiguration{Enabled: row.Enabled, RuntimeID: oneSpecID(row.RuntimeID), EffectiveRuntimeID: oneSpecID(effective), OverviewRepositoryID: oneSpecID(row.OverviewRepositoryID), Revision: row.Revision, Setup: &setup, RuntimeSource: source})
}

func (h *Handler) SaveOneSpecProject(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, 403, "only members can configure OneSpec")
		return
	}
	var input oneSpecConfiguration
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input) != nil || input.ExpectedRevision < 0 {
		writeError(w, 400, "invalid OneSpec project settings")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin project settings")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if err = q.LockOneSpecTeam(r.Context(), project.WorkspaceID); err != nil {
		writeError(w, 500, "failed to lock team")
		return
	}
	if _, err = q.LockProjectWorkbenchRoles(r.Context(), db.LockProjectWorkbenchRolesParams{WorkspaceID: project.WorkspaceID, ID: project.ID}); err != nil {
		writeError(w, 404, "project not found")
		return
	}
	current, err := q.GetOneSpecProject(r.Context(), db.GetOneSpecProjectParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 500, "failed to read project settings")
		return
	}
	if current.Revision != input.ExpectedRevision {
		writeError(w, 409, "project settings changed; reload before saving")
		return
	}
	var runtimeID, repoID pgtype.UUID
	if input.RuntimeID != nil {
		runtimeID, ok = parseUUIDOrBadRequest(w, *input.RuntimeID, "runtime_id")
		if !ok {
			return
		}
		runtime, e := q.LockAgentRuntimeForAccessChange(r.Context(), runtimeID)
		if e != nil || runtime.WorkspaceID != project.WorkspaceID || !canUseRuntimeForAgent(member, runtime) {
			writeError(w, 403, "runtime is unavailable or inaccessible")
			return
		}
	}
	if input.OverviewRepositoryID != nil {
		repoID, ok = parseUUIDOrBadRequest(w, *input.OverviewRepositoryID, "overview_repository_id")
		if !ok {
			return
		}
		repo, e := q.GetProjectResourceInWorkspaceForUpdate(r.Context(), db.GetProjectResourceInWorkspaceForUpdateParams{ID: repoID, WorkspaceID: project.WorkspaceID})
		if e != nil || repo.ProjectID != project.ID || (repo.ResourceType != "github_repo" && repo.ResourceType != "git_repo" && repo.ResourceType != "local_directory") {
			writeError(w, 400, "select a repository attached to this project")
			return
		}
	}
	saved, err := q.SaveOneSpecProject(r.Context(), db.SaveOneSpecProjectParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID, Enabled: input.Enabled, RuntimeID: runtimeID, OverviewRepositoryID: repoID})
	if err != nil {
		writeError(w, 500, "failed to save project settings")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, project.WorkspaceID, member.UserID, "onespec_project_updated", map[string]any{"project_id": uuidToString(project.ID), "revision": saved.Revision}); err != nil {
		writeError(w, 500, "failed to audit project settings")
		return
	}
	if saved.Enabled {
		if _, err = service.ResolveOneSpecTeam(r.Context(), q, project.WorkspaceID, member.UserID, &project, true); err != nil {
			writeError(w, 500, "failed to prepare OneSpec team")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit project settings")
		return
	}
	h.GetOneSpecProject(w, r)
}

func parseOneSpecFlow(row db.OnespecFlow) (workbench.OneSpecFlow, error) {
	var flow workbench.OneSpecFlow
	err := json.Unmarshal(row.State, &flow)
	return flow, err
}
