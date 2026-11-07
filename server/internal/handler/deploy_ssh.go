package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func (h *Handler) checkSSHDeployTarget(w http.ResponseWriter, r *http.Request, tx pgx.Tx, q *db.Queries, target db.DeployTarget, config workbench.DeployTargetInput, member db.Member, revision int64) {
	credential, err := q.GetWorkbenchCredentialForExecution(r.Context(), db.GetWorkbenchCredentialForExecutionParams{WorkspaceID: target.WorkspaceID, ID: target.CredentialID})
	if err != nil || credential.Kind != "ssh-key" || !slices.Contains(credential.Usage, "deploy") ||
		(credential.ProjectID.Valid && credential.ProjectID != target.ProjectID) || (credential.ExpiresAt.Valid && !credential.ExpiresAt.Time.After(time.Now())) {
		writeError(w, 400, "deployment credential is unavailable or expired")
		return
	}
	secret, err := workbench.OpenCredential(h.WorkbenchSecretBox, uuidToString(credential.ID), uuidToString(target.WorkspaceID), credential.SecretEnc, credential.WrappedKey)
	if err != nil {
		writeError(w, 400, "deployment credential could not be opened")
		return
	}
	defer clear(secret)
	check := workbench.DeployCheck{ID: uuidToString(dbid.NewV7()), Status: "running", At: time.Now().UTC(), RequestedBy: uuidToString(member.UserID), Detail: "checking the configured SSH host"}
	raw, _ := json.Marshal(check)
	_, err = q.StartDeployTargetCheck(r.Context(), db.StartDeployTargetCheckParams{WorkspaceID: target.WorkspaceID, ProjectID: target.ProjectID, ID: target.ID, ExpectedRevision: revision, CheckResult: raw})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "target changed; reload before checking")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to start SSH check")
		return
	}
	details := map[string]any{"target_id": uuidToString(target.ID), "project_id": uuidToString(target.ProjectID), "check_id": check.ID, "credential_id": uuidToString(credential.ID), "usage": "deploy", "operation": "target_check", "outcome": "started"}
	if err := writeWorkbenchAudit(r.Context(), q, target.WorkspaceID, member.UserID, "deploy_target_check_requested", details); err != nil {
		writeError(w, 500, "failed to audit SSH check")
		return
	}
	if err := writeWorkbenchAudit(r.Context(), q, target.WorkspaceID, member.UserID, "credential_used", details); err != nil {
		writeError(w, 500, "failed to audit credential use")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit SSH check")
		return
	}

	probeContext, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	result := workbench.ProbeSSH(probeContext, config, secret)
	cancel()
	result.ID, result.RequestedBy = check.ID, check.RequestedBy
	raw, _ = json.Marshal(result)
	// Persist the bounded probe's outcome even when the requesting browser disconnects.
	resultContext, cancelResult := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancelResult()
	resultTx, err := h.TxStarter.Begin(resultContext)
	if err != nil {
		writeError(w, 500, "failed to store SSH result")
		return
	}
	defer resultTx.Rollback(resultContext)
	resultQueries := h.Queries.WithTx(resultTx)
	saved, err := resultQueries.FinishRuntimeDeployTargetCheck(resultContext, db.FinishRuntimeDeployTargetCheckParams{WorkspaceID: target.WorkspaceID, ID: target.ID, CheckID: check.ID, CheckResult: raw})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "check was superseded by a configuration change")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to store SSH result")
		return
	}
	details["outcome"], details["check"] = result.Status, result
	if err := writeWorkbenchAudit(resultContext, resultQueries, target.WorkspaceID, member.UserID, "deploy_target_checked", details); err != nil {
		writeError(w, 500, "failed to audit SSH result")
		return
	}
	if err := resultTx.Commit(resultContext); err != nil {
		writeError(w, 500, "failed to commit SSH result")
		return
	}
	writeJSON(w, 200, deployTargetResponse(saved))
}
