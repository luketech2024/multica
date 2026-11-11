package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type deliveryRecoveryInput struct {
	Items []string `json:"items"`
	Tasks []struct {
		IssueID  string   `json:"issue_id"`
		Revision int64    `json:"revision"`
		Role     string   `json:"role"`
		RepoID   string   `json:"repo_id"`
		Covers   []string `json:"covers"`
	} `json:"tasks"`
}

// Recovery adopts completed work explicitly selected by a member. Evidence and
// acceptance still pass through their normal validation and audit paths.
func (h *Handler) recoverDelivery(w http.ResponseWriter, r *http.Request, tx pgx.Tx, q *db.Queries, current db.Issue, criteria workbench.Requirements, revision int64, input *deliveryRecoveryInput, actorID string) {
	if current.Revision != revision {
		writeRevisionConflict(w, "issue", current.ID, revision, current.Revision)
		return
	}
	if input == nil || criteria.Plan != nil || criteria.Gate.Accept != nil || criteria.Change != nil || (current.Status != "in_review" && current.Status != "done" && current.Status != "blocked") {
		writeError(w, http.StatusConflict, "recovery requires a task without a structured execution plan")
		return
	}
	reserved, err := q.IssueHasWorkbenchRelease(r.Context(), db.IssueHasWorkbenchReleaseParams{WorkspaceID: current.WorkspaceID, IssueID: uuidToString(current.ID)})
	if err != nil || reserved {
		writeError(w, http.StatusConflict, "a task already included in a release cannot be recovered")
		return
	}
	active, err := q.WorkbenchPlanHasActiveRuns(r.Context(), db.WorkbenchPlanHasActiveRunsParams{ParentID: current.ID, WorkspaceID: current.WorkspaceID})
	if err != nil || active {
		writeError(w, http.StatusConflict, "wait for task execution to finish before recovering delivery")
		return
	}
	project, err := q.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: current.ProjectID, WorkspaceID: current.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if criteria.Team == nil {
		roles, _, err := workbench.LoadProjectRoles(r.Context(), q, project)
		if err != nil {
			writeError(w, http.StatusConflict, "configure the project team before recovery")
			return
		}
		criteria.Team = &workbench.ProjectTeamSnapshot{ProjectID: uuidToString(project.ID), ProjectRoleConfiguration: roles}
	}
	if criteria.Template == nil {
		row, err := q.GetProjectBusinessTemplate(r.Context(), db.GetProjectBusinessTemplateParams{WorkspaceID: current.WorkspaceID, ProjectID: current.ProjectID})
		if err == nil {
			var template workbench.ProjectTemplate
			if json.Unmarshal(row.ResourceRef, &template) != nil || workbench.ValidateBusinessTemplate(template.Content) != nil {
				writeError(w, http.StatusConflict, "invalid project template")
				return
			}
			criteria.Template = &template
		} else if !errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "failed to read project template")
			return
		}
	}
	if criteria.Gate.Spec == nil {
		if len(input.Items) == 0 || len(input.Items) > 200 {
			writeError(w, http.StatusBadRequest, "supply acceptance items before recovery")
			return
		}
		draft := criteria
		draft.Items = []workbench.Item{}
		for _, text := range input.Items {
			draft.Items = append(draft.Items, workbench.Item{Text: text, Origin: "user", Covers: []string{}})
		}
		before, _ := json.Marshal(criteria)
		incoming, _ := json.Marshal(draft)
		edited, err := workbench.EditRequirements(before, incoming, false, actorID, "member", time.Now())
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		criteria, _ = workbench.ParseRequirements(edited)
		stamp := time.Now().UTC().Format(time.RFC3339Nano)
		for i := range criteria.Items {
			criteria.Items[i].Checked = &workbench.Check{At: stamp, By: actorID}
		}
		checked, _ := json.Marshal(criteria)
		confirmed, err := workbench.EditRequirements(checked, nil, true, actorID, "member", time.Now())
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		criteria, _ = workbench.ParseRequirements(confirmed)
	}
	resources, err := q.ListProjectResourcesInWorkspace(r.Context(), db.ListProjectResourcesInWorkspaceParams{ProjectID: current.ProjectID, WorkspaceID: current.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read repositories")
		return
	}
	repos := map[string]bool{}
	for _, repo := range resources {
		repos[uuidToString(repo.ID)] = true
	}
	children, err := q.ListWorkbenchPlanChildren(r.Context(), db.ListWorkbenchPlanChildrenParams{ParentID: current.ID, WorkspaceID: current.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read subtasks")
		return
	}
	byID := map[string]db.Issue{}
	for _, child := range children {
		byID[uuidToString(child.ID)] = child
	}
	tasks := []workbench.PlanTask{}
	for _, selected := range input.Tasks {
		child, exists := byID[selected.IssueID]
		if !exists || child.Status != "done" || child.ProjectID != current.ProjectID || child.Revision != selected.Revision {
			writeError(w, http.StatusConflict, "select current completed subtasks from this project")
			return
		}
		tasks = append(tasks, workbench.PlanTask{Key: selected.IssueID, Title: child.Title, Role: selected.Role, RepoID: selected.RepoID, Covers: selected.Covers, Expected: []string{"delivery"}})
	}
	if err := workbench.ValidateExecutionPlan(tasks, criteria, repos, nil); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	criteria.Plan = &workbench.ExecutionPlan{Version: criteria.Version, Key: "recovery", Issues: []string{}}
	criteria.Delivery, criteria.Review, criteria.NeedsResplit = nil, nil, false
	coverage := map[string][]string{}
	changed := []db.Issue{}
	for _, task := range tasks {
		child := byID[task.Key]
		handoff := workbench.Handoff{V: 2, RequirementVersion: criteria.Version, Role: task.Role, RepoID: task.RepoID, Covers: task.Covers, DependsOn: []string{}, Expected: task.Expected, MaxRetry: 2}
		var properties map[string]any
		if json.Unmarshal(child.Properties, &properties) != nil || properties == nil {
			properties = map[string]any{}
		}
		properties["workbench_handoff"] = handoff
		raw, _ := json.Marshal(properties)
		after, err := q.RecoverWorkbenchHandoff(r.Context(), db.RecoverWorkbenchHandoffParams{ID: child.ID, WorkspaceID: child.WorkspaceID, Properties: raw, ExpectedRevision: child.Revision})
		if err != nil {
			writeError(w, http.StatusConflict, "subtask changed during recovery")
			return
		}
		changed = append(changed, after)
		criteria.Plan.Issues = append(criteria.Plan.Issues, task.Key)
		for _, item := range task.Covers {
			coverage[item] = append(coverage[item], task.Key)
		}
	}
	for i := range criteria.Items {
		criteria.Items[i].Covers = coverage[criteria.Items[i].ID]
	}
	raw, _ := json.Marshal(criteria)
	after, err := q.UpdateIssue(r.Context(), db.UpdateIssueParams{ID: current.ID, Status: strToText("in_review"), AcceptanceCriteria: raw, ExpectedRevision: pgtype.Int8{Int64: current.Revision, Valid: true}, AssigneeType: strToText("member"), AssigneeID: parseUUID(actorID), ProjectID: current.ProjectID, ParentIssueID: current.ParentIssueID, StartDate: current.StartDate, DueDate: current.DueDate, Stage: current.Stage, DuplicateOfIssueID: current.DuplicateOfIssueID})
	if err != nil {
		writeError(w, http.StatusConflict, "recovery could not be saved")
		return
	}
	activity, err := recordRequirementsActivity(r.Context(), q, current, after)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to audit recovery")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit recovery")
		return
	}
	h.publishRequirementsActivity(activity)
	h.publishIssueSnapshots(r.Context(), append(changed, after), "member", actorID)
	response := issueToResponse(after, h.getIssuePrefix(r.Context(), after.WorkspaceID))
	h.fillStatusCategory(r.Context(), after.WorkspaceID, &response)
	writeJSON(w, http.StatusOK, response)
}
