package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) workbenchAdmin(w http.ResponseWriter, r *http.Request) (db.Member, bool) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin")
	if !ok {
		return member, false
	}
	actorType, _ := h.resolveActor(r, uuidToString(member.UserID), workspaceID)
	if actorType != "member" {
		writeError(w, 403, "only a member can manage workbench credentials")
		return member, false
	}
	return member, true
}

func (h *Handler) ListWorkbenchCredentials(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListWorkbenchCredentials(r.Context(), member.WorkspaceID)
	if err != nil {
		writeError(w, 500, "failed to list credentials")
		return
	}
	writeJSON(w, 200, map[string]any{"credentials": rows, "encryption_configured": h.WorkbenchSecretBox != nil})
}

func (h *Handler) SaveWorkbenchCredential(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	if h.WorkbenchSecretBox == nil {
		writeError(w, 503, "configure MULTICA_WORKBENCH_SECRET_KEY before saving credentials")
		return
	}
	var input workbench.CredentialInput
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 100000)).Decode(&input) != nil {
		writeError(w, 400, "invalid credential request")
		return
	}
	id := dbid.NewV7()
	rotating := chi.URLParam(r, "credentialId") != ""
	if rotating {
		id, ok = parseUUIDOrBadRequest(w, chi.URLParam(r, "credentialId"), "credential id")
		if !ok {
			return
		}
		if input.ExpectedRevision < 1 {
			writeError(w, 400, "expected_revision is required")
			return
		}
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin credential update")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err = q.LockWorkspaceForChatSessionCreate(r.Context(), member.WorkspaceID); err != nil {
		writeError(w, http.StatusConflict, "workspace is no longer available")
		return
	}
	if rotating {
		current, err := q.GetWorkbenchCredentialForUpdate(r.Context(), db.GetWorkbenchCredentialForUpdateParams{WorkspaceID: member.WorkspaceID, ID: id})
		if err != nil {
			writeError(w, 404, "credential not found")
			return
		}
		input.Label, input.Kind, input.Usage, input.ProjectID = current.Label, current.Kind, current.Usage, uuidToPtr(current.ProjectID)
	}
	if err = input.Validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	var projectID pgtype.UUID
	if input.ProjectID != nil {
		projectID, ok = parseUUIDOrBadRequest(w, *input.ProjectID, "project_id")
		if !ok {
			return
		}
		if !rotating {
			if _, err = q.LockProjectForChatSessionCreate(r.Context(), db.LockProjectForChatSessionCreateParams{ID: projectID, WorkspaceID: member.WorkspaceID}); err != nil {
				writeError(w, 400, "project must belong to this workspace")
				return
			}
		}
	}
	var expires pgtype.Timestamptz
	if input.ExpiresAt != nil && *input.ExpiresAt != "" {
		value, err := time.Parse(time.RFC3339, *input.ExpiresAt)
		if err != nil || !value.After(time.Now()) {
			writeError(w, 400, "expires_at must be a future RFC3339 timestamp")
			return
		}
		expires = pgtype.Timestamptz{Time: value, Valid: true}
	}
	ciphertext, wrappedKey, err := workbench.SealCredential(h.WorkbenchSecretBox, uuidToString(id), uuidToString(member.WorkspaceID), input.Secret)
	if err != nil {
		writeError(w, 500, "failed to encrypt credential")
		return
	}
	characters := []rune(strings.TrimSpace(input.Secret))
	lastFour := ""
	if input.Kind != "ssh-password" && len(characters) > 4 {
		lastFour = string(characters[len(characters)-4:])
	}
	input.Secret = ""
	var response db.ListWorkbenchCredentialsRow
	action := "credential_created"
	if rotating {
		row, writeErr := q.RotateWorkbenchCredential(r.Context(), db.RotateWorkbenchCredentialParams{WorkspaceID: member.WorkspaceID, ID: id, SecretEnc: ciphertext, WrappedKey: wrappedKey, LastFour: lastFour, ExpiresAt: expires, ExpectedRevision: input.ExpectedRevision})
		err = writeErr
		response = db.ListWorkbenchCredentialsRow(row)
		action = "credential_rotated"
	} else {
		row, writeErr := q.CreateWorkbenchCredential(r.Context(), db.CreateWorkbenchCredentialParams{ID: id, WorkspaceID: member.WorkspaceID, ProjectID: projectID, Label: input.Label, Kind: input.Kind, Usage: input.Usage, SecretEnc: ciphertext, WrappedKey: wrappedKey, LastFour: lastFour, ExpiresAt: expires})
		err = writeErr
		response = db.ListWorkbenchCredentialsRow(row)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "credential was updated or deleted")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to save credential")
		return
	}
	var settled []db.ArchiveCredentialExpiryNoticesRow
	if rotating {
		if _, err = tx.Exec(r.Context(), `UPDATE deploy_targets SET last_check=NULL, revision=revision+1, updated_at=now() WHERE workspace_id=$1 AND credential_id=$2`, member.WorkspaceID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to invalidate credential checks")
			return
		}
		if settled, err = q.ArchiveCredentialExpiryNotices(r.Context(), db.ArchiveCredentialExpiryNoticesParams{WorkspaceID: member.WorkspaceID, CredentialID: uuidToString(id)}); err != nil {
			writeError(w, 500, "failed to settle credential reminders")
			return
		}
	}
	if err = writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, action, map[string]any{"credential_id": uuidToString(id), "label": response.Label, "usage": response.Usage}); err != nil {
		writeError(w, 500, "failed to record credential audit")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit credential")
		return
	}
	h.publishCredentialReminderArchives(member, settled)
	writeJSON(w, 200, response)
}

