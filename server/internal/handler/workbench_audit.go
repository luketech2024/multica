package handler

import (
	"context"
	"encoding/csv"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var workbenchAuditActions = []string{
	"credential_created", "credential_rotated", "credential_deleted", "credential_used",
	"deploy_target_saved", "deploy_target_deleted",
	"deploy_target_check_requested", "deploy_target_checked",
	"project_roles_created", "project_roles_updated", "project_template_created", "project_template_updated",
	"repository_profile_created", "repository_profile_updated", "business_template_published",
	"release_created",
}

func auditCSVCell(value string) string {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "@") {
		return "'" + value
	}
	return value
}

func writeWorkbenchAuditCSV(ctx context.Context, q *db.Queries, output io.Writer, workspaceID pgtype.UUID, selection auditSelection) error {
	if _, err := io.WriteString(output, "\ufeff"); err != nil {
		return err
	}
	writer := csv.NewWriter(output)
	writer.UseCRLF = true
	if err := writer.Write([]string{"exported_at", time.Now().UTC().Format(time.RFC3339Nano), "range_start", selection.Since.Format(time.RFC3339Nano), "range_end", selection.Until.Format(time.RFC3339Nano), "scope", selection.Scope}); err != nil {
		return err
	}
	if err := writer.Write([]string{"id", "created_at", "actor_type", "actor_id", "actor_name", "issue_id", "action", "details"}); err != nil {
		return err
	}
	params := selection.params(workspaceID)
	for {
		rows, err := q.ListWorkbenchAuditPage(ctx, params)
		if err != nil {
			return err
		}
		for _, row := range rows {
			record := []string{uuidToString(row.ID), timestampToString(row.CreatedAt), row.ActorType.String,
				uuidToString(row.ActorID), row.ActorName, uuidToString(row.IssueID), row.Action, string(row.Details)}
			for index := range record {
				record[index] = auditCSVCell(record[index])
			}
			if err := writer.Write(record); err != nil {
				return err
			}
			params.BeforeAt, params.BeforeID = row.CreatedAt, row.ID
		}
		if len(rows) < 1000 {
			break
		}
	}
	writer.Flush()
	return writer.Error()
}

func (h *Handler) ExportWorkbenchAudit(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	selection, err := parseAuditSelection(r, "configuration")
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
	if _, err := tx.Exec(r.Context(), "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		writeError(w, 500, "failed to start audit snapshot")
		return
	}
	file, err := os.CreateTemp("", "multica-audit-*.csv")
	if err != nil {
		writeError(w, 500, "failed to prepare audit export")
		return
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := writeWorkbenchAuditCSV(r.Context(), h.Queries.WithTx(tx), file, member.WorkspaceID, selection); err != nil {
		slog.Error("build workbench audit export", "workspace_id", uuidToString(member.WorkspaceID), "error", err)
		writeError(w, 500, "failed to build audit export")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to finish audit snapshot")
		return
	}
	info, err := file.Stat()
	if err != nil {
		writeError(w, 500, "failed to read audit export")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, 500, "failed to read audit export")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="workbench-audit-`+selection.Until.Format("2006-01-02")+`.csv"`)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Cache-Control", "no-store")
	if _, err := io.Copy(w, file); err != nil {
		slog.Warn("send workbench audit export", "workspace_id", uuidToString(member.WorkspaceID), "error", err)
	}
}
