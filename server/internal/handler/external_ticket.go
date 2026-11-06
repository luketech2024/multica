package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) CountExternalTicketDuplicates(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	value := r.URL.Query().Get("value")
	if err := workbench.ValidateExternalTicket(value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if value == "" {
		writeJSON(w, http.StatusOK, map[string]int{"count": 0})
		return
	}
	var exclude pgtype.UUID
	if raw := r.URL.Query().Get("exclude_id"); raw != "" {
		exclude, ok = parseUUIDOrBadRequest(w, raw, "exclude_id")
		if !ok {
			return
		}
	}
	count, err := h.Queries.CountExternalTicketDuplicates(r.Context(), db.CountExternalTicketDuplicatesParams{WorkspaceID: member.WorkspaceID, ExternalTicketNo: value, ExcludeID: exclude})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check external ticket number")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"count": count})
}
