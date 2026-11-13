package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestOneSpecSetupNewWorkspaceHasPendingRoles(t *testing.T) {
	var ws WorkspaceResponse
	testutil.Call(t, testHandler.CreateWorkspace, newRequest("POST", "/", map[string]any{"name": "Pending OneSpec", "slug": "pending-" + uuid.NewString()})).Want(201).JSON(&ws)
	t.Cleanup(func() {
		_ = testHandler.Queries.DeleteOneSpecWorkspace(context.Background(), parseUUID(ws.ID))
		_, _ = testPool.Exec(context.Background(), `DELETE FROM workspace WHERE id=$1`, ws.ID)
	})
	var settings oneSpecConfiguration
	testutil.Call(t, testHandler.GetOneSpecWorkspace, testutil.WithURLParams(newRequest("GET", "/", nil), "id", ws.ID)).Want(200).JSON(&settings)
	if settings.Setup.Status != "pending_runtime" || len(settings.Setup.Roles) != 7 {
		t.Fatalf("workspace did not initialize pending roles: %+v", settings.Setup)
	}
}

func oneSpecSetupWorkspace(t *testing.T) (string, func(string, string, any) *http.Request) {
	t.Helper()
	ws := dbfx.Workspace(t, t.Name(), "setup-"+uuid.NewString())
	dbfx.Member(t, ws, testUserID, "owner")
	t.Cleanup(func() { _ = testHandler.Queries.DeleteOneSpecWorkspace(context.Background(), parseUUID(ws)) })
	return ws, func(method, id string, body any) *http.Request {
		r := testutil.WithURLParams(newRequest(method, "/", body), "id", id)
		r.Header.Set("X-Workspace-ID", ws)
		return r
	}
}

func TestOneSpecSetupRequiresProjectOptIn(t *testing.T) {
	ws, request := oneSpecSetupWorkspace(t)
	runtime := dbfx.Runtime(t, "Workspace runtime", testutil.Cols{"workspace_id": ws})
	testutil.Call(t, testHandler.SaveOneSpecWorkspace, request("PUT", ws, map[string]any{"default_runtime_id": runtime, "expected_revision": 0})).Want(200)
	for _, mode := range []string{"omitted", "disabled", "enabled"} {
		t.Run(mode, func(t *testing.T) {
			body := map[string]any{"title": "Project " + mode, "description": "Setup test"}
			if mode != "omitted" {
				body["onespec_enabled"] = mode == "enabled"
			}
			var project ProjectResponse
			testutil.Call(t, testHandler.CreateProject, request("POST", "", body)).Want(201).JSON(&project)
			var settings oneSpecConfiguration
			testutil.Call(t, testHandler.GetOneSpecProject, request("GET", project.ID, nil)).Want(200).JSON(&settings)
			if settings.Enabled != (mode == "enabled") {
				t.Fatalf("unexpected enabled state: %+v", settings)
			}
			var count int
			dbfx.QueryRow(t, `SELECT count(*) FROM project_resource WHERE project_id=$1 AND resource_type='workbench_roles'`, project.ID).Scan(&count)
			if mode != "enabled" {
				if count != 0 {
					t.Fatal("ordinary project received a OneSpec team")
				}
				testutil.Call(t, testHandler.ProvisionOneSpecTeam, request("POST", project.ID, nil)).Want(409)
				return
			}
			if count != 1 || settings.Setup.Status != "ready" || len(settings.Setup.Roles) != 7 {
				t.Fatalf("automatic team missing: %+v", settings.Setup)
			}
			for _, role := range settings.Setup.Roles {
				if role.AgentID == nil || role.RuntimeID == nil || *role.RuntimeID != runtime || role.BindingSource != "onespec_default" {
					t.Fatalf("unexpected role: %+v", role)
				}
			}
		})
	}
}