func (h *Handler) DeleteWorkbenchCredential(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "credentialId"), "credential id")
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
		writeError(w, 500, "failed to begin credential deletion")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	// The row lock serializes unbinding/deletion with target credential binding.
	if _, err = tx.Exec(r.Context(), `SELECT id FROM credentials WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, member.WorkspaceID, id); err != nil {
		writeError(w, 500, "failed to lock credential")
		return
	}
	count, err := q.DeleteWorkbenchCredential(r.Context(), db.DeleteWorkbenchCredentialParams{WorkspaceID: member.WorkspaceID, ID: id, Revision: revision})
	if err != nil {
		writeError(w, 500, "failed to delete credential")
		return
	}
	if count == 0 {
		writeError(w, 409, "credential is referenced, updated or deleted; unbind it before deleting")
		return
	}
	settled, err := q.ArchiveCredentialExpiryNotices(r.Context(), db.ArchiveCredentialExpiryNoticesParams{WorkspaceID: member.WorkspaceID, CredentialID: uuidToString(id)})
	if err != nil {
		writeError(w, 500, "failed to settle credential reminders")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, "credential_deleted", map[string]string{"credential_id": uuidToString(id)}); err != nil {
		writeError(w, 500, "failed to record credential audit")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit credential deletion")
		return
	}
	h.publishCredentialReminderArchives(member, settled)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) publishCredentialReminderArchives(member db.Member, notices []db.ArchiveCredentialExpiryNoticesRow) {
	for _, notice := range notices {
		h.publish(protocol.EventInboxArchived, uuidToString(member.WorkspaceID), "member", uuidToString(member.UserID), map[string]any{
			"item_id": uuidToString(notice.ID), "issue_id": nil, "recipient_id": uuidToString(notice.RecipientID),
		})
	}
}

func (h *Handler) ListWorkbenchCredentialReferences(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "credentialId"), "credential id")
	if !ok {
		return
	}
	rows, err := h.Queries.ListCredentialTargets(r.Context(), db.ListCredentialTargetsParams{WorkspaceID: member.WorkspaceID, CredentialID: id})
	if err != nil {
		writeError(w, 500, "failed to list credential references")
		return
	}
	items := make([]DeployTargetResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, deployTargetResponse(row))
	}
	writeJSON(w, 200, map[string]any{"targets": items})
}
