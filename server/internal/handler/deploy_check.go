package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) CheckDeployTarget(w http.ResponseWriter, r *http.Request) {
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
	var input struct {
		ExpectedRevision int64 `json:"expected_revision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input) != nil || input.ExpectedRevision < 1 {
		writeError(w, 400, "expected_revision is required")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to start target check")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err := q.LockProjectForChatSessionCreate(r.Context(), db.LockProjectForChatSessionCreateParams{WorkspaceID: project.WorkspaceID, ID: project.ID}); err != nil {
		writeError(w, 409, "project is no longer available")
		return
	}
	target, err := q.GetDeployTarget(r.Context(), db.GetDeployTargetParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID, ID: id})
	if err != nil {
		writeError(w, 404, "target not found")
		return
	}
	config := deployTargetResponse(target)
	if len(config.MissingFields) > 0 {
		writeError(w, 400, "complete target configuration before checking")
		return
	}
	if target.Channel != "runtime" {
		writeError(w, 400, "this check requires a runtime deployment target")
		return
	}
	runtime, err := q.GetAgentRuntimeForWorkspace(r.Context(), db.GetAgentRuntimeForWorkspaceParams{WorkspaceID: project.WorkspaceID, ID: target.RuntimeID})
	if err != nil || !canUseRuntimeForAgent(member, runtime) {
		writeError(w, 403, "runtime is not available to this member")
		return
	}
	if target.CredentialID.Valid {
		credential, err := q.GetWorkbenchCredentialForBinding(r.Context(), db.GetWorkbenchCredentialForBindingParams{WorkspaceID: project.WorkspaceID, ID: target.CredentialID})
		if err != nil || (credential.ExpiresAt.Valid && !credential.ExpiresAt.Time.After(time.Now())) {
			writeError(w, 400, "deployment credential is unavailable or expired")
			return
		}
	}
	check := workbench.DeployCheck{ID: uuidToString(dbid.NewV7()), Status: "queued", At: time.Now().UTC(), RequestedBy: uuidToString(member.UserID), Detail: "waiting for the selected runtime"}
	raw, _ := json.Marshal(check)
	saved, err := q.StartDeployTargetCheck(r.Context(), db.StartDeployTargetCheckParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID, ID: id, ExpectedRevision: input.ExpectedRevision, CheckResult: raw})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "target changed; reload before checking")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to queue target check")
		return
	}
	if err := writeWorkbenchAudit(r.Context(), q, project.WorkspaceID, member.UserID, "deploy_target_check_requested", map[string]any{"target_id": uuidToString(id), "project_id": uuidToString(project.ID), "check": check}); err != nil {
		writeError(w, 500, "failed to audit target check")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit target check")
		return
	}
	h.requestDaemonPendingWork(uuidToString(target.RuntimeID), "deploy_check")
	writeJSON(w, 202, deployTargetResponse(saved))
}

func (h *Handler) claimRuntimeDeployCheck(ctx context.Context, runtimeID string) (*protocol.DaemonDeployCheck, error) {
	id, err := util.ParseUUID(runtimeID)
	if err != nil {
		return nil, err
	}
	target, err := h.Queries.ClaimRuntimeDeployTargetCheck(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	config := deployTargetResponse(target)
	check := workbench.ParseDeployCheck(target.LastCheck)
	return &protocol.DaemonDeployCheck{TargetID: uuidToString(target.ID), CheckID: check.ID, Kind: target.Kind, Directory: config.Location["dir"], StartCommand: config.Location["start_command"]}, nil
}

func (h *Handler) ReportDeployTargetCheck(w http.ResponseWriter, r *http.Request) {
	runtime, ok := h.requireDaemonRuntimeAccess(w, r, chi.URLParam(r, "runtimeId"))
	if !ok {
		return
	}
	targetID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "targetId"), "target id")
	if !ok {
		return
	}
	checkID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "checkId"), "check id")
	if !ok {
		return
	}
	var result protocol.DaemonDeployCheckResult
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&result) != nil || result.MS < 0 || result.MS > 60000 {
		writeError(w, 400, "invalid deployment check result")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to start check result")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	target, err := q.GetRuntimeDeployTargetCheck(r.Context(), db.GetRuntimeDeployTargetCheckParams{WorkspaceID: runtime.WorkspaceID, RuntimeID: runtime.ID, ID: targetID})
	if err != nil {
		writeError(w, 404, "runtime target not found")
		return
	}
	check := workbench.ParseDeployCheck(target.LastCheck)
	if check.ID != uuidToString(checkID) {
		writeError(w, 409, "check was superseded by a target change")
		return
	}
	if check.Status == "passed" || check.Status == "failed" {
		writeJSON(w, 200, deployTargetResponse(target))
		return
	}
	if check.Status != "running" || !check.At.After(time.Now().Add(-workbench.DeployCheckTimeout)) {
		writeError(w, 409, "check was not claimed or has expired; retry the check")
		return
	}
	check.At, check.MS = time.Now().UTC(), result.MS
	check.OK = result.DirectoryReady && result.ToolReady
	check.Status, check.Detail = "passed", "deployment directory and required runtime tools are ready"
	if !result.DirectoryReady {
		check.Status, check.Detail = "failed", "deployment directory is missing or not writable"
	}
	if !result.ToolReady {
		check.Status, check.Detail = "failed", "required deployment tool is unavailable"
	}
	raw, _ := json.Marshal(check)
	saved, err := q.FinishRuntimeDeployTargetCheck(r.Context(), db.FinishRuntimeDeployTargetCheckParams{WorkspaceID: runtime.WorkspaceID, ID: targetID, CheckID: check.ID, CheckResult: raw})
	if err != nil {
		writeError(w, 500, "failed to store check result")
		return
	}
	details, _ := json.Marshal(map[string]any{"target_id": uuidToString(targetID), "project_id": uuidToString(target.ProjectID), "runtime_id": uuidToString(runtime.ID), "check": check})
	if _, err := q.CreateActivity(r.Context(), db.CreateActivityParams{ID: dbid.NewV7(), WorkspaceID: runtime.WorkspaceID, ActorType: strToText("system"), ActorID: pgtype.UUID{}, Action: "deploy_target_checked", Details: details}); err != nil {
		writeError(w, 500, "failed to audit check result")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit check result")
		return
	}
	writeJSON(w, 200, deployTargetResponse(saved))
}
