package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type SecurityCheckResponse struct {
	db.SecurityCheck
	Identifier      string `json:"identifier"`
	EffectiveStatus string `json:"effective_status"`
}

func securityCheckResponse(check db.SecurityCheck) SecurityCheckResponse {
	status := check.Status
	if status == "accepted" && check.AcceptedUntil.Valid && !check.AcceptedUntil.Time.After(time.Now()) {
		status = "unresolved"
	}
	return SecurityCheckResponse{SecurityCheck: check, Identifier: fmt.Sprintf("SEC-%02d", check.Number), EffectiveStatus: status}
}

func (h *Handler) GetSecurityCheck(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "checkId"), "check id")
	if !ok {
		return
	}
	check, err := h.Queries.GetSecurityCheck(r.Context(), db.GetSecurityCheckParams{WorkspaceID: member.WorkspaceID, ID: id})
	if err != nil {
		writeError(w, 404, "security check not found")
		return
	}
	writeJSON(w, 200, securityCheckResponse(check))
}

func (h *Handler) ListSecurityChecks(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	before := parseUUID("ffffffff-ffff-ffff-ffff-ffffffffffff")
	var project pgtype.UUID
	if raw := r.URL.Query().Get("before"); raw != "" {
		before, ok = parseUUIDOrBadRequest(w, raw, "before")
		if !ok {
			return
		}
	}
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		project, ok = parseUUIDOrBadRequest(w, raw, "project_id")
		if !ok {
			return
		}
	}
	status := r.URL.Query().Get("status")
	if !slices.Contains([]string{"", "unresolved", "pending_recheck", "fixed", "accepted"}, status) {
		writeError(w, 400, "invalid security filter")
		return
	}
	checks, err := h.Queries.ListSecurityChecks(r.Context(), db.ListSecurityChecksParams{WorkspaceID: member.WorkspaceID, BeforeID: before, ProjectID: project, StatusFilter: status})
	if err != nil {
		writeError(w, 500, "failed to list security checks")
		return
	}
	counts, err := h.Queries.SecurityCheckCounts(r.Context(), db.SecurityCheckCountsParams{WorkspaceID: member.WorkspaceID, ProjectID: project})
	if err != nil {
		writeError(w, 500, "failed to count security checks")
		return
	}
	items := make([]SecurityCheckResponse, 0, len(checks))
	for _, check := range checks {
		items = append(items, securityCheckResponse(check))
	}
	next := ""
	if len(checks) == 100 {
		next = uuidToString(checks[len(checks)-1].ID)
	}
	writeJSON(w, 200, map[string]any{"checks": items, "counts": counts, "next_cursor": next})
}

