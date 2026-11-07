package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	inboxwriter "github.com/multica-ai/multica/server/internal/inbox"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const maxAuditExportBytes = 64 << 20

var errAuditExportTooLarge = errors.New("audit export exceeds 64 MiB; select a shorter date range")

type auditExportBuffer struct{ bytes.Buffer }

func (buffer *auditExportBuffer) Write(data []byte) (int, error) {
	if len(data) > maxAuditExportBytes-buffer.Len() {
		return 0, errAuditExportTooLarge
	}
	return buffer.Buffer.Write(data)
}

func (h *Handler) CreateWorkbenchAuditExport(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	selection, err := parseAuditSelection(r, "all")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to start audit export")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
		writeError(w, 500, "failed to start audit snapshot")
		return
	}
	q := h.Queries.WithTx(tx)
	if _, err := q.LockWorkspaceForChatSessionCreate(r.Context(), member.WorkspaceID); err != nil {
		writeError(w, 404, "workspace not found")
		return
	}
	var content auditExportBuffer
	if err := writeWorkbenchAuditCSV(r.Context(), q, &content, member.WorkspaceID, selection); err != nil {
		if errors.Is(err, errAuditExportTooLarge) {
			writeError(w, 413, err.Error())
			return
		}
		writeError(w, 500, "failed to build audit export")
		return
	}
	expires := time.Now().UTC().Add(24 * time.Hour)
	export, err := q.CreateWorkbenchAuditExport(r.Context(), db.CreateWorkbenchAuditExportParams{
		ID: dbid.NewV7(), WorkspaceID: member.WorkspaceID, RequesterID: member.UserID,
		Filename: "workbench-audit-" + selection.Until.Format("2006-01-02") + ".csv", Content: content.Bytes(), Scope: selection.Scope,
		RangeStart: pgtype.Timestamptz{Time: selection.Since, Valid: true}, RangeEnd: pgtype.Timestamptz{Time: selection.Until, Valid: true}, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true},
	})
	if err != nil {
		writeError(w, 500, "failed to save audit export")
		return
	}
	details, _ := json.Marshal(map[string]string{"source": "workbench_audit_export", "workbench": "true", "export_id": uuidToString(export.ID), "expires_at": expires.Format(time.RFC3339Nano), "scope": selection.Scope})
	notice, inserted, err := inboxwriter.CreateIfEligible(r.Context(), q, db.CreateInboxItemParams{
		ID: dbid.NewV7(), WorkspaceID: member.WorkspaceID, RecipientType: "member", RecipientID: member.UserID,
		Type: "review_requested", Severity: "info", Title: export.Filename, ActorType: strToText("system"), Details: details,
	})
	if err != nil || !inserted {
		writeError(w, 500, "failed to notify audit export")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to finish audit export")
		return
	}
	h.publish(protocol.EventInboxNew, uuidToString(member.WorkspaceID), "system", "", map[string]any{"item": inboxToResponse(notice)})
	writeJSON(w, 201, map[string]any{"id": uuidToString(export.ID), "filename": export.Filename, "expires_at": expires, "range_start": selection.Since, "range_end": selection.Until, "scope": selection.Scope, "size": content.Len()})
}

func (h *Handler) DownloadWorkbenchAuditExport(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "exportId"), "export id")
	if !ok {
		return
	}
	export, err := h.Queries.GetWorkbenchAuditExport(r.Context(), db.GetWorkbenchAuditExportParams{WorkspaceID: member.WorkspaceID, RequesterID: member.UserID, ID: id})
	if err != nil {
		writeError(w, 404, "audit export unavailable or expired")
		return
	}
	if !export.ExpiresAt.Time.After(time.Now()) {
		writeError(w, 410, "audit export expired; create a new export")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+export.Filename+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(export.Content)))
	w.Header().Set("Cache-Control", "no-store")
	w.Write(export.Content)
}
