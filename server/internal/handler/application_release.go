package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) UpdateApplicationRelease(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	actor, _ := h.resolveActor(r, uuidToString(member.UserID), uuidToString(member.WorkspaceID))
	if actor != "member" {
		writeError(w, 403, "only members can edit releases")
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "releaseId"), "release id")
	if !ok {
		return
	}
	var input struct {
		Delivery         workbench.ApplicationRelease `json:"delivery"`
		ExpectedRevision int64                        `json:"expected_revision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input) != nil || input.ExpectedRevision < 1 {
		writeError(w, 400, "invalid release content")
		return
	}
	if err := input.Delivery.Validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	input.Delivery.Snapshot = nil
	input.Delivery.Results = nil
	for i := range input.Delivery.Changes {
		input.Delivery.Changes[i].ResolvedVersion = ""
		input.Delivery.Changes[i].ArtifactDigest = ""
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin release update")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	order, err := q.GetWorkbenchRelease(r.Context(), db.GetWorkbenchReleaseParams{WorkspaceID: member.WorkspaceID, ID: id})
	if err != nil {
		writeError(w, 404, "release not found")
		return
	}
	var manifest workbench.ReleaseManifest
	if json.Unmarshal(order.Manifest, &manifest) != nil || manifest.Delivery == nil {
		writeError(w, 400, "this release uses task sources")
		return
	}
	if _, err = q.LockProjectForChatSessionCreate(r.Context(), db.LockProjectForChatSessionCreateParams{ID: order.ProjectID, WorkspaceID: member.WorkspaceID}); err != nil {
		writeError(w, 409, "project is unavailable")
		return
	}
	config, _, err := readProjectDeployment(r.Context(), q, member.WorkspaceID, order.ProjectID)
	if err != nil {
		writeError(w, 500, "failed to load applications")
		return
	}
	for _, change := range input.Delivery.Changes {
		found := false
		for _, app := range config.Applications {
			if app.ID == change.ApplicationID {
				found = true
				break
			}
		}
		if !found {
			writeError(w, 400, "release application must belong to this project")
			return
		}
	}
	manifest.Delivery = &input.Delivery
	raw, _ := json.Marshal(manifest)
	order, err = q.UpdateReleaseManifest(r.Context(), db.UpdateReleaseManifestParams{WorkspaceID: member.WorkspaceID, ID: id, Manifest: raw, ExpectedRevision: input.ExpectedRevision})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "release changed or is no longer a draft")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to save release content")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, "release_content_updated", map[string]any{"release_id": uuidToString(id), "revision": order.Revision}); err != nil {
		writeError(w, 500, "failed to audit release update")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit release update")
		return
	}
	writeJSON(w, 200, releaseOrderResponse(order))
}

func readApplicationReleaseGates(ctx context.Context, q *db.Queries, order db.WorkbenchReleaseOrder, release workbench.ApplicationRelease, confirmation workbench.ReleaseConfirmations) ([]workbench.ReleaseGate, error) {
	config, _, err := readProjectDeployment(ctx, q, order.WorkspaceID, order.ProjectID)
	if err != nil {
		return nil, err
	}
	if release.Snapshot != nil {
		config = *release.Snapshot
	}
	source := workbench.NewReleaseGate("acceptance")
	source.Required = false
	security := workbench.NewReleaseGate("security")
	security.Required = order.Env == "prod"
	project, err := q.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{WorkspaceID: order.WorkspaceID, ID: order.ProjectID})
	if err != nil {
		return nil, err
	}
	roles, _, err := workbench.LoadProjectRoles(ctx, q, project)
	if err != nil {
		return nil, err
	}
	if security.Required {
		if !roles.Config.Roles["sec"].Enabled {
			if !confirmation.SecurityWaiver || confirmation.RoleRevision != roles.Revision || confirmation.By == "" {
				security.Block("security_waiver", "")
			}
		} else {
			for _, change := range release.Changes {
				if change.Type != "app" {
					continue
				}
				for _, app := range config.Applications {
					if app.ID != change.ApplicationID {
						continue
					}
					if app.RepoID == "" {
						security.Block("security_scan", app.ID)
						continue
					}
					scan, scanErr := q.LatestSecurityScanForArtifact(ctx, db.LatestSecurityScanForArtifactParams{WorkspaceID: order.WorkspaceID, RepoID: parseUUID(app.RepoID), ArtifactVersion: change.Version})
					if scanErr != nil && !errors.Is(scanErr, pgx.ErrNoRows) {
						return nil, scanErr
					}
					if scanErr != nil || scan.Status != "completed" || (app.SourceKind == "git" && scan.SourceCommit != change.Version) {
						security.Block("security_scan", app.ID)
					}
					count, countErr := q.CountBlockingSecurityChecks(ctx, db.CountBlockingSecurityChecksParams{WorkspaceID: order.WorkspaceID, RepoIds: []pgtype.UUID{parseUUID(app.RepoID)}})
					if countErr != nil {
						return nil, countErr
					}
					if count > 0 {
						security.Block("security_risks", app.ID)
					}
				}
			}
		}
	}
	changes := workbench.NewReleaseGate("changes")
	if !confirmation.Changes || !confirmation.Migration || confirmation.By == "" {
		changes.Block("change_confirmation", "")
	}
	targets := workbench.NewReleaseGate("targets")
	for _, message := range release.Missing(config, order.Env) {
		targets.Block("target_config", message)
	}
	return []workbench.ReleaseGate{source, security, changes, targets}, nil
}
