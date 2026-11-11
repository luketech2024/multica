package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestConfirmedRequirementsDispatchSchedulerAndAcceptItsPlan(t *testing.T) {
	ctx := context.Background()
	project := dbfx.Project(t, "Structured workflow")
	repo := dbfx.Insert(t, "project_resource", testutil.Cols{"project_id": project, "workspace_id": testWorkspaceID, "resource_type": "github_repo", "resource_ref": `{"url":"https://example.test/workflow.git"}`})
	scheduler := dbfx.Agent(t, "Scheduler", testRuntimeID)
	developer := dbfx.Agent(t, "Developer", testRuntimeID)
	roles := workbench.DefaultProjectRoles()
	roles.Roles["sched"] = workbench.ProjectRole{Enabled: true, AgentID: &scheduler}
	roles.Roles["dev"] = workbench.ProjectRole{Enabled: true, AgentID: &developer}
	for _, role := range []string{"test", "sec", "ops"} {
		roles.Roles[role] = workbench.ProjectRole{Enabled: false}
	}
	criteria := workbench.Requirements{V: 1, Version: 1, NextItemNo: 2, Items: []workbench.Item{{ID: "A-01", Text: "Display contact email", Checked: &workbench.Check{At: "confirmed", By: testUserID}}}, Team: &workbench.ProjectTeamSnapshot{ProjectID: project, ProjectRoleConfiguration: workbench.ProjectRoleConfiguration{Config: roles, Revision: 1}}}
	raw, _ := json.Marshal(criteria)
	parent := dbfx.Issue(t, "Contact email", testutil.Cols{"project_id": project, "status": "todo", "assignee_type": "member", "assignee_id": testUserID, "acceptance_criteria": raw})
	dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id=$1 OR issue_id IN (SELECT id FROM issue WHERE parent_issue_id=$1)`, parent)
	dbfx.Cleanup(t, `DELETE FROM issue WHERE parent_issue_id=$1`, parent)
	dbfx.Exec(t, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1)) WHERE id=$1`, testWorkspaceID)
	current, _ := testHandler.Queries.GetIssue(ctx, parseUUID(parent))
	var confirmed IssueResponse
	testutil.Call(t, testHandler.UpdateIssue, testutil.WithURLParams(newRequest("PUT", "/api/issues/"+parent, map[string]any{"expected_revision": current.Revision, "confirm_requirements": true}), "id", parent)).Want(http.StatusOK).JSON(&confirmed)
	current, _ = testHandler.Queries.GetIssue(ctx, parseUUID(parent))
	if current.AssigneeType.String != "agent" || uuidToString(current.AssigneeID) != scheduler {
		t.Fatal("confirmation did not hand the parent to its scheduler")
	}
	var run string
	if err := testPool.QueryRow(ctx, `SELECT id::text FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 ORDER BY created_at DESC LIMIT 1`, parent, scheduler).Scan(&run); err != nil {
		t.Fatalf("confirmation did not enqueue scheduler: %v", err)
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='running',runtime_id=$2 WHERE id=$1`, run, testRuntimeID)
	plan := map[string]any{"expected_revision": current.Revision, "version": 1, "idempotency_key": "contact-plan", "tasks": []workbench.PlanTask{{Key: "dev", Title: "Display email", Role: "dev", RepoID: repo, Covers: []string{"A-01"}, Expected: []string{"report"}}}}
	request := func() *http.Request {
		r := testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/execution-plan", plan), "id", parent)
		r.Header.Set("X-Actor-Source", "task_token")
		r.Header.Set("X-Agent-ID", scheduler)
		r.Header.Set("X-Task-ID", run)
		return r
	}
	wrong := request()
	wrong.Header.Set("X-Agent-ID", developer)
	testutil.Call(t, testHandler.SaveExecutionPlan, wrong).Want(http.StatusForbidden)
	var saved IssueResponse
	testutil.Call(t, testHandler.SaveExecutionPlan, request()).Want(http.StatusOK).JSON(&saved)
	parsed, _ := workbench.ParseRequirements(saved.AcceptanceCriteria)
	if len(parsed.Plan.Issues) != 1 {
		t.Fatal("scheduler plan was not persisted")
	}
	testutil.Call(t, testHandler.SaveExecutionPlan, request()).Want(http.StatusOK)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, run)
	child := parsed.Plan.Issues[0]
	dbfx.Exec(t, `UPDATE issue SET status='in_progress' WHERE id IN ($1,$2)`, parent, child)
	childIssue, _ := testHandler.Queries.GetIssue(ctx, parseUUID(child))
	devRun := dbfx.Task(t, developer, testutil.Cols{"issue_id": child, "runtime_id": testRuntimeID, "status": "running"})
	current, _ = testHandler.Queries.GetIssue(ctx, parseUUID(parent))
	evidence := workbench.DeliveryEvidence{IssueID: child, IssueRevision: childIssue.Revision, Commit: strings.Repeat("a", 40), Passed: true, Summary: "Email verified", Reports: []workbench.DeliveryReport{{Name: "report", URL: "https://example.test/report"}}, Changes: &workbench.DeliveryChanges{Files: 1, Additions: 1}, PreviewURL: "https://preview.example.test/"}
	delivery := testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/delivery-evidence", map[string]any{"expected_revision": current.Revision, "evidence": evidence}), "id", parent)
	delivery.Header.Set("X-Actor-Source", "task_token")
	delivery.Header.Set("X-Agent-ID", developer)
	delivery.Header.Set("X-Task-ID", devRun)
	var delivered IssueResponse
	testutil.Call(t, testHandler.SaveDeliveryEvidence, delivery).Want(http.StatusOK).JSON(&delivered)
	childIssue, _ = testHandler.Queries.GetIssue(ctx, parseUUID(child))
	parsed, _ = workbench.ParseRequirements(delivered.AcceptanceCriteria)
	if childIssue.Status != "done" || parsed.Delivery[child].IssueRevision != childIssue.Revision {
		t.Fatal("delivery did not atomically finish the child with current evidence")
	}
	var review deliveryReviewState
	testutil.Call(t, testHandler.ReviewIssueDelivery, testutil.WithURLParams(newRequest("GET", "/api/issues/"+parent+"/delivery-review", nil), "id", parent)).Want(http.StatusOK).JSON(&review)
	if len(review.Blockers) != 1 || review.Blockers[0].Code != "running" {
		t.Fatalf("acceptance must wait for the reporting run to settle: %+v", review)
	}
}
