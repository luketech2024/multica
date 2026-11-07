package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var securityAuditActions = []string{
	"security_scan_recorded", "security_accept", "security_request_recheck", "security_recheck", "security_acceptance_expired",
	"workspace_member_role_changed", "credential_used", "certificate_renewal_failed",
}

type auditSelection struct {
	Scope string
	Since time.Time
	Until time.Time
}

type auditCursor struct {
	Until  time.Time `json:"until"`
	Before time.Time `json:"before"`
	ID     string    `json:"id"`
}

type auditRecordResponse struct {
	db.ListWorkbenchAuditPageRow
	Details json.RawMessage `json:"details"`
}

func parseAuditSelection(r *http.Request, defaultScope string) (auditSelection, error) {
	now := time.Now().UTC()
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = defaultScope
	}
	if !slices.Contains([]string{"configuration", "security", "all"}, scope) {
		return auditSelection{}, errors.New("invalid audit scope")
	}
	days := 90
	if raw := r.URL.Query().Get("days"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || !slices.Contains([]int{7, 30, 90}, value) {
			return auditSelection{}, errors.New("audit range must be 7, 30 or 90 days")
		}
		days = value
	}
	until := now
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var cursor auditCursor
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil || json.Unmarshal(decoded, &cursor) != nil || cursor.Until.After(now) || cursor.Until.Before(now.Add(-24*time.Hour)) {
			return auditSelection{}, errors.New("audit cursor expired or invalid")
		}
		until = cursor.Until
	}
	return auditSelection{Scope: scope, Since: until.Add(-time.Duration(days) * 24 * time.Hour), Until: until}, nil
}

func (selection auditSelection) params(workspace pgtype.UUID) db.ListWorkbenchAuditPageParams {
	actions := workbenchAuditActions
	if selection.Scope == "security" {
		actions = securityAuditActions
	}
	if selection.Scope == "all" {
		actions = append(slices.Clone(workbenchAuditActions), securityAuditActions...)
	}
	return db.ListWorkbenchAuditPageParams{WorkspaceID: workspace, Actions: actions,
		SinceAt: pgtype.Timestamptz{Time: selection.Since, Valid: true}, UntilAt: pgtype.Timestamptz{Time: selection.Until, Valid: true},
		BeforeAt: pgtype.Timestamptz{Time: selection.Until, Valid: true}, BeforeID: parseUUID("ffffffff-ffff-ffff-ffff-ffffffffffff")}
}

func (h *Handler) ListWorkbenchAudit(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	selection, err := parseAuditSelection(r, "all")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	params := selection.params(member.WorkspaceID)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var cursor auditCursor
		decoded, _ := base64.RawURLEncoding.DecodeString(raw)
		if json.Unmarshal(decoded, &cursor) != nil || cursor.Before.After(selection.Until) || cursor.Before.Before(selection.Since) {
			writeError(w, 400, "invalid audit cursor")
			return
		}
		id, ok := parseUUIDOrBadRequest(w, cursor.ID, "cursor id")
		if !ok {
			return
		}
		params.BeforeAt = pgtype.Timestamptz{Time: cursor.Before, Valid: true}
		params.BeforeID = id
	}
	rows, err := h.Queries.ListWorkbenchAuditPage(r.Context(), params)
	if err != nil {
		writeError(w, 500, "failed to list audit records")
		return
	}
	records := make([]auditRecordResponse, 0, len(rows))
	for _, row := range rows {
		records = append(records, auditRecordResponse{ListWorkbenchAuditPageRow: row, Details: json.RawMessage(row.Details)})
	}
	next := ""
	if len(rows) == 1000 {
		last := rows[len(rows)-1]
		raw, _ := json.Marshal(auditCursor{Until: selection.Until, Before: last.CreatedAt.Time, ID: uuidToString(last.ID)})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	writeJSON(w, 200, map[string]any{"records": records, "next_cursor": next, "range_start": selection.Since, "range_end": selection.Until, "scope": selection.Scope})
}
