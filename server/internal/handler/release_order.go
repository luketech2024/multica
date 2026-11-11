package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func releaseOrderResponse(row db.WorkbenchReleaseOrder) map[string]any {
	return map[string]any{
		"id": uuidToString(row.ID), "workspace_id": uuidToString(row.WorkspaceID), "project_id": uuidToString(row.ProjectID),
		"identifier": fmt.Sprintf("REL-%02d", row.Number), "env": row.Env, "status": row.Status,
		"manifest": json.RawMessage(row.Manifest), "revision": row.Revision, "created_by": uuidToString(row.CreatedBy),
		"confirmations": json.RawMessage(row.Confirmations),
		"lease_until":   row.LeaseUntil,
		"created_at":    timestampToString(row.CreatedAt), "updated_at": timestampToString(row.UpdatedAt),
	}
}

func (h *Handler) GetReleaseOrder(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "releaseId"), "release id")
	if !ok {
		return
	}
	row, err := h.Queries.GetWorkbenchRelease(r.Context(), db.GetWorkbenchReleaseParams{WorkspaceID: member.WorkspaceID, ID: id})
	if err != nil {
		writeError(w, http.StatusNotFound, "release order not found")
		return
	}
	gates, err := readReleaseGates(r.Context(), h.Queries, row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to evaluate release gates")
		return
	}
	response := releaseOrderResponse(row)
	response["gates"] = gates
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ListReleaseCandidates(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	project, ok := h.loadProjectForResource(w, r, r.URL.Query().Get("project_id"))
	if !ok {
		return
	}
	env := r.URL.Query().Get("env")
	if env != "staging" && env != "prod" {
		writeError(w, http.StatusBadRequest, "invalid release environment")
		return
	}
	before := parseUUID("ffffffff-ffff-ffff-ffff-ffffffffffff")
	if raw := r.URL.Query().Get("before"); raw != "" {
		before, ok = parseUUIDOrBadRequest(w, raw, "before")
		if !ok {
			return
		}
	}
	rows, err := h.Queries.ListWorkbenchReleaseCandidates(r.Context(), db.ListWorkbenchReleaseCandidatesParams{WorkspaceID: member.WorkspaceID, ProjectID: project.ID, BeforeID: before})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list release candidates")
		return
	}
	reservations, err := h.Queries.ListWorkbenchReleaseReservations(r.Context(), db.ListWorkbenchReleaseReservationsParams{WorkspaceID: member.WorkspaceID, ProjectID: project.ID, Env: env})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read artifact reservations")
		return
	}
	reserved := map[string]bool{}
	for _, entry := range reservations {
		reserved[uuidToString(entry.RepoID)+"/"+entry.MergeCommit] = true
	}
	prefix := h.getIssuePrefix(r.Context(), member.WorkspaceID)
	items := []map[string]any{}
	blocked := []map[string]any{}
	for _, row := range rows {
		ready, err := readIssueReleaseReadiness(r.Context(), h.Queries, row)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to validate candidate evidence")
			return
		}
		if len(ready.Blockers) > 0 {
			blocked = append(blocked, map[string]any{"id": uuidToString(row.ID), "identifier": issueToResponse(row, prefix).Identifier, "title": row.Title, "blockers": ready.Blockers})
			continue
		}
		available := true
		for _, merge := range ready.Merges {
			if reserved[merge.RepoID+"/"+merge.MergeCommit] {
				available = false
				break
			}
		}
		if available {
			items = append(items, map[string]any{"id": uuidToString(row.ID), "identifier": issueToResponse(row, prefix).Identifier, "title": row.Title, "revision": row.Revision, "merges": ready.Merges})
		} else {
			blocked = append(blocked, map[string]any{"id": uuidToString(row.ID), "identifier": issueToResponse(row, prefix).Identifier, "title": row.Title, "blockers": []workbench.ReleaseBlocker{{Code: "reserved"}}})
		}
	}
	var next *string
	if len(rows) == 100 {
		value := uuidToString(rows[len(rows)-1].ID)
		next = &value
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": items, "blocked": blocked, "next_cursor": next})
}

func (h *Handler) ListReleaseOrders(w http.ResponseWriter, r *http.Request) {
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
	env := r.URL.Query().Get("env")
	if env != "" && env != "staging" && env != "prod" {
		writeError(w, http.StatusBadRequest, "invalid release environment")
		return
	}
	rows, err := h.Queries.ListWorkbenchReleases(r.Context(), db.ListWorkbenchReleasesParams{WorkspaceID: member.WorkspaceID, BeforeID: before, ProjectID: project, EnvFilter: env})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list release orders")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, releaseOrderResponse(row))
	}
	var next *string
	if len(rows) == 100 {
		value := uuidToString(rows[len(rows)-1].ID)
		next = &value
	}
	writeJSON(w, http.StatusOK, map[string]any{"releases": items, "next_cursor": next})
}

