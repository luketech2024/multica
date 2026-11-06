package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) executionPlanDependencies(w http.ResponseWriter, r *http.Request, q *db.Queries, parent db.Issue, tasks []workbench.PlanTask) (map[string][]string, bool) {
	ids := []pgtype.UUID{}
	for taskIndex, task := range tasks {
		if len(task.ExistingDependencies) > 100 {
			writeError(w, http.StatusBadRequest, "each subtask supports up to 100 existing dependencies")
			return nil, false
		}
		for index, value := range task.ExistingDependencies {
			id, ok := parseUUIDOrBadRequest(w, value, "existing_dependencies")
			if !ok {
				return nil, false
			}
			ids = append(ids, id)
			tasks[taskIndex].ExistingDependencies[index] = uuidToString(id)
		}
	}
	graph := map[string][]string{}
	if len(ids) == 0 {
		return graph, true
	}
	issues, err := q.ListWorkbenchDependencyIssues(r.Context(), db.ListWorkbenchDependencyIssuesParams{WorkspaceID: parent.WorkspaceID, IssueIds: ids})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read existing dependencies")
		return nil, false
	}
	for _, issue := range issues {
		var handoff workbench.Handoff
		raw := workbench.HandoffJSON(issue.Properties)
		if len(raw) > 0 && (json.Unmarshal(raw, &handoff) != nil || handoff.V != 2) {
			writeError(w, http.StatusConflict, "an existing dependency has invalid handoff data")
			return nil, false
		}
		if issue.ProjectID != parent.ProjectID || issue.ID == parent.ID || issue.ParentIssueID == parent.ID || issue.Status == "cancelled" || handoff.Superseded {
			writeError(w, http.StatusBadRequest, "dependencies must be available tasks in the same project, outside the plan being replaced")
			return nil, false
		}
		graph[uuidToString(issue.ID)] = handoff.DependsOn
	}
	return graph, true
}
