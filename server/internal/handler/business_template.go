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
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) ListBusinessTemplates(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member", "viewer")
	if !ok {
		return
	}
	assets, err := listBusinessTemplateAssets(r.Context(), h.Queries, member.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load business templates")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": assets})
}

func (h *Handler) GetProjectBusinessTemplate(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	row, err := h.Queries.GetProjectBusinessTemplate(r.Context(), db.GetProjectBusinessTemplateParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"template": nil})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load project template")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"template": json.RawMessage(row.ResourceRef)})
}

func (h *Handler) SaveProjectBusinessTemplate(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	member, ok := h.requireWorkspaceRole(w, r, uuidToString(project.WorkspaceID), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	actor, _ := h.resolveActor(r, uuidToString(member.UserID), uuidToString(project.WorkspaceID))
	if actor != "member" {
		writeError(w, http.StatusForbidden, "only a member can change project templates")
		return
	}
	var input struct {
		Content          workbench.BusinessTemplate `json:"content"`
		ExpectedRevision int64                      `json:"expected_revision"`
		SourceRevision   *int64                     `json:"source_revision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 131072)).Decode(&input) != nil || input.ExpectedRevision < 0 {
		writeError(w, http.StatusBadRequest, "invalid template request")
		return
	}
	if err := workbench.ValidateBusinessTemplate(input.Content); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin template update")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err = q.LockProjectWorkbenchRoles(r.Context(), db.LockProjectWorkbenchRolesParams{WorkspaceID: project.WorkspaceID, ID: project.ID}); err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	row, err := q.GetProjectBusinessTemplate(r.Context(), db.GetProjectBusinessTemplateParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	var saved workbench.ProjectTemplate
	if err == nil {
		if json.Unmarshal(row.ResourceRef, &saved) != nil || saved.Revision < 1 {
			writeError(w, http.StatusConflict, "invalid saved project template")
			return
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to read project template")
		return
	}
	if saved.Revision != input.ExpectedRevision {
		writeError(w, http.StatusConflict, "project template changed; reload before saving")
		return
	}
	before := saved
	if input.SourceRevision != nil {
		assets, err := listBusinessTemplateAssets(r.Context(), q, project.WorkspaceID)
		if err != nil {
			writeError(w, 500, "failed to load template version")
			return
		}
		found := false
		for _, asset := range assets {
			if asset.ID == saved.SourceID && asset.Revision == *input.SourceRevision && asset.Revision > saved.SourceRevision {
				saved.SourceRevision = asset.Revision
				found = true
				break
			}
		}
		if !found {
			writeError(w, 409, "template version changed; reload before merging")
			return
		}
	}
	saved.Content, saved.Revision = input.Content, saved.Revision+1
	raw, _ := json.Marshal(saved)
	if row.ID.Valid {
		_, err = q.UpdateProjectResource(r.Context(), db.UpdateProjectResourceParams{ID: row.ID, ResourceRef: raw, Label: row.Label, Position: row.Position})
	} else {
		_, err = q.CreateProjectResource(r.Context(), db.CreateProjectResourceParams{ProjectID: project.ID, WorkspaceID: project.WorkspaceID, ResourceType: "workbench_template", ResourceRef: raw, Label: pgtype.Text{String: input.Content.Name, Valid: true}, CreatedBy: member.UserID})
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save project template")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, project.WorkspaceID, member.UserID, "project_template_updated", map[string]any{"project_id": uuidToString(project.ID), "before": before, "after": saved}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record template audit")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit template update")
		return
	}
	response := projectToResponse(project)
	response.IssueCount, response.DoneCount = h.loadProjectIssueStats(r.Context(), project.WorkspaceID, project.ID)
	response.ResourceCount = h.loadProjectResourceCount(r.Context(), project.ID)
	h.publish(protocol.EventProjectUpdated, uuidToString(project.WorkspaceID), "member", uuidToString(member.UserID), map[string]any{"project": response})
	writeJSON(w, http.StatusOK, map[string]any{"template": saved})
}

func listBusinessTemplateAssets(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID) ([]workbench.BusinessTemplateAsset, error) {
	assets, err := workbench.BuiltinBusinessTemplates()
	if err != nil {
		return nil, err
	}
	rows, err := q.ListWorkspaceBusinessTemplates(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		var content workbench.BusinessTemplate
		if err := json.Unmarshal(row.Content, &content); err != nil {
			return nil, err
		}
		assets = append(assets, workbench.BusinessTemplateAsset{ID: uuidToString(row.ID), Revision: row.Revision, Content: content, DisabledRoles: []string{}})
	}
	return assets, nil
}

func (h *Handler) SaveWorkspaceBusinessTemplate(w http.ResponseWriter, r *http.Request) {
	member, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	actor, _ := h.resolveActor(r, uuidToString(member.UserID), uuidToString(member.WorkspaceID))
	if actor != "member" {
		writeError(w, 403, "only members can publish business templates")
		return
	}
	var input struct {
		Content          workbench.BusinessTemplate `json:"content"`
		ExpectedRevision int64                      `json:"expected_revision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 131072)).Decode(&input) != nil {
		writeError(w, 400, "invalid template request")
		return
	}
	if err := workbench.ValidateBusinessTemplate(input.Content); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	id := dbid.NewV7()
	updating := chi.URLParam(r, "templateId") != ""
	if updating {
		id, ok = parseUUIDOrBadRequest(w, chi.URLParam(r, "templateId"), "template id")
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
		writeError(w, 500, "failed to begin template publication")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err = q.LockWorkspaceForChatSessionCreate(r.Context(), member.WorkspaceID); err != nil {
		writeError(w, 409, "workspace is unavailable")
		return
	}
	var before json.RawMessage
	if updating {
		row, err := q.GetWorkspaceBusinessTemplateForUpdate(r.Context(), db.GetWorkspaceBusinessTemplateForUpdateParams{WorkspaceID: member.WorkspaceID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, 404, "template not found")
			return
		}
		if err != nil {
			writeError(w, 500, "failed to load template")
			return
		}
		if row.Revision != input.ExpectedRevision {
			writeError(w, 409, "template changed; reload before publishing")
			return
		}
		before = row.Content
	}
	raw, _ := json.Marshal(input.Content)
	var revision int64
	if updating {
		row, updateErr := q.UpdateWorkspaceBusinessTemplate(r.Context(), db.UpdateWorkspaceBusinessTemplateParams{WorkspaceID: member.WorkspaceID, ID: id, Content: raw, ExpectedRevision: input.ExpectedRevision})
		err, revision = updateErr, row.Revision
	} else {
		row, createErr := q.CreateWorkspaceBusinessTemplate(r.Context(), db.CreateWorkspaceBusinessTemplateParams{WorkspaceID: member.WorkspaceID, ID: id, Content: raw})
		err, revision = createErr, row.Revision
	}
	if err != nil {
		writeError(w, 500, "failed to publish template")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, "business_template_published", map[string]any{"template_id": uuidToString(id), "revision": revision, "before": before, "after": input.Content}); err != nil {
		writeError(w, 500, "failed to record template audit")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit template publication")
		return
	}
	writeJSON(w, 200, workbench.BusinessTemplateAsset{ID: uuidToString(id), Revision: revision, Content: input.Content, DisabledRoles: []string{}})
}
