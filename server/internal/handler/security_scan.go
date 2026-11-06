package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func securityScanResponse(scan db.SecurityScan) map[string]any {
	return map[string]any{"id": uuidToString(scan.ID), "run_id": scan.RunID, "repo_id": uuidToString(scan.RepoID),
		"artifact_version": scan.ArtifactVersion, "source_commit": scan.SourceCommit, "status": scan.Status,
		"scanned_at": timestampToString(scan.ScannedAt), "recorded_at": timestampToString(scan.RecordedAt),
		"actor_type": scan.ActorType, "actor_id": uuidToString(scan.ActorID), "report": json.RawMessage(scan.Report)}
}

func (h *Handler) GetSecurityScan(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "scanId"), "scan id")
	if !ok {
		return
	}
	scan, err := h.Queries.GetSecurityScan(r.Context(), db.GetSecurityScanParams{WorkspaceID: member.WorkspaceID, ID: id})
	if err != nil {
		writeError(w, 404, "scan not found")
		return
	}
	writeJSON(w, 200, securityScanResponse(scan))
}

func (h *Handler) RecordSecurityScan(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	var input workbench.SecurityScanInput
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&input) != nil {
		writeError(w, 400, "invalid security report")
		return
	}
	if err := input.Validate(time.Now()); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	childID, ok := parseUUIDOrBadRequest(w, input.IssueID, "issue_id")
	if !ok {
		return
	}
	repoID, ok := parseUUIDOrBadRequest(w, input.RepoID, "repo_id")
	if !ok {
		return
	}
	child, ok := h.loadIssueForUser(w, r, input.IssueID)
	if !ok {
		return
	}
	if child.WorkspaceID != member.WorkspaceID || !child.ParentIssueID.Valid {
		writeError(w, 400, "security reports require a planned subtask")
		return
	}
	actorType, actorID := h.resolveActor(r, uuidToString(member.UserID), uuidToString(member.WorkspaceID))
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin scan recording")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err := q.LockProjectForChatSessionCreate(r.Context(), db.LockProjectForChatSessionCreateParams{WorkspaceID: member.WorkspaceID, ID: child.ProjectID}); err != nil {
		writeError(w, 404, "project not found")
		return
	}
	parent, err := q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: child.ParentIssueID, WorkspaceID: member.WorkspaceID})
	if err != nil {
		writeError(w, 404, "parent task not found")
		return
	}
	child, err = q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: childID, WorkspaceID: member.WorkspaceID})
	if err != nil || child.ParentIssueID != parent.ID || child.ProjectID != parent.ProjectID {
		writeError(w, 409, "subtask context changed")
		return
	}
	if actorType == "agent" && (child.AssigneeType.String != "agent" || uuidToString(child.AssigneeID) != actorID) {
		writeError(w, 403, "agents may only record scans for their own subtask")
		return
	}
	if child.Revision != input.IssueRevision {
		writeRevisionConflict(w, "issue", child.ID, input.IssueRevision, child.Revision)
		return
	}
	criteria, err := workbench.ParseRequirements(parent.AcceptanceCriteria)
	var handoff workbench.Handoff
	if err != nil || json.Unmarshal(workbench.HandoffJSON(child.Properties), &handoff) != nil || handoff.Role != "sec" || handoff.RepoID != input.RepoID ||
		criteria.Team == nil || !criteria.Team.Config.Roles["sec"].Enabled || criteria.Plan == nil || !slices.Contains(criteria.Plan.Issues, input.IssueID) ||
		criteria.Plan.Version != criteria.Version || handoff.RequirementVersion != criteria.Version || handoff.Superseded || criteria.Gate.Spec == nil || criteria.Change != nil || criteria.NeedsResplit {
		writeError(w, 409, "scan must belong to the current security plan and repository")
		return
	}
	if !slices.Contains([]string{"in_progress", "done", "blocked"}, child.Status) || parent.Status == "cancelled" {
		writeError(w, 409, "security execution has not started or was cancelled")
		return
	}
	repo, err := q.GetProjectResourceInWorkspaceForUpdate(r.Context(), db.GetProjectResourceInWorkspaceForUpdateParams{ID: repoID, WorkspaceID: member.WorkspaceID})
	if err != nil || repo.ProjectID != parent.ProjectID {
		writeError(w, 404, "repository not found in this project")
		return
	}
	raw, err := json.Marshal(input)
	if err != nil {
		writeError(w, 400, "invalid scan report")
		return
	}
	digest := sha256.Sum256(raw)
	digestText := hex.EncodeToString(digest[:])
	previous, err := q.GetSecurityScanByRun(r.Context(), db.GetSecurityScanByRunParams{WorkspaceID: member.WorkspaceID, RepoID: repoID, RunID: input.RunID})
	if err == nil {
		if previous.Digest != digestText {
			writeError(w, 409, "scan run id already records different evidence")
			return
		}
		writeJSON(w, 200, securityScanResponse(previous))
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 500, "failed to check scan identity")
		return
	}
	scan, err := q.CreateSecurityScan(r.Context(), db.CreateSecurityScanParams{WorkspaceID: member.WorkspaceID, ProjectID: parent.ProjectID,
		RepoID: repoID, IssueID: child.ID, RunID: input.RunID, ArtifactVersion: input.ArtifactVersion, SourceCommit: input.Commit,
		Status: input.Status, ScannedAt: pgtype.Timestamptz{Time: input.ScannedAt, Valid: true}, ActorType: actorType, ActorID: parseUUID(actorID), Report: raw, Digest: digestText})
	if err != nil {
		writeError(w, 500, "failed to store scan evidence")
		return
	}
	for _, finding := range input.Findings {
		err = q.UpsertSecurityCheck(r.Context(), db.UpsertSecurityCheckParams{WorkspaceID: member.WorkspaceID, ProjectID: parent.ProjectID, RepoID: repoID, IssueID: child.ID,
			Fingerprint: finding.Fingerprint, Severity: finding.Severity, Category: finding.Category, Title: finding.Title, Cwe: finding.CWE,
			Location: finding.Location, Recommendation: finding.Recommendation, ArtifactVersion: input.ArtifactVersion, FirstScanID: scan.ID, LastSeenAt: scan.ScannedAt})
		if err != nil {
			writeError(w, 500, "failed to record security findings")
			return
		}
	}
	details, _ := json.Marshal(map[string]any{"scan_id": uuidToString(scan.ID), "project_id": uuidToString(parent.ProjectID), "issue_id": input.IssueID, "repo_id": input.RepoID, "artifact_version": input.ArtifactVersion, "status": input.Status, "findings": len(input.Findings)})
	if _, err := q.CreateActivity(r.Context(), db.CreateActivityParams{ID: dbid.NewV7(), WorkspaceID: member.WorkspaceID,
		ActorType: strToText(actorType), ActorID: parseUUID(actorID), Action: "security_scan_recorded", Details: details}); err != nil {
		writeError(w, 500, "failed to record scan audit")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit scan report")
		return
	}
	writeJSON(w, 200, securityScanResponse(scan))
}
