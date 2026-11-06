package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type DeployTargetResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id"`
	workbench.DeployTargetInput
	Revision      int64           `json:"revision"`
	MissingFields []string        `json:"missing_fields"`
	LastCheck     json.RawMessage `json:"last_check"`
}

func deployTargetResponse(row db.DeployTarget) DeployTargetResponse {
	input := workbench.DeployTargetInput{RepoID: uuidToString(row.RepoID), Label: row.Label, Env: row.Env, Kind: row.Kind, Channel: row.Channel, RuntimeID: uuidToPtr(row.RuntimeID), CredentialID: uuidToPtr(row.CredentialID), Location: map[string]string{}}
	_ = json.Unmarshal(row.Location, &input.Location)
	return DeployTargetResponse{ID: uuidToString(row.ID), WorkspaceID: uuidToString(row.WorkspaceID), ProjectID: uuidToString(row.ProjectID), DeployTargetInput: input, Revision: row.Revision, MissingFields: input.MissingFields(), LastCheck: row.LastCheck}
}

func writeWorkbenchAudit(ctx context.Context, q *db.Queries, workspaceID, actorID pgtype.UUID, action string, details any) error {
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = q.CreateActivity(ctx, db.CreateActivityParams{ID: dbid.NewV7(), WorkspaceID: workspaceID, ActorType: strToText("member"), ActorID: actorID, Action: action, Details: raw})
	return err
}

func (h *Handler) ListDeployTargets(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	rows, err := h.Queries.ListDeployTargets(r.Context(), db.ListDeployTargetsParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if err != nil {
		writeError(w, 500, "failed to list deployment targets")
		return
	}
	items := make([]DeployTargetResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, deployTargetResponse(row))
	}
	writeJSON(w, 200, map[string]any{"targets": items})
}

func (h *Handler) SaveDeployTarget(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	actorType, _ := h.resolveActor(r, uuidToString(member.UserID), uuidToString(project.WorkspaceID))
	if actorType != "member" {
		writeError(w, 403, "only a member can configure deployment targets")
		return
	}
	var input workbench.DeployTargetInput
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&input) != nil {
		writeError(w, 400, "invalid target configuration")
		return
	}
	if err := input.Validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	repoID, ok := parseUUIDOrBadRequest(w, input.RepoID, "repo_id")
	if !ok {
		return
	}
	var targetID, runtimeID, credentialID pgtype.UUID
	if id := chi.URLParam(r, "targetId"); id != "" {
		targetID, ok = parseUUIDOrBadRequest(w, id, "target id")
		if !ok {
			return
		}
		if input.ExpectedRevision < 1 {
			writeError(w, 400, "expected_revision is required")
			return
		}
	}
	if input.RuntimeID != nil {
		runtimeID, ok = parseUUIDOrBadRequest(w, *input.RuntimeID, "runtime_id")
		if !ok {
			return
		}
	}
	if input.CredentialID != nil {
		credentialID, ok = parseUUIDOrBadRequest(w, *input.CredentialID, "credential_id")
		if !ok {
			return
		}
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin target update")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err = q.LockProjectForChatSessionCreate(r.Context(), db.LockProjectForChatSessionCreateParams{ID: project.ID, WorkspaceID: project.WorkspaceID}); err != nil {
		writeError(w, 409, "project is no longer available")
		return
	}
	repo, err := q.GetProjectResourceInWorkspaceForUpdate(r.Context(), db.GetProjectResourceInWorkspaceForUpdateParams{ID: repoID, WorkspaceID: project.WorkspaceID})
	if err != nil || repo.ProjectID != project.ID {
		writeError(w, 400, "repository must belong to this project")
		return
	}
	if runtimeID.Valid {
		if _, err = q.GetAgentRuntimeForWorkspace(r.Context(), db.GetAgentRuntimeForWorkspaceParams{ID: runtimeID, WorkspaceID: project.WorkspaceID}); err != nil {
			writeError(w, 400, "runtime must belong to this workspace")
			return
		}
	}
	if credentialID.Valid {
		credential, credentialErr := q.GetWorkbenchCredentialForBinding(r.Context(), db.GetWorkbenchCredentialForBindingParams{WorkspaceID: project.WorkspaceID, ID: credentialID})
		if credentialErr != nil || (credential.ProjectID.Valid && credential.ProjectID != project.ID) || !slices.Contains(credential.Usage, "deploy") {
			writeError(w, 400, "credential must allow deployment in this project")
			return
		}
		if kind := input.CredentialKind(); kind != "" && credential.Kind != kind {
			writeError(w, http.StatusBadRequest, "credential type does not match the deployment channel")
			return
		}
	}
	location, err := json.Marshal(input.Location)
	if err != nil {
		writeError(w, 400, "invalid target location")
		return
	}
	var saved db.DeployTarget
	if targetID.Valid {
		saved, err = q.UpdateDeployTarget(r.Context(), db.UpdateDeployTargetParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID, ID: targetID, RepoID: repoID, Label: input.Label, Env: input.Env, Kind: input.Kind, Channel: input.Channel, RuntimeID: runtimeID, CredentialID: credentialID, Location: location, ExpectedRevision: input.ExpectedRevision})
	} else {
		saved, err = q.CreateDeployTarget(r.Context(), db.CreateDeployTargetParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID, RepoID: repoID, Label: input.Label, Env: input.Env, Kind: input.Kind, Channel: input.Channel, RuntimeID: runtimeID, CredentialID: credentialID, Location: location})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "target was updated or deleted; reload before saving")
		return
	}
	var constraint *pgconn.PgError
	if errors.As(err, &constraint) && constraint.Code == "23505" {
		writeError(w, 409, "a target with this label already exists for this repository and environment")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to save target")
		return
	}
	response := deployTargetResponse(saved)
	if err = writeWorkbenchAudit(r.Context(), q, project.WorkspaceID, member.UserID, "deploy_target_saved", response); err != nil {
		writeError(w, 500, "failed to record target audit")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit target")
		return
	}
	writeJSON(w, 200, response)
}

func (h *Handler) DeleteDeployTarget(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "targetId"), "target id")
	if !ok {
		return
	}
	revision, err := strconv.ParseInt(r.URL.Query().Get("expected_revision"), 10, 64)
	if err != nil || revision < 1 {
		writeError(w, 400, "expected_revision is required")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin target deletion")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	count, err := q.DeleteDeployTarget(r.Context(), db.DeleteDeployTargetParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID, ID: id, Revision: revision})
	if err != nil {
		writeError(w, 500, "failed to delete target")
		return
	}
	if count == 0 {
		writeError(w, 409, "target was updated or deleted")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, project.WorkspaceID, member.UserID, "deploy_target_deleted", map[string]any{"target_id": uuidToString(id), "project_id": uuidToString(project.ID)}); err != nil {
		writeError(w, 500, "failed to record deletion audit")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit target deletion")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
