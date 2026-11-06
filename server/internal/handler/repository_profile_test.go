package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestRepositoryProfilePersistsWithRevisionAndSurvivesRepositoryEdits(t *testing.T) {
	project := dbfx.Insert(t, "project", testutil.Cols{"workspace_id": testWorkspaceID, "title": "Repository profiles"})
	repo := dbfx.Insert(t, "project_resource", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "resource_type": "github_repo", "resource_ref": `{"url":"https://example.com/api.git","ref":"main"}`})
	otherRepo := dbfx.Insert(t, "project_resource", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "resource_type": "github_repo", "resource_ref": `{"url":"https://example.com/web.git"}`})
	input := workbench.RepositoryProfileInput{Template: "java-backend", Tags: []string{"java21", "integration-test"}, Delivery: []string{"docker-image"}}
	call := func(id string, status int) workbench.RepositoryProfile {
		t.Helper()
		req := testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project+"/resources/"+id+"/workbench-profile", input), "id", project, "resourceId", id)
		result := testutil.Call(t, testHandler.SaveRepositoryProfile, req).Want(status)
		var response workbench.RepositoryProfile
		if status == http.StatusOK {
			result.JSON(&response)
		}
		return response
	}
	saved := call(repo, http.StatusOK)
	if saved.Revision != 1 || saved.Stack.Derived.Test != "mvn -q -B verify" || string(saved.Stack.Baseline) != "null" {
		t.Fatalf("unexpected saved profile %+v", saved)
	}
	call(repo, http.StatusConflict)
	duplicate := testutil.WithURLParams(newRequest("POST", "/api/projects/"+project+"/resources", map[string]any{
		"resource_type": "github_repo",
		"resource_ref":  map[string]any{"url": "https://example.com/api.git", "ref": "main"},
	}), "id", project)
	testutil.Call(t, testHandler.CreateProjectResource, duplicate).Want(http.StatusConflict)
	var detail workbench.RepositoryProfile
	testutil.Call(t, testHandler.GetRepositoryProfile, testutil.WithURLParams(newRequest("GET", "/api/projects/"+project+"/resources/"+repo+"/workbench-profile", nil), "id", project, "resourceId", repo)).Want(http.StatusOK).JSON(&detail)
	if detail.Revision != 1 || detail.Stack.Template != "java-backend" {
		t.Fatal("profile did not survive reread")
	}
	testutil.Call(t, testHandler.GetRepositoryProfile, testutil.WithURLParams(newRequest("GET", "/api/projects/"+project+"/resources/"+otherRepo+"/workbench-profile", nil), "id", project, "resourceId", otherRepo)).Want(http.StatusOK).JSON(&detail)
	if detail.Revision != 0 {
		t.Fatal("profile leaked to another repository")
	}
	edit := testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project+"/resources/"+repo, map[string]any{"resource_ref": map[string]any{"url": "https://example.com/api.git", "ref": "main", "stack": map[string]any{"v": 99}}}), "id", project, "resourceId", repo)
	testutil.Call(t, testHandler.UpdateProjectResource, edit).Want(http.StatusOK)
	row, err := testHandler.Queries.GetProjectResource(context.Background(), parseUUID(repo))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := workbench.ReadRepositoryProfile(row.ResourceRef)
	if err != nil || profile.Stack.V != 3 || profile.Revision != 1 {
		t.Fatal("ordinary resource update discarded or forged the profile")
	}
	edit = testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project+"/resources/"+repo, map[string]any{"resource_ref": map[string]any{"url": "https://example.com/api.git", "ref": "release"}}), "id", project, "resourceId", repo)
	testutil.Call(t, testHandler.UpdateProjectResource, edit).Want(http.StatusOK)
	row, err = testHandler.Queries.GetProjectResource(context.Background(), parseUUID(repo))
	if err != nil {
		t.Fatal(err)
	}
	profile, err = workbench.ReadRepositoryProfile(row.ResourceRef)
	if err != nil || profile.Revision != 2 {
		t.Fatal("source change failed to advance profile revision")
	}
	input.ExpectedRevision = 1
	call(repo, http.StatusConflict)
	input.ExpectedRevision = 2
	input.Overrides = map[string]string{"test": "./verify.sh"}
	if updated := call(repo, http.StatusOK); updated.Revision != 3 || updated.Stack.Overrides["test"].By != "manual" {
		t.Fatal("override not saved with provenance")
	}
	input.ExpectedRevision = 3
	statuses := make(chan int, 2)
	for range 2 {
		req := testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project+"/resources/"+repo+"/workbench-profile", input), "id", project, "resourceId", repo)
		go func() {
			response := httptest.NewRecorder()
			testHandler.SaveRepositoryProfile(response, req)
			statuses <- response.Code
		}()
	}
	first, second := <-statuses, <-statuses
	if !((first == http.StatusOK && second == http.StatusConflict) || (second == http.StatusOK && first == http.StatusConflict)) {
		t.Fatalf("concurrent updates returned %d and %d", first, second)
	}
	var auditCount int
	dbfx.QueryRow(t, `SELECT count(*) FROM activity_log WHERE workspace_id=$1 AND action='repository_profile_updated' AND details->>'repo_id'=$2`, testWorkspaceID, repo).Scan(&auditCount)
	if auditCount != 3 {
		t.Fatalf("expected three successful audits, got %d", auditCount)
	}
	var raw map[string]any
	if json.Unmarshal(row.ResourceRef, &raw) != nil || raw["url"] != "https://example.com/api.git" {
		t.Fatal("repository source was lost")
	}
}