func TestOneSpecSetupConcurrentRetries(t *testing.T) {
	ws, request := oneSpecSetupWorkspace(t)
	runtime := dbfx.Runtime(t, "Concurrent runtime", testutil.Cols{"workspace_id": ws})
	dbfx.Exec(t, `INSERT INTO onespec_workspace_config (workspace_id,default_runtime_id) VALUES ($1,$2)`, ws, runtime)
	project := dbfx.Project(t, "Concurrent setup", testutil.Cols{"workspace_id": ws})
	dbfx.Exec(t, `INSERT INTO onespec_project_config (workspace_id,project_id,enabled) VALUES ($1,$2,true)`, ws, project)
	responses := make(chan *httptest.ResponseRecorder, 3)
	for range 3 {
		r := request("POST", project, nil)
		go func() {
			response := httptest.NewRecorder()
			testHandler.ProvisionOneSpecTeam(response, r)
			responses <- response
		}()
	}
	var agentID string
	for range 3 {
		response := <-responses
		if response.Code != 200 {
			t.Fatalf("concurrent setup failed: %s", response.Body.String())
		}
		var team ProjectRolesResponse
		if err := json.Unmarshal(response.Body.Bytes(), &team); err != nil {
			t.Fatal(err)
		}
		current := *team.Config.Roles["dev"].AgentID
		if agentID != "" && current != agentID {
			t.Fatal("concurrent setup created different agents")
		}
		agentID = current
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent WHERE workspace_id=$1 AND system_key LIKE 'onespec:%'`, ws).Scan(&count)
	if count != 7 {
		t.Fatalf("concurrent setup created %d agents", count)
	}
}

func TestOneSpecSetupPendingRuntimeAndRetry(t *testing.T) {
	ws, request := oneSpecSetupWorkspace(t)
	var project ProjectResponse
	testutil.Call(t, testHandler.CreateProject, request("POST", "", map[string]any{"title": "Pending team", "description": "Configure later", "onespec_enabled": true})).Want(201).JSON(&project)
	var settings oneSpecConfiguration
	testutil.Call(t, testHandler.GetOneSpecProject, request("GET", project.ID, nil)).Want(200).JSON(&settings)
	if settings.Setup.Status != "pending_runtime" || len(settings.Setup.Roles) != 7 {
		t.Fatalf("expected pending roles: %+v", settings.Setup)
	}
	var issue IssueResponse
	testutil.Call(t, testHandler.CreateIssue, request("POST", "", map[string]any{"title": "Start after setup", "project_id": project.ID})).Want(201).JSON(&issue)
	var flow oneSpecResponse
	testutil.Call(t, testHandler.GetOneSpecFlow, request("GET", issue.ID, nil)).Want(200).JSON(&flow)
	runtime := dbfx.Runtime(t, "Later runtime", testutil.Cols{"workspace_id": ws})
	testutil.Call(t, testHandler.SaveOneSpecWorkspace, request("PUT", ws, map[string]any{"default_runtime_id": runtime, "expected_revision": 0})).Want(200)
	run := testutil.WithURLParams(request("POST", issue.ID, map[string]any{"expected_revision": flow.Revision}), "id", issue.ID, "action", "run")
	testutil.Call(t, testHandler.UpdateOneSpecFlow, run).Want(200).JSON(&flow)
	if flow.Flow.RuntimeID != runtime || flow.Flow.Team.Config.Roles["product"].AgentID == nil {
		t.Fatal("first execution did not persist its prepared team snapshot")
	}
	var first, second ProjectRolesResponse
	testutil.Call(t, testHandler.ProvisionOneSpecTeam, request("POST", project.ID, nil)).Want(200).JSON(&first)
	testutil.Call(t, testHandler.ProvisionOneSpecTeam, request("POST", project.ID, nil)).Want(200).JSON(&second)
	if first.Revision != second.Revision || first.Config.Roles["dev"].AgentID == nil || !sameRoleID(first.Config.Roles["dev"].AgentID, second.Config.Roles["dev"].AgentID) {
		t.Fatal("setup retry changed bindings")
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent WHERE workspace_id=$1 AND system_key LIKE 'onespec:%'`, ws).Scan(&count)
	if count != 7 {
		t.Fatalf("retry produced %d agents", count)
	}
	dev := *first.Config.Roles["dev"].AgentID
	dbfx.Exec(t, `UPDATE agent SET archived_at=now() WHERE id=$1`, dev)
	testutil.Call(t, testHandler.ProvisionOneSpecTeam, request("POST", project.ID, nil)).Want(200)
	testutil.Call(t, testHandler.GetOneSpecProject, request("GET", project.ID, nil)).Want(200).JSON(&settings)
	for _, role := range settings.Setup.Roles {
		if role.Key == "dev" && (role.Status != "archived" || role.AgentID == nil || *role.AgentID != dev) {
			t.Fatalf("archived agent replaced: %+v", role)
		}
	}
}

