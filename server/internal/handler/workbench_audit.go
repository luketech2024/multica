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
	"credential_created", "credential_rotated", "credential_deleted",
	"deploy_target_saved", "deploy_target_deleted",
	"project_roles_created", "project_roles_updated", "project_template_created", "project_template_updated",
	"repository_profile_created", "repository_profile_updated",
}

func auditCSVCell(value string) string {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "@") {
		return "'" + value
	}
	return value
}

func writeWorkbenchAuditCSV(ctx context.Context, q *db.Queries, output io.Writer, workspaceID pgtype.UUID, until time.Time) error {
	if _, err := io.WriteString(output, "\ufeff"); err != nil {
		return err
	}
	writer := csv.NewWriter(output)
	writer.UseCRLF = true
	since := until.Add(-90 * 24 * time.Hour)
	if err := writer.Write([]string{"exported_at", until.Format(time.RFC3339Nano), "range_start", since.Format(time.RFC3339Nano), "range_end", until.Format(time.RFC3339Nano), "scope", "configuration"}); err != nil {
		return err
	}
	if err := writer.Write([]string{"id", "created_at", "actor_type", "actor_id", "actor_name", "issue_id", "action", "details"}); err != nil {
		return err
	}
	params := db.ListWorkbenchAuditPageParams{
		WorkspaceID: workspaceID, Actions: workbenchAuditActions,
		SinceAt: pgtype.Timestamptz{Time: since, Valid: true},
		UntilAt: pgtype.Timestamptz{Time: until, Valid: true}, BeforeAt: pgtype.Timestamptz{Time: until, Valid: true},
		BeforeID: parseUUID("ffffffff-ffff-ffff-ffff-ffffffffffff"),
	}
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
	until := time.Now().UTC()
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
	if err := writeWorkbenchAuditCSV(r.Context(), h.Queries.WithTx(tx), file, member.WorkspaceID, until); err != nil {
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
	w.Header().Set("Content-Disposition", `attachment; filename="workbench-audit-`+until.Format("2006-01-02")+`.csv"`)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Cache-Control", "no-store")
	if _, err := io.Copy(w, file); err != nil {
		slog.Warn("send workbench audit export", "workspace_id", uuidToString(member.WorkspaceID), "error", err)
	}
}
