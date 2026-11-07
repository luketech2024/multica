package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
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

func TestWorkspaceBusinessTemplatePublicationAndSelectiveUpgrade(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	builtins, err := workbench.BuiltinBusinessTemplates()
	if err != nil {
		t.Fatal(err)
	}
	content := builtins[0].Content
	content.Name = "Shared project rules"
	publish := func(h *Handler, id string, revision int64, content workbench.BusinessTemplate, status int) workbench.BusinessTemplateAsset {
		t.Helper()
		request := newRequest("POST", "/api/workbench/business-templates", map[string]any{"content": content, "expected_revision": revision})
		if id != "" {
			request = testutil.WithURLParams(request, "templateId", id)
		}
		response := testutil.Call(t, h.SaveWorkspaceBusinessTemplate, request).Want(status)
		var asset workbench.BusinessTemplateAsset
		if status == 200 {
			response.JSON(&asset)
		}
		return asset
	}
	asset := publish(testHandler, "", 0, content, 200)
	dbfx.Cleanup(t, "DELETE FROM workbench_business_templates WHERE id=$1", asset.ID)
	dbfx.Cleanup(t, "DELETE FROM activity_log WHERE details->>'template_id'=$1", asset.ID)
	if asset.Builtin || asset.Revision != 1 {
		t.Fatal("custom template must start at revision 1")
	}
	var catalog struct {
		Templates []workbench.BusinessTemplateAsset `json:"templates"`
	}
	testutil.Call(t, testHandler.ListBusinessTemplates, newRequest("GET", "/api/workbench/business-templates", nil)).Want(200).JSON(&catalog)
	found := false
	for _, entry := range catalog.Templates {
		if entry.ID == asset.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("published template missing from catalog")
	}
	var project ProjectResponse
	testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", map[string]any{"title": "Custom template fork", "description": "Project context", "business_template": map[string]any{"id": asset.ID, "revision": 1}})).Want(201).JSON(&project)
	t.Cleanup(func() {
		testutil.Call(t, testHandler.DeleteProject, testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+project.ID, nil), "id", project.ID)).Want(204)
	})
	read := func() workbench.ProjectTemplate {
		var result struct {
			Template workbench.ProjectTemplate `json:"template"`
		}
		testutil.Call(t, testHandler.GetProjectBusinessTemplate, testutil.WithURLParams(newRequest("GET", "/", nil), "id", project.ID)).Want(200).JSON(&result)
		return result.Template
	}
	content.Name = "Shared project rules v2"
	content.DefinitionOfDone = []string{"Validated against the current source commit"}
	publish(testHandler, asset.ID, 1, content, 200)
	publish(testHandler, asset.ID, 1, content, 409)
	if read().Content.Name != "Shared project rules" || read().SourceRevision != 1 {
		t.Fatal("publication changed a fork automatically")
	}
	merged := read().Content
	merged.DefinitionOfDone = content.DefinitionOfDone
	merge := func(source, revision int64, status int) {
		request := testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"content": merged, "expected_revision": revision, "source_revision": source}), "id", project.ID)
		testutil.Call(t, testHandler.SaveProjectBusinessTemplate, request).Want(status)
	}
	merge(3, 1, 409)
	merge(2, 1, 200)
	merge(2, 1, 409)
	saved := read()
	if saved.SourceRevision != 2 || saved.Revision != 2 || saved.Content.Name != "Shared project rules" || saved.Content.DefinitionOfDone[0] != content.DefinitionOfDone[0] {
		t.Fatal("selective merge lost project values or source version")
	}
	failing := *testHandler
	failing.TxStarter = rollbackOnCommitTxStarter{pool: testPool}
	publish(&failing, asset.ID, 2, content, 500)
	row, err := testHandler.Queries.GetWorkspaceBusinessTemplateForUpdate(context.Background(), db.GetWorkspaceBusinessTemplateForUpdateParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(asset.ID)})
	if err != nil || row.Revision != 2 {
		t.Fatalf("failed commit changed asset: revision=%d err=%v", row.Revision, err)
	}
	var audits int
	dbfx.QueryRow(t, "SELECT count(*) FROM activity_log WHERE details->>'template_id'=$1 AND action='business_template_published'", asset.ID).Scan(&audits)
	if audits != 2 {
		t.Fatalf("publication audits=%d, want two committed writes", audits)
	}
	invalid := content
	invalid.Forbidden = []workbench.TemplateProhibition{{Rule: "Never publish secrets"}}
	publish(testHandler, asset.ID, 2, invalid, 400)
	foreign := dbfx.Workspace(t, "Other templates", "other-templates-"+asset.ID)
	dbfx.Exec(t, "UPDATE workbench_business_templates SET workspace_id=$1 WHERE id=$2", foreign, asset.ID)
	publish(testHandler, asset.ID, 2, content, 404)
	outsider := dbfx.User(t, "Template outsider", "template-outsider-"+asset.ID+"@example.test")
	outsiderRequest := newRequest("POST", "/api/workbench/business-templates", map[string]any{"content": content})
	outsiderRequest.Header.Set("X-User-ID", outsider)
	testutil.Call(t, testHandler.SaveWorkspaceBusinessTemplate, outsiderRequest).Want(404)
	agentRequest := newRequest("POST", "/api/workbench/business-templates", map[string]any{"content": content})
	agentRequest.Header.Set("X-Actor-Source", "task_token")
	agentRequest.Header.Set("X-Agent-ID", testUserID)
	testutil.Call(t, testHandler.SaveWorkspaceBusinessTemplate, agentRequest).Want(403)
	dbfx.Member(t, foreign, testUserID, "owner")
	testutil.Call(t, testHandler.DeleteWorkspace, testutil.WithURLParams(newRequest("DELETE", "/api/workspaces/"+foreign, nil), "id", foreign)).Want(204)
	var remaining int
	dbfx.QueryRow(t, "SELECT count(*) FROM workbench_business_templates WHERE id=$1", asset.ID).Scan(&remaining)
	if remaining != 0 || read().Content.Name != "Shared project rules" {
		t.Fatal("workspace deletion must remove its assets and retain other project copies")
	}
}
