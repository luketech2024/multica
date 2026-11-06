package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestWorkbenchRoleRuntimePinsCreationAndRunTransitions(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	project := dbfx.Project(t, "Pinned role runtime")
	original := dbfx.Runtime(t, "Original role runtime")
	replacement := dbfx.Runtime(t, "Replacement runtime")
	agent := dbfx.Agent(t, "Pinned product agent", original)
	roles := workbench.DefaultProjectRoles()
	roles.Roles["product"] = workbench.ProjectRole{Enabled: true, AgentID: &agent, RuntimeID: &replacement}
	save := func(code int) {
		t.Helper()
		testutil.Call(t, testHandler.SaveProjectWorkbenchRoles, testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project+"/workbench-roles", map[string]any{"expected_revision": 1, "config": roles}), "id", project)).Want(code)
	}
	save(http.StatusBadRequest)
	roles.Roles["product"] = workbench.ProjectRole{Enabled: true, AgentID: &agent, RuntimeID: &original}
	save(http.StatusOK)
	body := map[string]any{"title": "Pinned clarification", "project_id": project, "status": "backlog", "acceptance_criteria": map[string]any{"v": 1, "items": []any{}}}
	created := createIssueWithPropertyBody(t, body)
	var criteria workbench.Requirements
	if err := json.Unmarshal(created.AcceptanceCriteria, &criteria); err != nil {
		t.Fatal(err)
	}
	if criteria.Team == nil || *criteria.Team.Config.Roles["product"].RuntimeID != original {
		t.Fatal("runtime pin missing from task snapshot")
	}
	dbfx.Exec(t, `UPDATE agent SET runtime_id=$1 WHERE id=$2`, replacement, agent)
	body["title"] = "Reject silently moved executor"
	testutil.Call(t, testHandler.CreateIssue, newRequest("POST", "/api/issues", body)).Want(http.StatusBadRequest)
	run := dbfx.Task(t, agent, testutil.Cols{"issue_id": created.ID, "runtime_id": replacement, "status": "failed"})
	_, err := testPool.Exec(context.Background(), `UPDATE agent_task_queue SET status='queued' WHERE id=$1`, run)
	var constraint *pgconn.PgError
	if !errors.As(err, &constraint) || constraint.ConstraintName != "workbench_execution_gate" {
		t.Fatalf("runtime drift was not rejected: %v", err)
	}
	dbfx.Exec(t, `UPDATE agent SET runtime_id=$1 WHERE id=$2`, original, agent)
	dbfx.Exec(t, `UPDATE agent_task_queue SET runtime_id=$1,status='queued' WHERE id=$2`, original, run)
	// Terminal reporting remains possible even after a runtime binding changes.
	dbfx.Exec(t, `UPDATE agent_task_queue SET runtime_id=$1,status='failed' WHERE id=$2`, replacement, run)

	handoff, _ := json.Marshal(map[string]any{"workbench_handoff": workbench.Handoff{
		V: 2, RequirementVersion: 1, Role: "test", Covers: []string{"A-01"}, Expected: []string{"report"}, DependsOn: []string{},
	}})
	child := dbfx.Issue(t, "Pinned test executor", testutil.Cols{"project_id": project, "parent_issue_id": created.ID, "properties": string(handoff), "assignee_type": "agent", "assignee_id": agent})
	criteria.Team.Config.Roles["test"] = workbench.ProjectRole{Enabled: true, AgentID: &agent, RuntimeID: &original}
	criteria.Gate.Spec = &workbench.GateCheck{PassedAt: "confirmed", PassedBy: testUserID}
	criteria.Items = []workbench.Item{{ID: "A-01", Text: "Verify export", Covers: []string{child}, Checked: &workbench.Check{At: "confirmed", By: testUserID}}}
	criteria.Plan = &workbench.ExecutionPlan{Version: 1, Issues: []string{child}}
	raw, _ := json.Marshal(criteria)
	dbfx.Exec(t, `UPDATE issue SET status='todo',acceptance_criteria=$1 WHERE id=$2`, raw, created.ID)
	childRun := dbfx.Task(t, agent, testutil.Cols{"issue_id": child, "runtime_id": replacement, "status": "failed"})
	_, err = testPool.Exec(context.Background(), `UPDATE agent_task_queue SET status='queued' WHERE id=$1`, childRun)
	if !errors.As(err, &constraint) || constraint.ConstraintName != "workbench_execution_gate" {
		t.Fatalf("subtask runtime pin was ignored: %v", err)
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET runtime_id=$1,status='queued' WHERE id=$2`, original, childRun)
}
