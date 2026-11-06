package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestProjectRolesPersistenceRequiredRolesAndRevision(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	project := dbfx.Project(t, "Project role configuration")
	var initial ProjectRolesResponse
	testutil.Call(t, testHandler.GetProjectWorkbenchRoles, testutil.WithURLParams(newRequest("GET", "/api/projects/"+project+"/workbench-roles", nil), "id", project)).Want(http.StatusOK).JSON(&initial)
	if initial.Revision != 1 || len(initial.Config.Roles) != 6 {
		t.Fatal("missing initial project roles")
	}
	write := func(config workbench.ProjectRoles, revision int64, status int) ProjectRolesResponse {
		t.Helper()
		request := testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project+"/workbench-roles", map[string]any{"config": config, "expected_revision": revision}), "id", project)
		result := testutil.Call(t, testHandler.SaveProjectWorkbenchRoles, request).Want(status)
		var response ProjectRolesResponse
		if status == http.StatusOK {
			result.JSON(&response)
		}
		return response
	}
	initial.Config.Roles["sched"] = workbench.ProjectRole{Enabled: false}
	write(initial.Config, initial.Revision, http.StatusBadRequest)
	initial.Config.Roles["sched"] = workbench.ProjectRole{Enabled: true}
	initial.Config.Roles["sec"] = workbench.ProjectRole{Enabled: false}
	saved := write(initial.Config, initial.Revision, http.StatusOK)
	if saved.Config.Roles["sec"].Enabled || saved.Revision != 2 {
		t.Fatal("optional role change was lost")
	}
	write(initial.Config, initial.Revision, http.StatusConflict)
	var reloaded ProjectRolesResponse
	testutil.Call(t, testHandler.GetProjectWorkbenchRoles, testutil.WithURLParams(newRequest("GET", "/api/projects/"+project+"/workbench-roles", nil), "id", project)).Want(http.StatusOK).JSON(&reloaded)
	if reloaded.Config.Roles["sec"].Enabled {
		t.Fatal("disabled role did not persist")
	}
	var resourceID string
	dbfx.QueryRow(t, `SELECT id FROM project_resource WHERE project_id=$1 AND resource_type='workbench_roles'`, project).Scan(&resourceID)
	for _, method := range []string{"PUT", "DELETE"} {
		handler := testHandler.UpdateProjectResource
		if method == "DELETE" {
			handler = testHandler.DeleteProjectResource
		}
		request := testutil.WithURLParams(newRequest(method, "/api/projects/"+project+"/resources/"+resourceID, map[string]any{"label": "Bypass"}), "id", project, "resourceId", resourceID)
		testutil.Call(t, handler, request).Want(http.StatusConflict)
	}
	var resources struct {
		Total int `json:"total"`
	}
	testutil.Call(t, testHandler.ListProjectResources, testutil.WithURLParams(newRequest("GET", "/api/projects/"+project+"/resources", nil), "id", project)).Want(http.StatusOK).JSON(&resources)
	if resources.Total != 0 {
		t.Fatal("team configuration must not appear as a repository")
	}
	missingRuntime := "11111111-1111-1111-1111-111111111111"
	initial.Config.Roles["product"] = workbench.ProjectRole{Enabled: true, RuntimeID: &missingRuntime}
	write(initial.Config, saved.Revision, http.StatusBadRequest)
	var audits int
	dbfx.QueryRow(t, `SELECT count(*) FROM activity_log WHERE action='project_roles_updated' AND details->>'project_id'=$1`, project).Scan(&audits)
	if audits != 1 {
		t.Fatalf("expected one committed audit, got %d", audits)
	}
}
