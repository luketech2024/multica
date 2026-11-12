package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestCreateProjectWithTeamIsAtomic(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	config := workbench.DefaultProjectRoles()
	config.Roles["sec"] = workbench.ProjectRole{Enabled: false}
	var project ProjectResponse
	body := map[string]any{"title": "Atomic project team", "description": "Verify project team configuration", "workbench_roles": config, "resources": []map[string]any{{"resource_type": "github_repo", "resource_ref": map[string]string{"url": "https://example.test/review.git"}}}}
	testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", body)).Want(http.StatusCreated).JSON(&project)
	t.Cleanup(func() {
		testutil.Call(t, testHandler.DeleteProject, testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+project.ID, nil), "id", project.ID)).Want(http.StatusNoContent)
	})
	if project.ResourceCount != 1 {
		t.Fatal("team configuration counted as a repository")
	}
	var roles ProjectRolesResponse
	testutil.Call(t, testHandler.GetProjectWorkbenchRoles, testutil.WithURLParams(newRequest("GET", "/api/projects/"+project.ID+"/workbench-roles", nil), "id", project.ID)).Want(http.StatusOK).JSON(&roles)
	if roles.Config.Roles["sec"].Enabled || roles.Revision != 1 {
		t.Fatal("initial team configuration missing")
	}
	var audits int
	dbfx.QueryRow(t, `SELECT count(*) FROM activity_log WHERE action='project_roles_created' AND details->>'project_id'=$1`, project.ID).Scan(&audits)
	if audits != 1 {
		t.Fatal("initial configuration audit missing")
	}
	for _, invalid := range []map[string]any{{"title": "A"}, {"description": nil}} {
		request := testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project.ID, invalid), "id", project.ID)
		testutil.Call(t, testHandler.UpdateProject, request).Want(http.StatusBadRequest)
	}
	config.Roles["product"] = workbench.ProjectRole{Enabled: false}
	body["title"], body["workbench_roles"] = "Invalid atomic project team", config
	testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", body)).Want(http.StatusBadRequest)
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM project WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, body["title"]).Scan(&count)
	if count != 0 {
		t.Fatal("invalid team left a partially created project")
	}
}

func TestProjectTeamSnapshotSurvivesConfigurationAndRequirementEdits(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	project := dbfx.Project(t, "Team snapshots")
	config := workbench.DefaultProjectRoles()
	config.Roles["sec"] = workbench.ProjectRole{Enabled: false}
	save := func(revision int64) {
		t.Helper()
		request := testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project+"/workbench-roles", map[string]any{"config": config, "expected_revision": revision}), "id", project)
		testutil.Call(t, testHandler.SaveProjectWorkbenchRoles, request).Want(http.StatusOK)
	}
	save(1)
	input := map[string]any{"v": 1, "items": []map[string]any{{"text": "Report export"}}}
	first := createIssueWithPropertyBody(t, map[string]any{"title": "First team snapshot", "project_id": project, "acceptance_criteria": input})
	config.Roles["sec"] = workbench.ProjectRole{Enabled: true}
	save(2)
	second := createIssueWithPropertyBody(t, map[string]any{"title": "Second team snapshot", "project_id": project, "acceptance_criteria": input})
	child := createIssueWithPropertyBody(t, map[string]any{"title": "Inherited team snapshot", "parent_issue_id": first.ID, "acceptance_criteria": input})
	assertTeam := func(issue IssueResponse, enabled bool, revision int64) workbench.Requirements {
		t.Helper()
		criteria, err := workbench.ParseRequirements(issue.AcceptanceCriteria)
		if err != nil || criteria.Team == nil || criteria.Team.Config.Roles["sec"].Enabled != enabled || criteria.Team.Revision != revision || criteria.Team.ProjectID != project {
			t.Fatalf("unexpected team snapshot: %s", issue.AcceptanceCriteria)
		}
		return criteria
	}
	criteria := assertTeam(first, false, 2)
	assertTeam(second, true, 3)
	assertTeam(child, false, 2)
	criteria.Team.Config.Roles["sec"] = workbench.ProjectRole{Enabled: true}
	criteria.Items[0].Text = "Updated export"
	var updated IssueResponse
	request := testutil.WithURLParams(newRequest("PUT", "/api/issues/"+first.ID, map[string]any{"acceptance_criteria": criteria, "expected_revision": first.Revision}), "id", first.ID)
	testutil.Call(t, testHandler.UpdateIssue, request).Want(http.StatusOK).JSON(&updated)
	assertTeam(updated, false, 2)
	var persisted json.RawMessage
	dbfx.QueryRow(t, `SELECT acceptance_criteria FROM issue WHERE id=$1`, first.ID).Scan(&persisted)
	updated.AcceptanceCriteria = persisted
	assertTeam(updated, false, 2)
}

func TestProjectRolesPersistenceRequiredRolesAndRevision(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	project := dbfx.Project(t, "Project role configuration")
	var initial ProjectRolesResponse
	testutil.Call(t, testHandler.GetProjectWorkbenchRoles, testutil.WithURLParams(newRequest("GET", "/api/projects/"+project+"/workbench-roles", nil), "id", project)).Want(http.StatusOK).JSON(&initial)
	if initial.Revision != 1 || len(initial.Config.Roles) != len(workbench.ProjectRoleOrder) {
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
