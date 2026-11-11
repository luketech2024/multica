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

func TestApplicationDeploymentPersistenceAndExistingSourceRelease(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	project := dbfx.Project(t, "Application deployment")
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM project_deployment WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, project)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM workbench_release_orders WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, project)
	})
	appID := uuidToString(dbid.NewV7())
	config := workbench.EmptyProjectDeployment()
	config.Applications = []workbench.DeploymentApplication{{ID: appID, Name: "Website", SourceKind: "git", Source: "https://example.test/website.git", ArtifactPath: "dist", Environments: map[string]workbench.ApplicationDeployment{}}}
	request := func(revision int64, status int) {
		t.Helper()
		testutil.Call(t, testHandler.SaveProjectDeployment, testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project+"/deployment", map[string]any{"config": config, "expected_revision": revision}), "id", project)).Want(status)
	}
	request(0, 200)
	request(0, 409)
	config.Applications[0].Name = "Updated website"
	request(1, 200)
	request(1, 409)
	var saved struct {
		Config   workbench.ProjectDeploymentConfig `json:"config"`
		Revision int64                             `json:"revision"`
	}
	testutil.Call(t, testHandler.GetProjectDeployment, testutil.WithURLParams(newRequest("GET", "/api/projects/"+project+"/deployment", nil), "id", project)).Want(200).JSON(&saved)
	if saved.Revision != 2 || saved.Config.Applications[0].Name != "Updated website" {
		t.Fatalf("configuration did not persist: %+v", saved)
	}
	change := workbench.ReleaseChange{ID: uuidToString(dbid.NewV7()), Type: "app", Name: "Website", ApplicationID: appID, Version: "v1"}
	input := workbench.ReleaseOrderInput{ProjectID: project, Env: "staging", RequestID: uuidToString(dbid.NewV7()), Delivery: &workbench.ApplicationRelease{Title: "Publish existing code", Mode: "auto", Changes: []workbench.ReleaseChange{change}}}
	var created struct {
		ID       string                    `json:"id"`
		Revision int64                     `json:"revision"`
		Manifest workbench.ReleaseManifest `json:"manifest"`
	}
	testutil.Call(t, testHandler.CreateReleaseOrder, newRequest("POST", "/api/releases", input)).Want(201).JSON(&created)
	if created.Manifest.Delivery == nil || len(created.Manifest.Tasks) != 0 {
		t.Fatal("existing sources incorrectly require tasks")
	}
	testutil.Call(t, testHandler.CreateReleaseOrder, newRequest("POST", "/api/releases", input)).Want(200)
	read := testutil.WithURLParams(newRequest("GET", "/api/releases/"+created.ID, nil), "releaseId", created.ID)
	var detail struct {
		Gates []workbench.ReleaseGate `json:"gates"`
	}
	testutil.Call(t, testHandler.GetReleaseOrder, read).Want(200).JSON(&detail)
	for _, gate := range detail.Gates {
		if gate.ID == "targets" && gate.Passed {
			t.Fatal("unconfigured application passed deployment checks")
		}
	}
	testutil.Call(t, testHandler.ExecuteApplicationRelease, testutil.WithURLParams(newRequest("POST", "/api/releases/"+created.ID+"/execute", map[string]any{"expected_revision": created.Revision}), "releaseId", created.ID)).Want(http.StatusConflict)
	input.Delivery.Title = "Revised release"
	update := func(revision int64, status int) {
		t.Helper()
		testutil.Call(t, testHandler.UpdateApplicationRelease, testutil.WithURLParams(newRequest("PUT", "/api/releases/"+created.ID+"/content", map[string]any{"delivery": input.Delivery, "expected_revision": revision}), "releaseId", created.ID)).Want(status)
	}
	update(created.Revision, 200)
	update(created.Revision, 409)
	var raw []byte
	dbfx.QueryRow(t, `SELECT manifest FROM workbench_release_orders WHERE workspace_id=$1 AND id=$2`, testWorkspaceID, created.ID).Scan(&raw)
	var manifest workbench.ReleaseManifest
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest.Delivery.Title != "Revised release" {
		t.Fatal("release edit did not persist")
	}
}
