package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func TestExecutionPlanExistingProjectDependencies(t *testing.T) {
	ctx := context.Background()
	project := dbfx.Insert(t, "project", testutil.Cols{"workspace_id": testWorkspaceID, "title": "Existing dependencies"})
	parent := dbfx.Issue(t, "Plan with dependencies", testutil.Cols{"project_id": project, "assignee_type": "member", "assignee_id": testUserID})
	prior := dbfx.Issue(t, "Existing prerequisite", testutil.Cols{"project_id": project})
	foreign := dbfx.Issue(t, "Other project task", nil)
	cancelled := dbfx.Issue(t, "Cancelled prerequisite", testutil.Cols{"project_id": project, "status": "cancelled"})
	oldChild := dbfx.Issue(t, "Old child", testutil.Cols{"project_id": project, "parent_issue_id": parent})
	cycleA := dbfx.Issue(t, "Cycle A", testutil.Cols{"project_id": project})
	cycleB := dbfx.Issue(t, "Cycle B", testutil.Cols{"project_id": project})
	backToParent := dbfx.Issue(t, "Depends on parent", testutil.Cols{"project_id": project})
	for id, dependency := range map[string]string{cycleA: cycleB, cycleB: cycleA, backToParent: parent} {
		raw, _ := json.Marshal(map[string]any{"workbench_handoff": map[string]any{"v": 2, "depends_on": []string{dependency}}})
		if _, err := testPool.Exec(ctx, `UPDATE issue SET properties=$1 WHERE id=$2`, raw, id); err != nil {
			t.Fatal(err)
		}
	}
	criteria := workbench.Requirements{V: 1, Version: 1, NextItemNo: 2,
		Items: []workbench.Item{{ID: "A-01", Text: "Document API", Checked: &workbench.Check{By: testUserID, At: "confirmed"}}},
		Gate:  workbench.Gates{Spec: &workbench.GateCheck{PassedAt: "confirmed", PassedBy: testUserID}},
		Team:  &workbench.ProjectTeamSnapshot{ProjectID: project, ProjectRoleConfiguration: workbench.ProjectRoleConfiguration{Config: workbench.DefaultProjectRoles(), Revision: 1}},
	}
	raw, _ := json.Marshal(criteria)
	if _, err := testPool.Exec(ctx, `UPDATE issue SET acceptance_criteria=$1 WHERE id=$2`, raw, parent); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1)) WHERE id=$1`, testWorkspaceID); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM issue WHERE parent_issue_id=$1`, parent)
	current, err := testHandler.Queries.GetIssue(ctx, parseUUID(parent))
	if err != nil {
		t.Fatal(err)
	}
	call := func(dependencies []string, status int) IssueResponse {
		t.Helper()
		tasks := []workbench.PlanTask{{Key: "T-01", Title: "Document", Role: "product", Covers: []string{"A-01"}, Expected: []string{"spec"}, ExistingDependencies: dependencies}}
		req := testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/execution-plan", map[string]any{"expected_revision": current.Revision, "version": 1, "idempotency_key": "external-plan", "tasks": tasks}), "id", parent)
		result := testutil.Call(t, testHandler.SaveExecutionPlan, req).Want(status)
		var response IssueResponse
		if status == http.StatusOK {
			result.JSON(&response)
		}
		return response
	}
	for _, dependency := range []string{foreign, cancelled, parent, oldChild, cycleA, backToParent, uuidToString(dbid.NewV7()), "not-an-id"} {
		call([]string{dependency}, http.StatusBadRequest)
	}
	call([]string{prior, prior}, http.StatusBadRequest)
	var children int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, parent).Scan(&children)
	if children != 1 {
		t.Fatal("rejected dependencies changed the previous children")
	}
	saved := call([]string{prior}, http.StatusOK)
	parsed, _ := workbench.ParseRequirements(saved.AcceptanceCriteria)
	child := parsed.Plan.Issues[0]
	var persisted []byte
	dbfx.QueryRow(t, `SELECT properties FROM issue WHERE id=$1`, child).Scan(&persisted)
	var handoff workbench.Handoff
	if json.Unmarshal(workbench.HandoffJSON(persisted), &handoff) != nil || len(handoff.DependsOn) != 1 || handoff.DependsOn[0] != prior {
		t.Fatal("existing prerequisite was not persisted in the handoff")
	}
	if _, err := testPool.Exec(ctx, `UPDATE issue SET status='in_progress' WHERE id=$1`, child); err == nil {
		t.Fatal("existing prerequisite did not block execution")
	}
	if _, err := testPool.Exec(ctx, `UPDATE issue SET status='done' WHERE id=$1`, prior); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE issue SET status='in_progress' WHERE id=$1`, child); err != nil {
		t.Fatal(err)
	}
}
