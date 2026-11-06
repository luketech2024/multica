package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issueposition"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func (h *Handler) SaveExecutionPlan(w http.ResponseWriter, r *http.Request) {
	r = h.withWakeupActor(r)
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	member, ok := h.requireWorkspaceRole(w, r, uuidToString(issue.WorkspaceID), "workspace not found", "owner", "admin", "member")
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, uuidToString(member.UserID), uuidToString(issue.WorkspaceID))
	if actorType != "member" {
		writeError(w, http.StatusForbidden, "only a member can approve an execution plan")
		return
	}
	var input struct {
		ExpectedRevision int64                `json:"expected_revision"`
		Version          int                  `json:"version"`
		Key              string               `json:"idempotency_key"`
		Tasks            []workbench.PlanTask `json:"tasks"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&input) != nil || input.ExpectedRevision < 1 || strings.TrimSpace(input.Key) == "" || len(input.Key) > 100 {
		writeError(w, http.StatusBadRequest, "revision, version, idempotency_key and tasks are required")
		return
	}
	payload, _ := json.Marshal(input.Tasks)
	digestBytes := sha256.Sum256(payload)
	digest := hex.EncodeToString(digestBytes[:])
	policy := service.ResolveIssueCountPolicy(r.Context(), h.IssueService.Entitlements, issue.WorkspaceID)
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin execution plan")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	current, err := q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	criteria, err := workbench.ParseRequirements(current.AcceptanceCriteria)
	if err != nil || !current.ProjectID.Valid || current.ParentIssueID.Valid {
		writeError(w, http.StatusConflict, "execution plans require a parent task in a project")
		return
	}
	if criteria.Plan != nil && criteria.Plan.Key == input.Key {
		if criteria.Plan.Digest != digest || criteria.Plan.Version != input.Version {
			writeError(w, http.StatusConflict, "idempotency key already used for another plan")
			return
		}
		response := issueToResponse(current, h.getIssuePrefix(r.Context(), current.WorkspaceID))
		h.fillStatusCategory(r.Context(), current.WorkspaceID, &response)
		writeJSON(w, http.StatusOK, response)
		return
	}
	if current.Revision != input.ExpectedRevision {
		writeRevisionConflict(w, "issue", current.ID, input.ExpectedRevision, current.Revision)
		return
	}
	if criteria.Version != input.Version || (current.Status != "todo" && current.Status != "backlog") || (criteria.Plan != nil && !criteria.NeedsResplit) {
		writeError(w, http.StatusConflict, "this requirement version is not ready for a new plan")
		return
	}
	resources, err := q.ListProjectResourcesInWorkspace(r.Context(), db.ListProjectResourcesInWorkspaceParams{ProjectID: current.ProjectID, WorkspaceID: current.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read project repositories")
		return
	}
	repos := map[string]bool{}
	for _, resource := range resources {
		repos[uuidToString(resource.ID)] = true
	}
	existing, ok := h.executionPlanDependencies(w, r, q, current, input.Tasks)
	if !ok {
		return
	}
	if err := workbench.ValidateExecutionPlan(input.Tasks, criteria, repos, existing); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	active, err := q.WorkbenchPlanHasActiveRuns(r.Context(), db.WorkbenchPlanHasActiveRunsParams{ParentID: current.ID, WorkspaceID: current.WorkspaceID})
	if err != nil || active {
		writeError(w, http.StatusConflict, "stop affected execution before replacing its plan")
		return
	}
	assignees := map[string]pgtype.UUID{}
	for _, task := range input.Tasks {
		role := criteria.Team.Config.Roles[task.Role]
		if role.AgentID == nil {
			continue
		}
		id, ok := parseUUIDOrBadRequest(w, *role.AgentID, "role agent_id")
		if !ok {
			return
		}
		agent, err := q.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{ID: id, WorkspaceID: current.WorkspaceID})
		if err != nil || !h.canInvokeAgent(r.Context(), agent, actorType, actorID, uuidToString(member.UserID), uuidToString(current.WorkspaceID)) {
			writeError(w, http.StatusConflict, "a configured executor is unavailable or inaccessible")
			return
		}
		assignees[task.Role] = id
	}
	previous, err := q.ListWorkbenchPlanChildren(r.Context(), db.ListWorkbenchPlanChildrenParams{ParentID: current.ID, WorkspaceID: current.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read previous subtasks")
		return
	}
	changed := []db.Issue{}
	for _, child := range previous {
		updated, err := q.SupersedeWorkbenchPlanChild(r.Context(), db.SupersedeWorkbenchPlanChildParams{ID: child.ID, WorkspaceID: current.WorkspaceID})
		if err != nil {
			writeError(w, http.StatusConflict, "failed to retire the previous plan")
			return
		}
		changed = append(changed, updated)
	}
	ids := map[string]pgtype.UUID{}
	for _, task := range input.Tasks {
		ids[task.Key] = dbid.NewV7()
	}
	plan := &workbench.ExecutionPlan{Version: input.Version, Key: input.Key, Digest: digest, Issues: []string{}}
	coverage := map[string][]string{}
	for _, task := range input.Tasks {
		dependencies := append([]string{}, task.ExistingDependencies...)
		for _, key := range task.DependsOn {
			dependencies = append(dependencies, uuidToString(ids[key]))
		}
		handoff := workbench.Handoff{V: 2, RequirementVersion: input.Version, Role: task.Role, RepoID: task.RepoID, Covers: task.Covers, DependsOn: dependencies, SerialGroup: task.SerialGroup, MaxRetry: 2, Expected: task.Expected}
		properties, _ := json.Marshal(map[string]any{"workbench_handoff": handoff})
		number, err := service.AllocateIssueNumber(r.Context(), q, current.WorkspaceID, policy)
		if err != nil {
			writeError(w, http.StatusConflict, "cannot allocate subtask number")
			return
		}
		position, err := issueposition.NextTopPosition(r.Context(), tx, current.WorkspaceID, "todo")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to position subtask")
			return
		}
		assigneeType, assigneeID := "member", member.UserID
		if id, exists := assignees[task.Role]; exists {
			assigneeType, assigneeID = "agent", id
		}
		child, err := q.CreateIssue(r.Context(), db.CreateIssueParams{
			ID: ids[task.Key], WorkspaceID: current.WorkspaceID, ProjectID: current.ProjectID, ParentIssueID: current.ID,
			Title: strings.TrimSpace(task.Title), Description: strToText(task.Description), Status: "todo", Priority: current.Priority,
			AssigneeType: strToText(assigneeType), AssigneeID: assigneeID, CreatorType: actorType, CreatorID: member.UserID,
			Number: number, Position: position, Properties: properties,
		})
		if err != nil {
			slog.Error("create execution plan subtask", "error", err)
			writeError(w, http.StatusConflict, "failed to create the complete execution plan")
			return
		}
		changed = append(changed, child)
		id := uuidToString(child.ID)
		plan.Issues = append(plan.Issues, id)
		for _, item := range task.Covers {
			coverage[item] = append(coverage[item], id)
		}
	}
	for index := range criteria.Items {
		criteria.Items[index].Covers = coverage[criteria.Items[index].ID]
	}
	criteria.Plan, criteria.NeedsResplit = plan, false
	raw, _ := json.Marshal(criteria)
	after, err := q.UpdateIssue(r.Context(), db.UpdateIssueParams{
		ID: current.ID, AcceptanceCriteria: raw, Status: strToText("todo"), ExpectedRevision: pgtype.Int8{Int64: current.Revision, Valid: true},
		AssigneeType: current.AssigneeType, AssigneeID: current.AssigneeID, ProjectID: current.ProjectID, ParentIssueID: current.ParentIssueID,
		StartDate: current.StartDate, DueDate: current.DueDate, Stage: current.Stage, DuplicateOfIssueID: current.DuplicateOfIssueID,
	})
	if err != nil {
		writeError(w, http.StatusConflict, "execution plan could not be saved")
		return
	}
	activity, err := recordRequirementsActivity(r.Context(), q, current, after)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record execution plan")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit execution plan")
		return
	}
	h.publishRequirementsActivity(activity)
	h.publishIssueSnapshots(r.Context(), append(changed, after), actorType, actorID)
	response := issueToResponse(after, h.getIssuePrefix(r.Context(), after.WorkspaceID))
	h.fillStatusCategory(r.Context(), after.WorkspaceID, &response)
	writeJSON(w, http.StatusOK, response)
}