func TestOneSpecSetupInheritedRolesRemainAutomaticAfterEditing(t *testing.T) {
	ws, request := oneSpecSetupWorkspace(t)
	first := dbfx.Runtime(t, "First default", testutil.Cols{"workspace_id": ws})
	second := dbfx.Runtime(t, "Second default", testutil.Cols{"workspace_id": ws})
	testutil.Call(t, testHandler.SaveOneSpecWorkspace, request("PUT", ws, map[string]any{"default_runtime_id": first, "expected_revision": 0})).Want(200)
	var project ProjectResponse
	testutil.Call(t, testHandler.CreateProject, request("POST", "", map[string]any{"title": "Inherited team", "onespec_enabled": true})).Want(201).JSON(&project)
	testutil.Call(t, testHandler.SaveOneSpecWorkspace, request("PUT", ws, map[string]any{"default_runtime_id": second, "expected_revision": 1})).Want(200)
	var team ProjectRolesResponse
	testutil.Call(t, testHandler.GetProjectWorkbenchRoles, request("GET", project.ID, nil)).Want(200).JSON(&team)
	if *team.Config.Roles["product"].RuntimeID != second {
		t.Fatal("inherited configuration was stale")
	}
	dev := team.Config.Roles["dev"]
	dev.BindingSource = "explicit"
	team.Config.Roles["dev"] = dev
	testutil.Call(t, testHandler.SaveProjectWorkbenchRoles, request("PUT", project.ID, map[string]any{"config": team.Config, "expected_revision": team.Revision})).Want(200)
	testutil.Call(t, testHandler.SaveOneSpecWorkspace, request("PUT", ws, map[string]any{"default_runtime_id": first, "expected_revision": 2})).Want(200)
	testutil.Call(t, testHandler.GetProjectWorkbenchRoles, request("GET", project.ID, nil)).Want(200).JSON(&team)
	if *team.Config.Roles["dev"].RuntimeID != second || *team.Config.Roles["product"].RuntimeID != first {
		t.Fatal("role editing lost explicit or inherited runtime semantics")
	}
}

func TestOneSpecSetupPreservesCustomRolesAndRuntimeIsolation(t *testing.T) {
	ws, request := oneSpecSetupWorkspace(t)
	first := dbfx.Runtime(t, "Default runtime", testutil.Cols{"workspace_id": ws})
	second := dbfx.Runtime(t, "Dedicated runtime", testutil.Cols{"workspace_id": ws})
	custom := dbfx.Agent(t, "Custom developer", first, testutil.Cols{"workspace_id": ws})
	config := workbench.DefaultProjectRoles()
	config.Roles["dev"] = workbench.ProjectRole{Enabled: true, AgentID: &custom, RuntimeID: &first}
	var project ProjectResponse
	testutil.Call(t, testHandler.SaveOneSpecWorkspace, request("PUT", ws, map[string]any{"default_runtime_id": first, "expected_revision": 0})).Want(200)
	testutil.Call(t, testHandler.CreateProject, request("POST", "", map[string]any{"title": "Custom team", "description": "Keep explicit role", "onespec_enabled": true, "workbench_roles": config})).Want(201).JSON(&project)
	testutil.Call(t, testHandler.SaveOneSpecProject, request("PUT", project.ID, map[string]any{"enabled": true, "runtime_id": second, "expected_revision": 1})).Want(200)
	var settings oneSpecConfiguration
	testutil.Call(t, testHandler.GetOneSpecProject, request("GET", project.ID, nil)).Want(200).JSON(&settings)
	if *settings.Setup.Team.Config.Roles["dev"].AgentID != custom || *settings.Setup.Team.Config.Roles["product"].RuntimeID != second {
		t.Fatal("runtime change replaced custom binding or failed to update defaults")
	}
	foreign := dbfx.Runtime(t, "Other workspace runtime")
	testutil.Call(t, testHandler.SaveOneSpecProject, request("PUT", project.ID, map[string]any{"enabled": true, "runtime_id": foreign, "expected_revision": 2})).Want(403)
	other := dbfx.User(t, "Workspace member", "onespec-setup-member@example.test")
	dbfx.Member(t, ws, other, "member")
	r := request("GET", project.ID, nil)
	r.Header.Set("X-User-ID", other)
	testutil.Call(t, testHandler.GetOneSpecProject, r).Want(200).JSON(&settings)
	for _, role := range settings.Setup.Roles {
		if role.Key == "product" && role.Status != "ready" {
			t.Fatalf("workspace member cannot use the shared agent: %+v", role)
		}
	}
	r = request("PUT", ws, map[string]any{"default_runtime_id": second, "expected_revision": 1})
	r.Header.Set("X-User-ID", other)
	testutil.Call(t, testHandler.SaveOneSpecWorkspace, r).Want(403)
}
