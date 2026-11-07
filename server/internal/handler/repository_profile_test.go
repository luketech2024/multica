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
	input.Overrides = map[string]string{"test": "./verify.sh", "build": "./build.sh"}
	if updated := call(repo, http.StatusOK); updated.Revision != 3 || updated.Stack.Overrides["test"].By != "manual" || updated.Stack.Overrides["build"].Value != "./build.sh" || updated.Stack.Overrides["build"].By != "manual" {
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

func TestCreateProjectRepositoryProfilesAreAtomicAndIndependent(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	input := workbench.RepositoryProfileInput{Template: "java-backend", Tags: []string{"java21"}, Delivery: []string{"docker-image"}}
	resources := []map[string]any{
		{"resource_type": "github_repo", "resource_ref": map[string]string{"url": "https://example.test/api.git"}, "repository_profile": input},
		{"resource_type": "github_repo", "resource_ref": map[string]string{"url": "https://example.test/web.git"}, "repository_profile": workbench.RepositoryProfileInput{Template: "web-frontend", Tags: []string{"ts"}, Delivery: []string{"static-site"}}},
	}
	body := map[string]any{"title": "Atomic repository profiles", "resources": resources}
	var created struct {
		ProjectResponse
		Resources []ProjectResourceResponse `json:"resources"`
	}
	testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", body)).Want(http.StatusCreated).JSON(&created)
	t.Cleanup(func() {
		testutil.Call(t, testHandler.DeleteProject, testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+created.ID, nil), "id", created.ID)).Want(http.StatusNoContent)
	})
	if len(created.Resources) != 2 {
		t.Fatal("expected two independently configured repositories")
	}
	for i, template := range []string{"java-backend", "web-frontend"} {
		profile, err := workbench.ReadRepositoryProfile(created.Resources[i].ResourceRef)
		if err != nil || profile.Revision != 1 || profile.Stack.Template != template {
			t.Fatalf("repository %d profile: %+v, %v", i, profile, err)
		}
	}
	var audits int
	dbfx.QueryRow(t, `SELECT count(*) FROM activity_log WHERE action='repository_profile_created' AND details->>'project_id'=$1`, created.ID).Scan(&audits)
	if audits != 2 {
		t.Fatalf("expected two profile audits, got %d", audits)
	}
	body["title"] = "Invalid repository profiles"
	input.Template = "invalid"
	resources[1]["repository_profile"] = input
	testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", body)).Want(http.StatusBadRequest)
	input.Template, input.ExpectedRevision = "java-backend", 1
	resources[1]["repository_profile"] = input
	testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", body)).Want(http.StatusBadRequest)
	var partial int
	dbfx.QueryRow(t, `SELECT count(*) FROM project WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, body["title"]).Scan(&partial)
	if partial != 0 {
		t.Fatal("invalid repository profile left a partial project")
	}
	input.ExpectedRevision = 0
	resources[1] = map[string]any{"resource_type": "github_repo", "resource_ref": map[string]string{"url": "https://example.test/api.git"}, "repository_profile": input}
	testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", body)).Want(http.StatusConflict)
	dbfx.QueryRow(t, `SELECT count(*) FROM project WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, body["title"]).Scan(&partial)
	if partial != 0 {
		t.Fatal("duplicate repository left a partial project")
	}
}
