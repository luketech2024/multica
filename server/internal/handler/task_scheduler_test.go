package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestTaskSelectedCoordinatorCanPlan(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "new confirmation"
		if legacy {
			name = "already confirmed"
		}
		t.Run(name, func(t *testing.T) {
			project := dbfx.Project(t, "Deployment workflow")
			agent := dbfx.Agent(t, "Selected coordinator", testRuntimeID)
			ops := dbfx.Agent(t, "Deployment operator", testRuntimeID)
			roles := workbench.DefaultProjectRoles()
			roles.Roles["ops"] = workbench.ProjectRole{Enabled: true, AgentID: &ops}
			criteria := workbench.Requirements{V: 1, Version: 1, NextItemNo: 2,
				Items: []workbench.Item{{ID: "A-01", Text: "Analyze deployment", Checked: &workbench.Check{At: "checked", By: testUserID}}},
				Team:  &workbench.ProjectTeamSnapshot{ProjectID: project, ProjectRoleConfiguration: workbench.ProjectRoleConfiguration{Config: roles, Revision: 1}}}
			if legacy {
				criteria.Gate.Spec = &workbench.GateCheck{PassedAt: "confirmed", PassedBy: testUserID}
			}
			raw, _ := json.Marshal(criteria)
			id := dbfx.Issue(t, "Deploy application", testutil.Cols{"project_id": project, "assignee_type": "agent", "assignee_id": agent, "acceptance_criteria": raw})
			dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id=$1 OR issue_id IN (SELECT id FROM issue WHERE parent_issue_id=$1)`, id)
			dbfx.Cleanup(t, `DELETE FROM issue WHERE parent_issue_id=$1`, id)
			dbfx.Exec(t, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1)) WHERE id=$1`, testWorkspaceID)
			current, _ := testHandler.Queries.GetIssue(t.Context(), parseUUID(id))
			if !legacy {
				testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/", map[string]any{"expected_revision": current.Revision, "confirm_requirements": true}), "id", id)).Want(200)
				current, _ = testHandler.Queries.GetIssue(t.Context(), parseUUID(id))
				confirmed, _ := workbench.ParseRequirements(current.AcceptanceCriteria)
				if confirmed.Team.Config.Roles["sched"].AgentID == nil || *confirmed.Team.Config.Roles["sched"].AgentID != agent {
					t.Fatal("confirmation did not bind the selected coordinator")
				}
				dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE issue_id=$1`, id)
			}
			run := dbfx.Task(t, agent, testutil.Cols{"issue_id": id, "runtime_id": testRuntimeID, "status": "running"})
			task := workbench.PlanTask{Key: "analyze", Title: "Analyze deployment", Role: "ops", Covers: []string{"A-01"}, Expected: []string{"report"}}
			call := func(role string, expected any, status int) *testutil.Response {
				t.Helper()
				task.Role = role
				data, _ := json.Marshal(task)
				var payload map[string]any
				_ = json.Unmarshal(data, &payload)
				payload["expected"] = expected
				r := withURLParam(newRequest("POST", "/", map[string]any{"expected_revision": current.Revision, "version": 1, "idempotency_key": "deploy", "tasks": []any{payload}}), "id", id)
				r.Header.Set("X-Actor-Source", "task_token")
				r.Header.Set("X-Agent-ID", agent)
				r.Header.Set("X-Task-ID", run)
				return testutil.Call(t, testHandler.SaveExecutionPlan, r).Want(status)
			}
			if !strings.Contains(call("ops", "report", 400).Body.String(), "tasks[].expected") {
				t.Fatal("type error did not identify expected")
			}
			if !strings.Contains(call("test", []string{"report"}, 409).Body.String(), "execution role test") {
				t.Fatal("missing executor not identified")
			}
			if dbfx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, id) != 0 {
				t.Fatal("failed preflight created children")
			}
			var saved IssueResponse
			call("ops", []string{"report"}, http.StatusOK).JSON(&saved)
			parsed, _ := workbench.ParseRequirements(saved.AcceptanceCriteria)
			if *parsed.Team.Config.Roles["sched"].AgentID != agent || len(parsed.Plan.Issues) != 1 {
				t.Fatal("plan did not retain its coordinator and one operation")
			}
		})
	}
}

func TestTaskSchedulerDoesNotOverridePlatformRole(t *testing.T) {
	project := dbfx.Project(t, "Platform role check")
	owner := dbfx.User(t, "Platform role owner", "platform-role-owner@example.test")
	agent := dbfx.Agent(t, "Platform product", testRuntimeID, testutil.Cols{"system_key": "onespec:product", "owner_id": owner})
	id := dbfx.Issue(t, "Discuss deployment", testutil.Cols{"project_id": project, "assignee_type": "agent", "assignee_id": agent})
	issue, err := testHandler.Queries.GetIssue(t.Context(), parseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	criteria := workbench.Requirements{Team: &workbench.ProjectTeamSnapshot{ProjectRoleConfiguration: workbench.ProjectRoleConfiguration{Config: workbench.DefaultProjectRoles()}}}
	if err := bindSelectedTaskScheduler(t.Context(), testHandler.Queries, issue, &criteria); err == nil {
		t.Fatal("a platform product agent became the scheduler")
	}
	if criteria.Team.Config.Roles["sched"].AgentID != nil {
		t.Fatal("rejected coordinator changed the team")
	}
}
