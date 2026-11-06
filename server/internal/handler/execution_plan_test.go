package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestExecutionPlanIsAtomicIdempotentAndEnforcesDependencies(t *testing.T) {
	ctx := context.Background()
	project := dbfx.Insert(t, "project", testutil.Cols{"workspace_id": testWorkspaceID, "title": "Execution plan"})
	repo := dbfx.Insert(t, "project_resource", testutil.Cols{"project_id": project, "workspace_id": testWorkspaceID, "resource_type": "github_repo", "resource_ref": `{"url":"https://example.com/plan.git"}`})
	dbfx.Insert(t, "project_resource", testutil.Cols{"project_id": project, "workspace_id": testWorkspaceID, "resource_type": "github_repo", "resource_ref": `{"url":"https://example.com/other.git"}`})
	parent := dbfx.Issue(t, "Plan root", testutil.Cols{"project_id": project, "assignee_type": "member", "assignee_id": testUserID})
	if _, err := testPool.Exec(ctx, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1)) WHERE id=$1`, testWorkspaceID); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM issue WHERE parent_issue_id=$1`, parent)
	criteria := workbench.Requirements{V: 1, Version: 1, NextItemNo: 2,
		Items: []workbench.Item{{ID: "A-01", Text: "Export report", Checked: &workbench.Check{By: testUserID, At: "confirmed"}}},
		Gate:  workbench.Gates{Spec: &workbench.GateCheck{PassedAt: "confirmed", PassedBy: testUserID}},
		Team:  &workbench.ProjectTeamSnapshot{ProjectID: project, ProjectRoleConfiguration: workbench.ProjectRoleConfiguration{Config: workbench.DefaultProjectRoles(), Revision: 1}},
	}
	raw, _ := json.Marshal(criteria)
	if _, err := testPool.Exec(ctx, `UPDATE issue SET acceptance_criteria=$1 WHERE id=$2`, raw, parent); err != nil {
		t.Fatal(err)
	}
	current, err := testHandler.Queries.GetIssue(ctx, parseUUID(parent))
	if err != nil {
		t.Fatal(err)
	}
	tasks := []workbench.PlanTask{
		{Key: "T-01", Title: "Export", Role: "dev", RepoID: repo, Covers: []string{"A-01"}, Expected: []string{"diff"}, SerialGroup: "api"},
		{Key: "T-02", Title: "Verify", Role: "test", RepoID: repo, Covers: []string{"A-01"}, DependsOn: []string{"T-01"}, Expected: []string{"selftest"}},
		{Key: "T-03", Title: "Document", Role: "dev", RepoID: repo, Covers: []string{"A-01"}, Expected: []string{"diff"}, SerialGroup: "api"},
	}
	call := func(key string, version int, revision int64, status int) IssueResponse {
		t.Helper()
		req := testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/execution-plan", map[string]any{"expected_revision": revision, "version": version, "idempotency_key": key, "tasks": tasks}), "id", parent)
		result := testutil.Call(t, testHandler.SaveExecutionPlan, req).Want(status)
		var response IssueResponse
		if status == http.StatusOK {
			result.JSON(&response)
		}
		return response
	}
	tasks[0].DependsOn = []string{"T-02"}
	call("plan-1", 1, current.Revision, http.StatusBadRequest)
	var children int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, parent).Scan(&children)
	if children != 0 {
		t.Fatal("invalid plan created partial subtasks")
	}
	tasks[0].DependsOn = nil
	saved := call("plan-1", 1, current.Revision, http.StatusOK)
	parsed, _ := workbench.ParseRequirements(saved.AcceptanceCriteria)
	if parsed.Plan == nil || len(parsed.Plan.Issues) != 3 || len(parsed.Items[0].Covers) != 3 {
		t.Fatal("plan coverage was not saved")
	}
	retry := call("plan-1", 1, current.Revision, http.StatusOK)
	if retry.Revision != saved.Revision {
		t.Fatal("idempotent retry rewrote the plan")
	}
	tasks[0].Title = "Changed payload"
	call("plan-1", 1, current.Revision, http.StatusConflict)
	first, second := parsed.Plan.Issues[0], parsed.Plan.Issues[1]
	third := parsed.Plan.Issues[2]
	firstIssue, err := testHandler.Queries.GetIssue(ctx, parseUUID(first))
	if err != nil {
		t.Fatal(err)
	}
	var detail IssueResponse
	testutil.Call(t, testHandler.GetIssue, testutil.WithURLParams(newRequest("GET", "/api/issues/"+first, nil), "id", first)).Want(http.StatusOK).JSON(&detail)
	var responseHandoff workbench.Handoff
	if json.Unmarshal(detail.WorkbenchHandoff, &responseHandoff) != nil || responseHandoff.RepoID != repo || responseHandoff.V != 2 {
		t.Fatal("task detail lost the typed handoff projection")
	}
	claim, err := testHandler.resolveIssueClaimProjectContext(ctx, firstIssue, db.AgentRuntime{})
	if err != nil || len(claim.Resources) != 1 || claim.Resources[0].ID != repo || len(claim.Repos) != 1 {
		t.Fatalf("claim did not restrict execution to the selected repository: %+v, %v", claim, err)
	}
	firstIssue.Properties = []byte(`{"workbench_handoff":{"v":2,"role":"product","repo_id":""}}`)
	claim, err = testHandler.resolveIssueClaimProjectContext(ctx, firstIssue, db.AgentRuntime{})
	if err != nil || len(claim.Resources) != 0 || len(claim.Repos) != 0 {
		t.Fatal("clarification inherited unrelated repository context")
	}
	localRepo := dbfx.Insert(t, "project_resource", testutil.Cols{"project_id": project, "workspace_id": testWorkspaceID, "resource_type": "local_directory", "resource_ref": `{"daemon_id":"selected-machine","local_path":"/test/repo","execution_mode":"in_place"}`})
	firstIssue.Properties, _ = json.Marshal(map[string]any{"workbench_handoff": workbench.Handoff{V: 2, Role: "dev", RepoID: localRepo}})
	if _, err = testHandler.resolveIssueClaimProjectContext(ctx, firstIssue, db.AgentRuntime{DaemonID: pgtype.Text{String: "other-machine", Valid: true}}); err == nil {
		t.Fatal("selected local repository was sent to another machine")
	}
	claim, err = testHandler.resolveIssueClaimProjectContext(ctx, firstIssue, db.AgentRuntime{DaemonID: pgtype.Text{String: "selected-machine", Valid: true}})
	if err != nil || len(claim.Resources) != 1 || claim.Resources[0].ID != localRepo {
		t.Fatalf("selected local repository unavailable on its own machine: %v", err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE issue SET status='in_progress' WHERE id=$1`, first); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE issue SET status='in_progress' WHERE id=$1`, third); err == nil {
		t.Fatal("serial group allowed concurrent execution")
	}
	if _, err := testPool.Exec(ctx, `UPDATE issue SET status='in_progress' WHERE id=$1`, second); err == nil {
		t.Fatal("dependency bypassed through issue status")
	}
	agent := dbfx.Agent(t, "Plan executor", testRuntimeID)
	run := dbfx.Task(t, agent, testutil.Cols{"issue_id": second, "runtime_id": testRuntimeID, "status": "cancelled"})
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status='running' WHERE id=$1`, run); err == nil {
		t.Fatal("dependency bypassed through run state")
	}
	if _, err := testPool.Exec(ctx, `UPDATE issue SET status='done' WHERE id=$1`, first); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE issue SET status='in_progress' WHERE id=$1`, second); err != nil {
		t.Fatal(err)
	}
	parsed.Version, parsed.NeedsResplit = 2, true
	raw, _ = json.Marshal(parsed)
	if _, err := testPool.Exec(ctx, `UPDATE issue SET status='backlog',acceptance_criteria=$1 WHERE id=$2`, raw, parent); err != nil {
		t.Fatal(err)
	}
	current, _ = testHandler.Queries.GetIssue(ctx, parseUUID(parent))
	replanned := call("plan-2", 2, current.Revision, http.StatusOK)
	parsed, _ = workbench.ParseRequirements(replanned.AcceptanceCriteria)
	if parsed.NeedsResplit || parsed.Plan.Version != 2 || parsed.Plan.Issues[0] == first {
		t.Fatal("new version did not replace its plan")
	}
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status='running' WHERE id=$1`, run); err == nil {
		t.Fatal("retired plan resumed")
	}
	if _, err := testPool.Exec(ctx, `UPDATE issue SET status='in_progress' WHERE id=$1`, first); err == nil {
		t.Fatal("retired record was reopened")
	}
}
