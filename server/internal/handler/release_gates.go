package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func readReleaseGates(ctx context.Context, q *db.Queries, order db.WorkbenchReleaseOrder) ([]workbench.ReleaseGate, error) {
	var manifest workbench.ReleaseManifest
	var confirmation workbench.ReleaseConfirmations
	if err := json.Unmarshal(order.Manifest, &manifest); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(order.Confirmations, &confirmation); err != nil {
		return nil, err
	}
	if manifest.Delivery != nil {
		return readApplicationReleaseGates(ctx, q, order, *manifest.Delivery, confirmation)
	}
	project, err := q.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{WorkspaceID: order.WorkspaceID, ID: order.ProjectID})
	if err != nil {
		return nil, err
	}
	roles, _, err := workbench.LoadProjectRoles(ctx, q, project)
	if err != nil {
		return nil, err
	}
	acceptance := workbench.NewReleaseGate("acceptance")
	if len(manifest.Tasks) == 0 {
		acceptance.Block("acceptance", "")
	}
	for _, task := range manifest.Tasks {
		current, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{WorkspaceID: order.WorkspaceID, ID: parseUUID(task.ID)})
		if errors.Is(err, pgx.ErrNoRows) {
			acceptance.Block("acceptance", task.ID)
			continue
		}
		if err != nil {
			return nil, err
		}
		ready, err := readIssueReleaseReadiness(ctx, q, current)
		if err != nil {
			return nil, err
		}
		criteria, err := workbench.ParseRequirements(current.AcceptanceCriteria)
		if err != nil {
			return nil, err
		}
		if current.ProjectID != order.ProjectID || len(ready.Blockers) > 0 || !reflect.DeepEqual(criteria, task.Criteria) {
			acceptance.Block("acceptance", task.ID)
		}
		for _, merge := range ready.Merges {
			found := false
			for _, repo := range manifest.Repositories {
				if repo.RepoID == merge.RepoID && slices.Contains(repo.Merges, merge) {
					found = true
					break
				}
			}
			if !found {
				acceptance.Block("merge_changed", merge.RepoID)
			}
		}
	}
	security := workbench.NewReleaseGate("security")
	security.Required = order.Env == "prod"
	if security.Required && !roles.Config.Roles["sec"].Enabled && (!confirmation.SecurityWaiver || confirmation.RoleRevision != roles.Revision || confirmation.By == "") {
		security.Block("security_waiver", "")
	}
	for _, repo := range manifest.Repositories {
		if !security.Required || !roles.Config.Roles["sec"].Enabled {
			continue
		}
		id := parseUUID(repo.RepoID)
		count, err := q.CountBlockingSecurityChecks(ctx, db.CountBlockingSecurityChecksParams{WorkspaceID: order.WorkspaceID, RepoIds: []pgtype.UUID{id}})
		if err != nil {
			return nil, err
		}
		if count > 0 {
			security.Block("security_risks", repo.RepoID)
		}
		scan, err := q.LatestSecurityScanForArtifact(ctx, db.LatestSecurityScanForArtifactParams{WorkspaceID: order.WorkspaceID, RepoID: id, ArtifactVersion: repo.Tag})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err != nil || scan.Status != "completed" || len(repo.Merges) == 0 {
			security.Block("security_scan", repo.RepoID)
			continue
		}
		for _, merge := range repo.Merges {
			if scan.SourceCommit != merge.MergeCommit {
				security.Block("security_scan", repo.RepoID)
				break
			}
		}
	}
	changes := workbench.NewReleaseGate("changes")
	changes.Required = order.Env == "prod"
	if changes.Required && (!confirmation.Changes || !confirmation.Migration || confirmation.By == "") {
		changes.Block("change_confirmation", "")
	}
	targets, err := readReleaseTargetGate(ctx, q, order, manifest)
	if err != nil {
		return nil, err
	}
	return []workbench.ReleaseGate{acceptance, security, changes, targets}, nil
}

