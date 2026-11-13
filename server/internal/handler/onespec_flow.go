package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type oneSpecResponse struct {
	Flow       *workbench.OneSpecFlow `json:"flow"`
	Revision   int64                  `json:"revision"`
	CanConfirm bool                   `json:"can_confirm"`
}

func oneSpecCanConfirm(flow workbench.OneSpecFlow, project db.Project, actorType, actorID string) bool {
	return actorType == "member" && (actorID == flow.ApproverID || (project.LeadType.String == "member" && actorID == uuidToString(project.LeadID)))
}

func (h *Handler) GetOneSpecFlow(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	row, err := h.Queries.GetOneSpecFlow(r.Context(), db.GetOneSpecFlowParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 200, oneSpecResponse{})
		return
	}
	if err != nil {
		writeError(w, 500, "failed to read OneSpec workflow")
		return
	}
	flow, err := parseOneSpecFlow(row)
	if err != nil {
		writeError(w, 500, "invalid workflow state")
		return
	}
	member, ok := h.workspaceMember(w, r, uuidToString(issue.WorkspaceID))
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, uuidToString(member.UserID), uuidToString(issue.WorkspaceID))
	project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, 404, "project not found")
		return
	}
	writeJSON(w, 200, oneSpecResponse{Flow: &flow, Revision: row.Revision, CanConfirm: oneSpecCanConfirm(flow, project, actorType, actorID)})
}

