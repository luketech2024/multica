package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestBusinessTemplateProjectForkAndRevision(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	var projects []ProjectResponse
	for _, title := range []string{"Template fork one", "Template fork two"} {
		var project ProjectResponse
		testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", map[string]any{
			"title": title, "description": "Project context", "business_template": map[string]any{"id": "apartment", "revision": 1},
		})).Want(http.StatusCreated).JSON(&project)
		projects = append(projects, project)
		t.Cleanup(func() {
			testutil.Call(t, testHandler.DeleteProject, testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+project.ID, nil), "id", project.ID)).Want(http.StatusNoContent)
		})
		if project.ResourceCount != 0 {
			t.Fatal("configuration resources must not count as repositories")
		}
	}
	read := func(id string) workbench.ProjectTemplate {
		t.Helper()
		var response struct {
			Template workbench.ProjectTemplate `json:"template"`
		}
		testutil.Call(t, testHandler.GetProjectBusinessTemplate, testutil.WithURLParams(newRequest("GET", "/api/projects/"+id+"/business-template", nil), "id", id)).Want(http.StatusOK).JSON(&response)
		return response.Template
	}
	saved := read(projects[0].ID)
	if saved.SourceID != "apartment" || saved.SourceRevision != 1 || saved.Revision != 1 {
		t.Fatal("template source or revision was lost")
	}
	write := func(content workbench.BusinessTemplate, revision int64, status int) {
		t.Helper()
		request := testutil.WithURLParams(newRequest("PUT", "/api/projects/"+projects[0].ID+"/business-template", map[string]any{"content": content, "expected_revision": revision}), "id", projects[0].ID)
		testutil.Call(t, testHandler.SaveProjectBusinessTemplate, request).Want(status)
	}
	saved.Content.Name = "Project-specific rules"
	input := map[string]any{"v": 1, "items": []map[string]any{{"text": "Report export"}}}
	first := createIssueWithPropertyBody(t, map[string]any{"title": "Template snapshot one", "project_id": projects[0].ID, "acceptance_criteria": input})
	write(saved.Content, 1, http.StatusOK)
	second := createIssueWithPropertyBody(t, map[string]any{"title": "Template snapshot two", "project_id": projects[0].ID, "acceptance_criteria": input})
	child := createIssueWithPropertyBody(t, map[string]any{"title": "Inherited template", "parent_issue_id": first.ID, "acceptance_criteria": input})
	for _, test := range []struct {
		issue    IssueResponse
		revision int64
	}{{first, 1}, {second, 2}, {child, 1}} {
		var criteria workbench.Requirements
		if json.Unmarshal(test.issue.AcceptanceCriteria, &criteria) != nil || criteria.Template == nil || criteria.Template.Revision != test.revision || criteria.Template.SourceID != "apartment" {
			t.Fatalf("template snapshot changed or missing: %s", test.issue.AcceptanceCriteria)
		}
	}
	write(saved.Content, 1, http.StatusConflict)
	saved.Content.Forbidden[0].Check = ""
	write(saved.Content, 2, http.StatusBadRequest)
	if read(projects[0].ID).Revision != 2 || read(projects[1].ID).Content.Name == "Project-specific rules" {
		t.Fatal("project template edits leaked to another fork or invalid writes advanced revision")
	}
	var resourceID string
	dbfx.QueryRow(t, `SELECT id FROM project_resource WHERE project_id=$1 AND resource_type='workbench_template'`, projects[0].ID).Scan(&resourceID)
	testutil.Call(t, testHandler.DeleteProjectResource, testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+projects[0].ID+"/resources/"+resourceID, nil), "id", projects[0].ID, "resourceId", resourceID)).Want(http.StatusConflict)
}
