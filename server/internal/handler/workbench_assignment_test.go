package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestWorkbenchDefaultAssigneeUsesProjectTeamAndPreservesManualChoice(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	project := dbfx.Project(t, "Default workbench assignments")
	runtime := dbfx.Runtime(t, "Assignment test runtime")
	product := dbfx.Agent(t, "Product assignment", runtime)
	sched := dbfx.Agent(t, "Scheduler assignment", runtime)
	roles := workbench.DefaultProjectRoles()
	roles.Roles["product"] = workbench.ProjectRole{Enabled: true, AgentID: &product}
	roles.Roles["sched"] = workbench.ProjectRole{Enabled: true, AgentID: &sched}
	raw, _ := json.Marshal(workbench.ProjectRoleConfiguration{Revision: 1, Config: roles})
	dbfx.Insert(t, "project_resource", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "resource_type": "workbench_roles", "resource_ref": string(raw)})
	for _, scenario := range []struct {
		name     string
		items    []map[string]any
		manual   bool
		clear    bool
		wantType string
		wantID   string
	}{
		{name: "Empty criteria use product", items: []map[string]any{}, wantType: "agent", wantID: product},
		{name: "Prepared criteria use scheduler", items: []map[string]any{{"text": "Export report"}}, wantType: "agent", wantID: sched},
		{name: "Manual choice is retained", items: []map[string]any{}, manual: true, wantType: "member", wantID: testUserID},
		{name: "Explicitly unassigned is retained", items: []map[string]any{}, clear: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			body := map[string]any{"title": scenario.name, "project_id": project, "status": "backlog", "acceptance_criteria": map[string]any{"v": 1, "items": scenario.items}}
			if scenario.manual {
				body["assignee_type"], body["assignee_id"] = "member", testUserID
			}
			if scenario.clear {
				body["use_project_assignee"] = false
			}
			created := createIssueWithPropertyBody(t, body)
			if scenario.wantID == "" {
				if created.AssigneeID != nil || created.AssigneeType != nil {
					t.Fatal("explicitly cleared assignment was overwritten")
				}
			} else if created.AssigneeID == nil || *created.AssigneeID != scenario.wantID || created.AssigneeType == nil || *created.AssigneeType != scenario.wantType {
				t.Fatalf("unexpected assignment %+v / %+v", created.AssigneeType, created.AssigneeID)
			}
		})
	}
	other := dbfx.Project(t, "Unconfigured team")
	created := createIssueWithPropertyBody(t, map[string]any{"title": "Creator owns unconfigured work", "project_id": other, "status": "backlog", "acceptance_criteria": map[string]any{"v": 1, "items": []any{}}})
	if created.AssigneeID == nil || *created.AssigneeID != testUserID || *created.AssigneeType != "member" {
		t.Fatal("unconfigured project has no accountable member")
	}
	foreignOwner := dbfx.User(t, "Private agent owner", "assignment-private-"+project+"@multica.ai")
	dbfx.Exec(t, `UPDATE agent SET owner_id=$1, permission_mode='private' WHERE id=$2`, foreignOwner, product)
	body := map[string]any{"title": "Inaccessible product assignment", "project_id": project, "status": "backlog", "acceptance_criteria": map[string]any{"v": 1, "items": []any{}}}
	testutil.Call(t, testHandler.CreateIssue, newRequest("POST", "/api/issues", body)).Want(http.StatusBadRequest)
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, body["title"]).Scan(&count)
	if count != 0 {
		t.Fatal("unauthorized default assignment created an issue")
	}
}