func (h *Handler) CreateReleaseOrder(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	actor, _ := h.resolveActor(r, uuidToString(member.UserID), uuidToString(member.WorkspaceID))
	if actor != "member" {
		writeError(w, http.StatusForbidden, "only members can create release orders")
		return
	}
	var input workbench.ReleaseOrderInput
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "invalid release order")
		return
	}
	project, ok := h.loadProjectForResource(w, r, input.ProjectID)
	if !ok {
		return
	}
	input.ProjectID = uuidToString(project.ID)
	requestID, ok := parseUUIDOrBadRequest(w, input.RequestID, "request_id")
	if !ok {
		return
	}
	input.RequestID = uuidToString(requestID)
	for i := range input.Tasks {
		id, valid := parseUUIDOrBadRequest(w, input.Tasks[i].ID, "task id")
		if !valid {
			return
		}
		input.Tasks[i].ID = uuidToString(id)
	}
	if err := input.Normalize(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	prefix := h.getIssuePrefix(r.Context(), member.WorkspaceID)
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin release creation")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err = q.LockProjectForChatSessionCreate(r.Context(), db.LockProjectForChatSessionCreateParams{ID: project.ID, WorkspaceID: member.WorkspaceID}); err != nil {
		writeError(w, http.StatusConflict, "project is unavailable")
		return
	}
	prior, err := q.GetWorkbenchReleaseByRequest(r.Context(), db.GetWorkbenchReleaseByRequestParams{WorkspaceID: member.WorkspaceID, RequestID: requestID})
	if err == nil {
		if prior.RequestHash != input.Digest() {
			writeError(w, http.StatusConflict, "request_id already belongs to a different release request")
			return
		}
		writeJSON(w, http.StatusOK, releaseOrderResponse(prior))
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to check release request")
		return
	}
	manifest := workbench.ReleaseManifest{Tasks: []workbench.ReleaseTaskSnapshot{}, Repositories: []workbench.ReleaseRepositorySnapshot{}}
	if input.Delivery != nil {
		config, _, loadErr := readProjectDeployment(r.Context(), q, member.WorkspaceID, project.ID)
		if loadErr != nil {
			writeError(w, 500, "failed to read application configuration")
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
				writeError(w, 400, "release application must belong to the project")
				return
			}
		}
		manifest.Delivery = input.Delivery
	}
	repositories := map[string][]workbench.ReleaseMerge{}
	for _, task := range input.Tasks {
		current, loadErr := q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: parseUUID(task.ID), WorkspaceID: member.WorkspaceID})
		if loadErr != nil || current.ProjectID != project.ID || current.ParentIssueID.Valid {
			writeError(w, http.StatusNotFound, "release task not found in this project")
			return
		}
		if current.Revision != task.ExpectedRevision {
			writeRevisionConflict(w, "issue", current.ID, task.ExpectedRevision, current.Revision)
			return
		}
		ready, readErr := readIssueReleaseReadiness(r.Context(), q, current)
		if readErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to validate release evidence")
			return
		}
		if len(ready.Blockers) != 0 {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "task is not ready for release", "issue_id": task.ID, "blockers": ready.Blockers})
			return
		}
		criteria, parseErr := workbench.ParseRequirements(current.AcceptanceCriteria)
		if parseErr != nil {
			writeError(w, http.StatusConflict, "task acceptance is unavailable")
			return
		}
		manifest.Tasks = append(manifest.Tasks, workbench.ReleaseTaskSnapshot{ID: task.ID, Identifier: issueToResponse(current, prefix).Identifier, Title: current.Title, Revision: current.Revision, Criteria: criteria})
		for _, merge := range ready.Merges {
			repositories[merge.RepoID] = append(repositories[merge.RepoID], merge)
		}
	}
	for repoID := range input.Tags {
		if _, exists := repositories[repoID]; !exists {
			writeError(w, http.StatusBadRequest, "tag refers to a repository outside this release")
			return
		}
	}
	number, err := q.NextWorkbenchReleaseNumber(r.Context(), member.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to allocate release number")
		return
	}
	ids := make([]string, 0, len(repositories))
	for id := range repositories {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	id := dbid.NewV7()
	for _, repoID := range ids {
		tag := input.Tags[repoID]
		if tag == "" {
			tag = fmt.Sprintf("%s-%d", time.Now().UTC().Format("2006.01.02"), number)
		}
		manifest.Repositories = append(manifest.Repositories, workbench.ReleaseRepositorySnapshot{RepoID: repoID, Tag: tag, Merges: repositories[repoID]})
		seen := map[string]bool{}
		for _, merge := range repositories[repoID] {
			if seen[merge.MergeCommit] {
				continue
			}
			seen[merge.MergeCommit] = true
			err = q.ReserveWorkbenchReleaseArtifact(r.Context(), db.ReserveWorkbenchReleaseArtifactParams{WorkspaceID: member.WorkspaceID, ReleaseID: id, RepoID: parseUUID(repoID), MergeCommit: merge.MergeCommit, Env: input.Env})
			if err != nil {
				var constraint *pgconn.PgError
				if errors.As(err, &constraint) && constraint.Code == "23505" {
					writeError(w, http.StatusConflict, "a repository artifact already belongs to a release in this environment")
					return
				}
				writeError(w, http.StatusInternalServerError, "failed to reserve release artifacts")
				return
			}
		}
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode release snapshot")
		return
	}
	row, err := q.CreateWorkbenchRelease(r.Context(), db.CreateWorkbenchReleaseParams{ID: id, WorkspaceID: member.WorkspaceID, ProjectID: project.ID, Number: number, Env: input.Env, RequestID: requestID, RequestHash: input.Digest(), Manifest: raw, CreatedBy: member.UserID})
	if err != nil {
		writeError(w, http.StatusConflict, "release request could not be saved")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, "release_created", map[string]any{"release_id": uuidToString(id), "project_id": input.ProjectID, "env": input.Env, "tasks": input.Tasks}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record release audit")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit release order")
		return
	}
	writeJSON(w, http.StatusCreated, releaseOrderResponse(row))
}
