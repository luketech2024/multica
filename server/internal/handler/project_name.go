package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The caller holds this transaction through the project write, so create and
// rename cannot both claim the same workspace name after passing validation.
func reserveProjectTitle(w http.ResponseWriter, r *http.Request, q *db.Queries, workspaceID pgtype.UUID, title string, excludeID pgtype.UUID) bool {
	if err := q.LockProjectNames(r.Context(), workspaceID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate project name")
		return false
	}
	exists, err := q.ProjectTitleExists(r.Context(), db.ProjectTitleExistsParams{WorkspaceID: workspaceID, Title: title, ExcludeID: excludeID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate project name")
		return false
	}
	if exists {
		writeError(w, http.StatusConflict, "project name already exists in this workspace")
		return false
	}
	return true
}