func (h *Handler) UpdateOneSpecFlow(w http.ResponseWriter, r *http.Request) {
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
	var input struct {
		Release          workbench.ReleaseOrderInput   `json:"release"`
		ExpectedRevision int64                         `json:"expected_revision"`
		Kind             string                        `json:"kind"`
		Artifact         workbench.OneSpecArtifact     `json:"artifact"`
		Stage            string                        `json:"stage"`
		Version          int                           `json:"version"`
		Approved         bool                          `json:"approved"`
		Reason           string                        `json:"reason"`
		Operation        workbench.OneSpecGitOperation `json:"operation"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&input) != nil || input.ExpectedRevision < 0 {
		writeError(w, 400, "invalid workflow request")
		return
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin workflow update")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	issue, err = q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, 404, "task not found")
		return
	}
	project, err := q.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, 409, "workflow requires a project")
		return
	}
	row, err := q.LockOneSpecFlow(r.Context(), db.LockOneSpecFlowParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
	action := chi.URLParam(r, "action")
	if action == "start" && errors.Is(err, pgx.ErrNoRows) {
		if actorType != "member" || (issue.CreatorType != "member" || issue.CreatorID != member.UserID) && (project.LeadType.String != "member" || project.LeadID != member.UserID) {
			writeError(w, 403, "only the creator or project lead can start a workflow")
			return
		}
		if input.ExpectedRevision != 0 || (issue.Status != "todo" && issue.Status != "backlog") {
			writeError(w, 409, "start from an unstarted task")
			return
		}
		row, err = service.InitializeOneSpecFlow(r.Context(), q, issue, input.Kind)
		if err != nil {
			writeError(w, 409, "enable OneSpec in the project and select a supported task kind")
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			writeError(w, 500, "failed to commit workflow")
			return
		}
		h.GetOneSpecFlow(w, r)
		return
	}
	if err != nil {
		writeError(w, 409, "workflow is not initialized")
		return
	}
	flow, err := parseOneSpecFlow(row)
	if err != nil {
		writeError(w, 500, "invalid workflow state")
		return
	}
	canConfirm := oneSpecCanConfirm(flow, project, actorType, actorID)
	if row.Revision != input.ExpectedRevision {
		if action == "decision" && oneSpecCanConfirm(flow, project, actorType, actorID) {
			latest := flow.Latest(input.Stage)
			for i := len(flow.Decisions) - 1; i >= 0; i-- {
				d := flow.Decisions[i]
				if d.Stage == input.Stage && latest != nil && d.Version == latest.Version {
					if d.Version == input.Version && d.ActorID == actorID && d.Approved == input.Approved && d.Reason == input.Reason {
						writeJSON(w, 200, oneSpecResponse{Flow: &flow, Revision: row.Revision, CanConfirm: true})
						return
					}
					break
				}
			}
		}
		writeError(w, 409, "workflow changed; reload before continuing")
		return
	}

	preparedTeam := false
	// A pending first stage may acquire its initial binding after runtime setup.
	if action == "run" && canConfirm && flow.Phase == "requirements" && flow.Status == "preparing" && len(flow.Artifacts) == 0 && flow.Team.Config.Roles["product"].AgentID == nil {
		active, e := q.ListActiveTasksByIssue(r.Context(), issue.ID)
		if e != nil {
			writeError(w, 500, "failed to inspect pending execution")
			return
		}
		if len(active) == 0 {
			setup, e := service.ResolveOneSpecTeam(r.Context(), q, issue.WorkspaceID, member.UserID, &project, true)
			if e != nil {
				writeError(w, 500, "failed to prepare OneSpec team")
				return
			}
			flow.Team = setup.Team
			preparedTeam = true
			runtime, e := service.OneSpecRuntime(r.Context(), q, issue.WorkspaceID, project.ID)
			if e != nil {
				writeError(w, 500, "failed to read runtime configuration")
				return
			}
			flow.RuntimeID = ""
			if runtime.Valid {
				flow.RuntimeID = uuidToString(runtime)
			}
		}
	}
	role := flow.Team.Config.Roles[workbench.OneSpecStageRole(flow.Phase)]
	coordinator := flow.Team.Config.Roles["sched"]
	allowedAgent := actorType == "agent" && role.AgentID != nil && *role.AgentID == actorID
	if flow.Phase == "requirements" && issue.AssigneeType.String == "agent" && uuidToString(issue.AssigneeID) == actorID && actorType == "agent" {
		allowedAgent = true
	}
	isCoordinator := actorType == "agent" && coordinator.AgentID != nil && *coordinator.AgentID == actorID
	criteria, err := workbench.ParseRequirements(issue.AcceptanceCriteria)
	if err != nil {
		writeError(w, 409, err.Error())
		return
	}
	if action != "reopen" && (flow.Phase == "archive" || flow.Phase == "archiving" || flow.Phase == "acceptance") {
		if tested := flow.Latest("testing"); tested != nil && tested.DeliveryDigest != "" && tested.DeliveryDigest != workbench.OneSpecDeliveryDigest(criteria) {
			writeError(w, 409, "delivery changed; reopen testing and verify the new delivery")
			return
		}
	}
	if action == "run" {
		pause, e := workbench.ParseExecutionPause(issue.Properties)
		if e != nil || pause != nil {
			writeError(w, 409, "task is paused; resume before starting execution")
			return
		}
		if !canConfirm {
			writeError(w, 403, "only the task creator or project lead can start execution")
			return
		}
		if flow.Status != "preparing" || flow.Phase == "acceptance" || !role.Enabled || role.AgentID == nil {
			writeError(w, 409, "the current stage is not executable or has no agent")
			return
		}
		agentID, valid := parseUUIDOrBadRequest(w, *role.AgentID, "agent_id")
		if !valid {
			return
		}
		agent, e := q.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{ID: agentID, WorkspaceID: issue.WorkspaceID})
		if e != nil || !h.canInvokeAgent(r.Context(), agent, actorType, actorID, uuidToString(member.UserID), uuidToString(issue.WorkspaceID)) {
			writeError(w, 403, "stage agent is inaccessible")
			return
		}
		stages := workbench.OneSpecStages(flow.Kind)
		if len(stages) > 1 && flow.Phase == stages[1] && len(flow.Decisions) > 0 {
			decision := flow.Decisions[len(flow.Decisions)-1]
			confirmedAt, parseErr := time.Parse(time.RFC3339Nano, decision.CreatedAt)
			if decision.Stage == "requirements" && decision.Approved && parseErr == nil {
				active, readErr := q.ListActiveTasksByIssue(r.Context(), issue.ID)
				if readErr != nil {
					writeError(w, 500, "failed to read current execution")
					return
				}
				for _, task := range active {
					if task.AgentID == agentID && !task.CreatedAt.Time.Before(confirmedAt) {
						writeJSON(w, 200, oneSpecResponse{Flow: &flow, Revision: row.Revision, CanConfirm: canConfirm})
						return
					}
				}
			}
		}
		if preparedTeam {
			criteria.Team = &workbench.ProjectTeamSnapshot{ProjectID: uuidToString(issue.ProjectID), ProjectRoleConfiguration: flow.Team}
			issue.AcceptanceCriteria, _ = json.Marshal(criteria)
		}
		if preparedTeam || issue.AssigneeID != agentID || issue.AssigneeType.String != "agent" {
			issue, e = q.UpdateIssue(r.Context(), db.UpdateIssueParams{ID: issue.ID, AcceptanceCriteria: issue.AcceptanceCriteria, ExpectedRevision: pgtype.Int8{Int64: issue.Revision, Valid: true}, AssigneeType: strToText("agent"), AssigneeID: agentID, ProjectID: issue.ProjectID, ParentIssueID: issue.ParentIssueID, StartDate: issue.StartDate, DueDate: issue.DueDate, Stage: issue.Stage, DuplicateOfIssueID: issue.DuplicateOfIssueID})
			if e != nil {
				writeError(w, 409, "failed to assign stage agent")
				return
			}
		}
		writer := &service.TaskService{Queries: q, Bus: events.New(), FeatureFlags: h.TaskService.FeatureFlags, Composio: h.TaskService.Composio}
		if preparedTeam {
			raw, _ := json.Marshal(flow)
			if _, e = q.UpdateOneSpecFlow(r.Context(), db.UpdateOneSpecFlowParams{State: raw, WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, ExpectedRevision: row.Revision}); e != nil {
				writeError(w, 409, "workflow changed; reload")
				return
			}
		}
		task, e := writer.EnqueueTaskForIssueByActor(r.Context(), issue, member.UserID)
		if e != nil {
			writeError(w, 409, "could not start stage execution: "+e.Error())
			return
		}
		if e = tx.Commit(r.Context()); e != nil {
			writeError(w, 500, "failed to commit stage execution")
			return
		}
		h.TaskService.NotifyTaskEnqueued(r.Context(), task)
		h.GetOneSpecFlow(w, r)
		return
	}
	switch action {
	case "refresh-configuration":
		if !canConfirm {
			writeError(w, 403, "only the creator or project lead can refresh configuration")
			return
		}
		if flow.Phase != "requirements" || len(flow.Artifacts) != 0 || (issue.Status != "todo" && issue.Status != "backlog") {
			writeError(w, 409, "refresh configuration before starting the workflow")
			return
		}
		active, e := q.WorkbenchPlanHasOtherActiveRuns(r.Context(), db.WorkbenchPlanHasOtherActiveRunsParams{ParentID: issue.ID, WorkspaceID: issue.WorkspaceID})
		if e != nil || active {
			writeError(w, 409, "stop active execution before refreshing configuration")
			return
		}
		updated, e := service.ResolveOneSpecFlow(r.Context(), q, issue, flow.Kind)
		if e != nil {
			writeError(w, 409, "enable and configure OneSpec in the project first")
			return
		}
		flow.RuntimeID, flow.OverviewRepositoryID, flow.Team = updated.RuntimeID, updated.OverviewRepositoryID, updated.Team
		flow.Generation++
		criteria.Team = &workbench.ProjectTeamSnapshot{ProjectID: uuidToString(issue.ProjectID), ProjectRoleConfiguration: flow.Team}
		issue.AcceptanceCriteria, _ = json.Marshal(criteria)
	case "release-draft":
		if flow.Kind != "release" || flow.Phase != "release_preparation" || (!canConfirm && !allowedAgent) {
			writeError(w, 403, "prepare release drafts in the assigned release stage")
			return
		}
		if flow.ReleaseID == "" {
			order, e := createOneSpecReleaseDraft(r.Context(), q, issue, member.UserID, input.Release)
			if e != nil {
				writeError(w, 400, e.Error())
				return
			}
			flow.ReleaseID = uuidToString(order.ID)
		}
	case "kind":
		if !canConfirm {
			writeError(w, 403, "only the creator or project lead can change workflow kind")
			return
		}
		if flow.Phase != "requirements" || len(flow.Artifacts) > 0 || len(workbench.OneSpecStages(input.Kind)) == 0 {
			writeError(w, 409, "choose the task kind before submitting requirements")
			return
		}
		flow.Kind = input.Kind
	case "artifact":
		input.Artifact.SourceTaskID = ""
		if input.Artifact.Stage == "requirements" && criteria.Phase != "" && !workbench.RequirementsReady(criteria) {
			writeError(w, 409, "submit the current structured requirements before requesting confirmation")
			return
		}
		if input.Artifact.Document != nil {
			task, valid := h.taskFromRequestHeader(r)
			if !valid || actorType != "agent" || task.IssueID != issue.ID || uuidToString(task.AgentID) != actorID || task.Status != "running" {
				writeError(w, 403, "requirement documents must be submitted by the active task agent")
				return
			}
			input.Artifact.SourceTaskID = uuidToString(task.ID)
			previous := flow.Latest("requirements")
			if previous == nil || !workbench.SameRequirementDocuments(previous.Document, input.Artifact.Document) {
				var previousDocument *workbench.RequirementsDocument
				if previous != nil {
					previousDocument = previous.Document
				}
				for i := range criteria.Items {
					if workbench.RequirementItemDocumentChanged(previousDocument, input.Artifact.Document, criteria.Items[i].Section) {
						criteria.Items[i].Checked = nil
					}
				}
				criteria.DraftRevision++
				criteria.ReadyRevision = criteria.DraftRevision
				issue.AcceptanceCriteria, _ = json.Marshal(criteria)
			}
		}
		if input.Artifact.Stage == "requirements" && criteria.Phase != "" {
			if !workbench.RequirementsReady(criteria) {
				writeError(w, 409, "submit the current structured requirements before requesting confirmation")
				return
			}
			input.Artifact.DraftRevision = criteria.DraftRevision
		}
		if !canConfirm && !allowedAgent {
			writeError(w, 403, "only the stage owner or confirmer can submit artifacts")
			return
		}
		for _, ref := range input.Artifact.Refs {
			id, ok := parseUUIDOrBadRequest(w, ref.RepositoryID, "repository_id")
			if !ok {
				return
			}
			repo, e := q.GetProjectResourceInWorkspaceForUpdate(r.Context(), db.GetProjectResourceInWorkspaceForUpdateParams{ID: id, WorkspaceID: issue.WorkspaceID})
			if e != nil || repo.ProjectID != issue.ProjectID || (repo.ResourceType != "git_repo" && repo.ResourceType != "github_repo" && repo.ResourceType != "local_directory") {
				writeError(w, 400, "artifact repository must belong to this project")
				return
			}
		}
		if err := flow.ValidatePriorVersions(criteria.Version); err != nil {
			writeError(w, 409, err.Error())
			return
		}
		if flow.Kind == "release" && (flow.Phase == "release_preparation" || flow.Phase == "deployment") {
			if flow.ReleaseID == "" {
				writeError(w, 409, "create a release draft first")
				return
			}
			order, e := q.GetWorkbenchRelease(r.Context(), db.GetWorkbenchReleaseParams{ID: parseUUID(flow.ReleaseID), WorkspaceID: issue.WorkspaceID})
			if e != nil || order.ProjectID != issue.ProjectID {
				writeError(w, 409, "linked release is unavailable")
				return
			}
			if flow.Phase == "deployment" && order.Status != "live" {
				writeError(w, 409, "complete and verify deployment in the release center before submitting deployment evidence")
				return
			}
			input.Artifact.ReleaseRevision = order.Revision
		}
		input.Artifact.RequirementVersion = criteria.Version
		input.Artifact.DeliveryDigest = workbench.OneSpecDeliveryDigest(criteria)
		if input.Artifact.Stage == "plan" {
			resources, e := q.ListProjectResourcesInWorkspace(r.Context(), db.ListProjectResourcesInWorkspaceParams{ProjectID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
			if e != nil {
				writeError(w, 500, "failed to load repositories")
				return
			}
			repos := map[string]bool{}
			for _, resource := range resources {
				repos[uuidToString(resource.ID)] = true
			}
			existing, ok := h.executionPlanDependencies(w, r, q, issue, input.Artifact.Tasks)
			if !ok {
				return
			}
			if e = workbench.ValidateExecutionPlan(input.Artifact.Tasks, criteria, repos, existing); e != nil {
				writeError(w, 400, e.Error())
				return
			}
		}
		err = flow.Submit(input.Artifact, actorID, time.Now())
	case "decision":
		if input.Stage == "requirements" && input.Approved {
			var pending bool
			if e := tx.QueryRow(r.Context(), "SELECT EXISTS (SELECT 1 FROM task_user_input WHERE issue_id=$1 AND workspace_id=$2 AND status='pending')", issue.ID, issue.WorkspaceID).Scan(&pending); e != nil || pending {
				writeError(w, 409, "answer the outstanding questions before confirming requirements")
				return
			}
		}
		if !canConfirm {
			writeError(w, 403, "only the task creator or project lead can confirm")
			return
		}
		if a := flow.Latest(input.Stage); a != nil && a.RequirementVersion != criteria.Version {
			writeError(w, 409, "requirements changed; reopen and regenerate the affected stage")
			return
		}
		if input.Stage == "deployment" && flow.ReleaseID != "" {
			order, e := q.GetWorkbenchRelease(r.Context(), db.GetWorkbenchReleaseParams{ID: parseUUID(flow.ReleaseID), WorkspaceID: issue.WorkspaceID})
			artifact := flow.Latest("deployment")
			if e != nil || artifact == nil || order.Status != "live" || order.Revision != artifact.ReleaseRevision {
				writeError(w, 409, "deployment changed; submit evidence for the current release")
				return
			}
		}
		err = flow.Decide(input.Stage, input.Version, input.Approved, input.Reason, actorID, time.Now())
		if err == nil && input.Approved && input.Stage == "requirements" && criteria.Gate.Spec == nil {
			if criteria.Phase != "" && (!workbench.RequirementsReady(criteria) || flow.Latest("requirements").DraftRevision != criteria.DraftRevision) {
				writeError(w, 409, "requirements changed; review the current discussion and draft")
				return
			}
			checked, _ := json.Marshal(criteria)
			updated, e := workbench.EditRequirements(issue.AcceptanceCriteria, checked, true, actorID, actorType, time.Now())
			if e != nil {
				writeError(w, 409, e.Error())
				return
			}
			criteria, e = workbench.ParseRequirements(updated)
			if e != nil {
				writeError(w, 409, e.Error())
				return
			}
			issue.AcceptanceCriteria = updated
		}
	case "reopen":
		if !canConfirm {
			writeError(w, 403, "only the task creator or project lead can reopen")
			return
		}
		active, e := q.WorkbenchPlanHasOtherActiveRuns(r.Context(), db.WorkbenchPlanHasOtherActiveRunsParams{ParentID: issue.ID, WorkspaceID: issue.WorkspaceID})
		if e != nil || active {
			writeError(w, 409, "stop active execution before reopening")
			return
		}
		err = flow.Reopen(input.Stage)
		if err == nil {
			criteria.Gate.Accept = nil
			criteria.Delivery = nil
			criteria.Review = nil
			if criteria.Plan != nil && (input.Stage == "requirements" || input.Stage == "design" || input.Stage == "plan") {
				criteria.Plan = nil
				criteria.NeedsResplit = false
			}
			if input.Stage == "requirements" {
				criteria.Phase = "draft"
				criteria.ReadyRevision = 0
				criteria.Version++
				criteria.Gate.Spec = nil
				for i := range criteria.Items {
					criteria.Items[i].Checked = nil
				}
			}
			if len(issue.AcceptanceCriteria) > 0 && string(issue.AcceptanceCriteria) != "null" {
				issue.AcceptanceCriteria, _ = json.Marshal(criteria)
			}
		}
	case "advance":
		if !canConfirm && !allowedAgent && !isCoordinator {
			writeError(w, 403, "only the stage owner, coordinator or confirmer can continue")
			return
		}
		if a := flow.Latest(flow.Phase); a == nil || a.RequirementVersion != criteria.Version {
			writeError(w, 409, "submit evidence for the current requirement version")
			return
		}
		if flow.Phase == "requirements" && criteria.Gate.Spec == nil && (flow.Kind == "feature" || flow.Kind == "change" || flow.Kind == "bugfix") {
			writeError(w, 409, "confirm the structured requirements before continuing")
			return
		}
		if flow.Phase == "plan" && (criteria.Plan == nil || criteria.Plan.Version != criteria.Version) {
			writeError(w, 409, "apply the confirmed execution plan before continuing")
			return
		}
		if flow.Phase == "implementation" || flow.Phase == "testing" {
			if criteria.Plan != nil {
				for _, id := range criteria.Plan.Issues {
					child, e := q.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: parseUUID(id), WorkspaceID: issue.WorkspaceID})
					if e != nil {
						writeError(w, 409, "a planned subtask is missing")
						return
					}
					var handoff workbench.Handoff
					if json.Unmarshal(workbench.HandoffJSON(child.Properties), &handoff) != nil {
						writeError(w, 409, "subtask handoff is unavailable")
						return
					}
					if flow.Phase == "implementation" && handoff.Role != "dev" {
						continue
					}
					evidence, exists := criteria.Delivery[id]
					if !exists || child.Status != "done" || evidence.IssueRevision != child.Revision || evidence.Version != criteria.Version || !evidence.Passed || evidence.DescriptionHash != workbench.DeliveryDescriptionHash(issue.Description.String) {
						writeError(w, 409, "complete current subtask verification evidence before continuing")
						return
					}
				}
			}
		}
		previous := flow.Latest(flow.Phase)
		if e := flow.ValidatePriorVersions(criteria.Version); e != nil {
			writeError(w, 409, e.Error())
			return
		}
		err = flow.Advance()
		if err == nil && flow.Phase == "acceptance" && previous != nil {
			evidence := *previous
			evidence.Stage = "acceptance"
			err = flow.Submit(evidence, actorID, time.Now())
		}
	case "git":
		git := flow.Team.Config.Roles["git"]
		if !canConfirm && !(actorType == "agent" && git.AgentID != nil && *git.AgentID == actorID) {
			writeError(w, 403, "only Git management or the confirmer can record operations")
			return
		}
		op := input.Operation
		if strings.TrimSpace(op.Key) == "" || len(op.Key) > 100 || strings.TrimSpace(op.Summary) == "" {
			writeError(w, 400, "operation key and summary are required")
			return
		}
		repoID, valid := parseUUIDOrBadRequest(w, op.RepositoryID, "repository_id")
		if !valid {
			return
		}
		repo, e := q.GetProjectResourceInWorkspaceForUpdate(r.Context(), db.GetProjectResourceInWorkspaceForUpdateParams{ID: repoID, WorkspaceID: issue.WorkspaceID})
		if e != nil || repo.ProjectID != issue.ProjectID {
			writeError(w, 400, "repository is not in this project")
			return
		}
		err = flow.RecordGit(op, uuidToString(issue.ID))
	default:
		writeError(w, 400, "unknown workflow action")
		return
	}
	if err != nil {
		writeError(w, 409, err.Error())
		return
	}
	raw, _ := json.Marshal(flow)
	row, err = q.UpdateOneSpecFlow(r.Context(), db.UpdateOneSpecFlowParams{State: raw, WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, ExpectedRevision: row.Revision})
	if err != nil {
		writeError(w, 409, "workflow changed; reload")
		return
	}
	if action == "advance" || action == "reopen" || action == "decision" || action == "refresh-configuration" || (action == "artifact" && input.Artifact.Document != nil) {
		nextRole := flow.Team.Config.Roles[workbench.OneSpecStageRole(flow.Phase)]
		assigneeType, assigneeID := issue.AssigneeType, issue.AssigneeID
		if nextRole.Enabled && nextRole.AgentID != nil {
			assigneeType = strToText("agent")
			assigneeID, ok = parseUUIDOrBadRequest(w, *nextRole.AgentID, "agent_id")
			if !ok {
				return
			}
		}
		status := issue.Status
		if action == "advance" || action == "reopen" {
			status = "todo"
		}
		if flow.Phase == "implementation" || flow.Phase == "testing" {
			status = "in_progress"
		}
		if flow.Phase == "acceptance" {
			status = "in_review"
		}
		if action == "decision" || action == "artifact" {
			assigneeType, assigneeID = issue.AssigneeType, issue.AssigneeID
		}
		if flow.Status == "completed" {
			status = "done"
			criteria.Gate.Accept = &workbench.GateCheck{PassedAt: time.Now().UTC().Format(time.RFC3339Nano), PassedBy: actorID}
			if len(issue.AcceptanceCriteria) > 0 && string(issue.AcceptanceCriteria) != "null" {
				issue.AcceptanceCriteria, _ = json.Marshal(criteria)
			}
		}
		issue, err = q.UpdateIssue(r.Context(), db.UpdateIssueParams{ID: issue.ID, AcceptanceCriteria: issue.AcceptanceCriteria, ExpectedRevision: pgtype.Int8{Int64: issue.Revision, Valid: true}, Status: strToText(status), AssigneeType: assigneeType, AssigneeID: assigneeID, ProjectID: issue.ProjectID, ParentIssueID: issue.ParentIssueID, StartDate: issue.StartDate, DueDate: issue.DueDate, Stage: issue.Stage, DuplicateOfIssueID: issue.DuplicateOfIssueID})
		if err != nil {
			slog.Warn("OneSpec task transition rejected", "error", err, "issue_id", uuidToString(issue.ID))
			writeError(w, 409, "task could not advance; resolve its execution or acceptance gate")
			return
		}
	}
	if err = writeWorkbenchAudit(r.Context(), q, issue.WorkspaceID, member.UserID, "onespec_"+action, map[string]any{"issue_id": uuidToString(issue.ID), "revision": row.Revision, "actor_type": actorType, "actor_id": actorID}); err != nil {
		writeError(w, 500, "failed to audit workflow")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit workflow")
		return
	}
	h.publishIssueSnapshots(r.Context(), []db.Issue{issue}, actorType, actorID)
	writeJSON(w, 200, oneSpecResponse{Flow: &flow, Revision: row.Revision, CanConfirm: canConfirm})
}