func readReleaseTargetGate(ctx context.Context, q *db.Queries, order db.WorkbenchReleaseOrder, manifest workbench.ReleaseManifest) (workbench.ReleaseGate, error) {
	gate := workbench.NewReleaseGate("targets")
	resources, err := q.ListProjectResourcesInWorkspace(ctx, db.ListProjectResourcesInWorkspaceParams{WorkspaceID: order.WorkspaceID, ProjectID: order.ProjectID})
	if err != nil {
		return gate, err
	}
	targets, err := q.ListDeployTargets(ctx, db.ListDeployTargetsParams{WorkspaceID: order.WorkspaceID, ProjectID: order.ProjectID})
	if err != nil {
		return gate, err
	}
	for _, repo := range manifest.Repositories {
		var profile workbench.RepositoryProfile
		for _, resource := range resources {
			if uuidToString(resource.ID) == repo.RepoID && resource.ResourceType == "github_repo" {
				profile, err = workbench.ReadRepositoryProfile(resource.ResourceRef)
				if err != nil {
					return gate, err
				}
				break
			}
		}
		if profile.Revision < 1 || len(profile.Delivery) == 0 {
			gate.Block("repository_profile", repo.RepoID)
			continue
		}
		for _, delivery := range profile.Delivery {
			if delivery == "none" {
				continue
			}
			kind := delivery
			if kind == "docker-image" {
				kind = "docker-host"
			}
			matched := false
			for _, target := range targets {
				if uuidToString(target.RepoID) != repo.RepoID || target.Env != order.Env || target.Kind != kind {
					continue
				}
				matched = true
				input := deployTargetResponse(target).DeployTargetInput
				id := uuidToString(target.ID)
				if len(input.MissingFields()) > 0 {
					gate.Block("target_config", id)
				}
				if !workbench.ParseDeployCheck(target.LastCheck).ValidAt(time.Now()) {
					gate.Block("target_check", id)
				}
				if target.RuntimeID.Valid {
					_, err := q.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{WorkspaceID: order.WorkspaceID, ID: target.RuntimeID})
					if errors.Is(err, pgx.ErrNoRows) {
						gate.Block("target_config", id)
					} else if err != nil {
						return gate, err
					}
				}
				if target.CredentialID.Valid {
					credential, err := q.GetWorkbenchCredentialForBinding(ctx, db.GetWorkbenchCredentialForBindingParams{WorkspaceID: order.WorkspaceID, ID: target.CredentialID})
					if err != nil && !errors.Is(err, pgx.ErrNoRows) {
						return gate, err
					}
					if err != nil || (credential.ProjectID.Valid && credential.ProjectID != order.ProjectID) || !slices.Contains(credential.Usage, "deploy") || (input.CredentialKind() != "" && credential.Kind != input.CredentialKind()) || (credential.ExpiresAt.Valid && !credential.ExpiresAt.Time.After(time.Now())) {
						gate.Block("target_credential", id)
					}
				}
			}
			if !matched {
				gate.Block("target_missing", repo.RepoID)
			}
		}
	}
	if len(manifest.Repositories) == 0 {
		gate.Block("repository_profile", "")
	}
	return gate, nil
}

func (h *Handler) ConfirmReleaseOrder(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	actorType, _ := h.resolveActor(r, uuidToString(member.UserID), uuidToString(member.WorkspaceID))
	if actorType != "member" {
		writeError(w, 403, "only a member can confirm a release")
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "releaseId"), "release id")
	if !ok {
		return
	}
	var input struct {
		ExpectedRevision int64 `json:"expected_revision"`
		Changes          bool  `json:"changes"`
		Migration        bool  `json:"migration"`
		SecurityWaiver   bool  `json:"security_waiver"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || input.ExpectedRevision < 1 {
		writeError(w, 400, "invalid release confirmation")
		return
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin confirmation")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	order, err := q.GetWorkbenchRelease(r.Context(), db.GetWorkbenchReleaseParams{WorkspaceID: member.WorkspaceID, ID: id})
	if err != nil {
		writeError(w, 404, "release order not found")
		return
	}
	project, err := q.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{WorkspaceID: member.WorkspaceID, ID: order.ProjectID})
	if err != nil {
		writeError(w, 404, "project not found")
		return
	}
	roles, _, err := workbench.LoadProjectRoles(r.Context(), q, project)
	if err != nil {
		writeError(w, 500, "failed to read project roles")
		return
	}
	confirmation := workbench.ReleaseConfirmations{Changes: input.Changes, Migration: input.Migration, SecurityWaiver: input.SecurityWaiver && !roles.Config.Roles["sec"].Enabled, By: uuidToString(member.UserID), At: time.Now().UTC().Format(time.RFC3339Nano), RoleRevision: roles.Revision}
	raw, _ := json.Marshal(confirmation)
	order, err = q.ConfirmWorkbenchRelease(r.Context(), db.ConfirmWorkbenchReleaseParams{WorkspaceID: member.WorkspaceID, ID: id, ExpectedRevision: input.ExpectedRevision, Confirmations: raw})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "release changed or cannot be confirmed in its current state")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to confirm release")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, "release_confirmed", map[string]any{"release_id": uuidToString(id), "confirmation": confirmation}); err != nil {
		writeError(w, 500, "failed to audit confirmation")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit confirmation")
		return
	}
	writeJSON(w, 200, releaseOrderResponse(order))
}