func (h *Handler) UpdateSecurityCheck(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	actorType, _ := h.resolveActor(r, uuidToString(member.UserID), uuidToString(member.WorkspaceID))
	if actorType != "member" {
		writeError(w, 403, "security decisions require a member")
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "checkId"), "check id")
	if !ok {
		return
	}
	var input struct {
		Action           string `json:"action"`
		ExpectedRevision int64  `json:"expected_revision"`
		ArtifactVersion  string `json:"artifact_version"`
		ScanID           string `json:"scan_id"`
		Reason           string `json:"reason"`
		Days             int    `json:"days"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input) != nil || input.ExpectedRevision < 1 {
		writeError(w, 400, "decision and expected_revision are required")
		return
	}
	check, err := h.Queries.GetSecurityCheck(r.Context(), db.GetSecurityCheckParams{WorkspaceID: member.WorkspaceID, ID: id})
	if err != nil {
		writeError(w, 404, "security check not found")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin security decision")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err := q.LockProjectForChatSessionCreate(r.Context(), db.LockProjectForChatSessionCreateParams{WorkspaceID: member.WorkspaceID, ID: check.ProjectID}); err != nil {
		writeError(w, 404, "project not found")
		return
	}
	if _, err := q.GetProjectResourceInWorkspaceForUpdate(r.Context(), db.GetProjectResourceInWorkspaceForUpdateParams{ID: check.RepoID, WorkspaceID: member.WorkspaceID}); err != nil {
		writeError(w, 404, "repository not found")
		return
	}
	check, err = q.LockSecurityCheck(r.Context(), db.LockSecurityCheckParams{WorkspaceID: member.WorkspaceID, ID: id})
	if err != nil {
		writeError(w, 404, "security check not found")
		return
	}
	if check.Revision != input.ExpectedRevision {
		writeRevisionConflict(w, "security check", check.ID, input.ExpectedRevision, check.Revision)
		return
	}
	if check.Status == "fixed" {
		writeError(w, 409, "a new scan must reopen a fixed risk")
		return
	}
	now := time.Now().UTC()
	params := db.UpdateSecurityCheckParams{WorkspaceID: member.WorkspaceID, ID: id, ExpectedRevision: input.ExpectedRevision,
		Status: check.Status, FixVersion: check.FixVersion, FixRequestedAt: check.FixRequestedAt, FixedScanID: check.FixedScanID,
		AcceptedUntil: check.AcceptedUntil, AcceptanceReason: check.AcceptanceReason}
	switch input.Action {
	case "request_recheck":
		input.ArtifactVersion = strings.TrimSpace(input.ArtifactVersion)
		if input.ArtifactVersion == "" || len(input.ArtifactVersion) > 200 {
			writeError(w, 400, "target artifact version is required")
			return
		}
		params.Status, params.FixVersion = "pending_recheck", input.ArtifactVersion
		params.FixRequestedAt = pgtype.Timestamptz{Time: now, Valid: true}
		params.AcceptedUntil, params.AcceptanceReason = pgtype.Timestamptz{}, ""
	case "accept":
		if member.Role != "owner" && member.Role != "admin" {
			writeError(w, 403, "only workspace managers can accept a risk")
			return
		}
		if err := workbench.ValidateSecurityAcceptance(check.Severity, input.Reason, input.Days); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		params.Status, params.AcceptanceReason = "accepted", strings.TrimSpace(input.Reason)
		params.AcceptedUntil = pgtype.Timestamptz{Time: now.Add(time.Duration(input.Days) * 24 * time.Hour), Valid: true}
	case "recheck":
		if check.Status != "pending_recheck" || !check.FixRequestedAt.Valid {
			writeError(w, 409, "mark the risk as awaiting recheck first")
			return
		}
		scanID, ok := parseUUIDOrBadRequest(w, input.ScanID, "scan_id")
		if !ok {
			return
		}
		scan, err := q.GetSecurityScan(r.Context(), db.GetSecurityScanParams{WorkspaceID: member.WorkspaceID, ID: scanID})
		if err != nil {
			writeError(w, 404, "scan record not found")
			return
		}
		var report workbench.SecurityScanInput
		if json.Unmarshal(scan.Report, &report) != nil {
			writeError(w, 409, "scan evidence is unavailable")
			return
		}
		if err := workbench.ValidateSecurityRecheck(report, uuidToString(check.RepoID), check.FixVersion, check.Fingerprint, check.FixRequestedAt.Time); err != nil {
			writeError(w, 409, err.Error())
			return
		}
		latest, err := q.LatestSecurityScanForArtifact(r.Context(), db.LatestSecurityScanForArtifactParams{WorkspaceID: member.WorkspaceID, RepoID: check.RepoID, ArtifactVersion: check.FixVersion})
		if err != nil || latest.ID != scan.ID {
			writeError(w, 409, "use the latest scan of the target artifact")
			return
		}
		params.Status, params.FixedScanID = "fixed", scan.ID
	default:
		writeError(w, 400, "unsupported security decision")
		return
	}
	after, err := q.UpdateSecurityCheck(r.Context(), params)
	if err != nil {
		writeError(w, 409, "security check changed; reload and retry")
		return
	}
	if err := writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, "security_"+input.Action, map[string]any{"check_id": uuidToString(id), "project_id": uuidToString(check.ProjectID), "before": check, "after": after}); err != nil {
		writeError(w, 500, "failed to record security audit")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit security decision")
		return
	}
	writeJSON(w, 200, securityCheckResponse(after))
}
