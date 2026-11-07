package handler

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
)

func deployConfigFixture(t *testing.T) (string, string) {
	t.Helper()
	project := dbfx.Project(t, "Deployment configuration")
	repo := dbfx.Insert(t, "project_resource", testutil.Cols{"project_id": project, "workspace_id": testWorkspaceID, "resource_type": "github_repo", "resource_ref": []byte(`{"url":"https://example.com/repo.git"}`)})
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM deploy_targets WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, project)
	})
	return project, repo
}

func TestDeployTargetDraftRevisionAndProjectIsolation(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	project, repo := deployConfigFixture(t)
	input := map[string]any{"label": "Staging node", "repo_id": repo, "env": "staging", "kind": "docker-host", "channel": "runtime", "location": map[string]string{}, "last_check": map[string]any{"ok": true}}
	request := func(target string, body any, status int) DeployTargetResponse {
		t.Helper()
		req := testutil.WithURLParams(newRequest("POST", "/api/projects/"+project+"/deploy-targets", body), "id", project, "targetId", target)
		result := testutil.Call(t, testHandler.SaveDeployTarget, req).Want(status)
		var response DeployTargetResponse
		if status == http.StatusOK {
			result.JSON(&response)
		}
		return response
	}
	created := request("", input, http.StatusOK)
	if len(created.MissingFields) != 5 || len(created.LastCheck) > 0 && string(created.LastCheck) != "null" {
		t.Fatalf("draft incorrectly ready: %+v", created)
	}
	request("", input, http.StatusConflict)
	input["expected_revision"] = created.Revision
	input["label"] = "Second name"
	updated := request(created.ID, input, http.StatusOK)
	if updated.Revision != created.Revision+1 {
		t.Fatal("revision did not advance")
	}
	request(created.ID, input, http.StatusConflict)
	otherProject, otherRepo := deployConfigFixture(t)
	_ = otherProject
	input["repo_id"] = otherRepo
	request("", input, http.StatusBadRequest)
	var list struct {
		Targets []DeployTargetResponse `json:"targets"`
	}
	testutil.Call(t, testHandler.ListDeployTargets, testutil.WithURLParams(newRequest("GET", "/api/projects/"+project+"/deploy-targets", nil), "id", project)).Want(http.StatusOK).JSON(&list)
	if len(list.Targets) != 1 || list.Targets[0].Label != "Second name" {
		t.Fatalf("saved target not reloaded: %+v", list)
	}
	testutil.Call(t, testHandler.DeleteProjectResource, testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+project+"/resources/"+repo, nil), "id", project, "resourceId", repo)).Want(http.StatusConflict)
}

func TestWorkbenchCredentialEncryptionRotationAndReferenceGuard(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	box, err := secretbox.New(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	h := *testHandler
	h.WorkbenchSecretBox = box
	project, repo := deployConfigFixture(t)
	var credential struct {
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
		LastFour string `json:"last_four"`
	}
	input := map[string]any{"label": "Deployment test", "kind": "registry-token", "usage": []string{"deploy"}, "secret": "test-secret-1234", "project_id": project}
	testutil.Call(t, h.SaveWorkbenchCredential, newRequest("POST", "/api/workbench/credentials", input)).Want(http.StatusOK).JSON(&credential)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM deploy_targets WHERE workspace_id=$1 AND credential_id=$2`, testWorkspaceID, credential.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM credentials WHERE workspace_id=$1 AND id=$2`, testWorkspaceID, credential.ID)
	})
	if credential.LastFour != "1234" {
		t.Fatal("missing masked suffix")
	}
	var encrypted, wrapped []byte
	dbfx.QueryRow(t, `SELECT secret_enc,wrapped_key FROM credentials WHERE id=$1`, credential.ID).Scan(&encrypted, &wrapped)
	if bytes.Contains(encrypted, []byte("test-secret-1234")) || len(wrapped) == 0 {
		t.Fatal("credential not envelope encrypted")
	}
	var target DeployTargetResponse
	create := testutil.WithURLParams(newRequest("POST", "/api/projects/"+project+"/deploy-targets", map[string]any{"label": "Registry", "repo_id": repo, "env": "prod", "kind": "npm-package", "channel": "platform", "credential_id": credential.ID, "location": map[string]string{"registry": "https://registry.example.com"}}), "id", project)
	testutil.Call(t, h.SaveDeployTarget, create).Want(http.StatusOK).JSON(&target)
	wrongType := testutil.WithURLParams(newRequest("POST", "/api/projects/"+project+"/deploy-targets", map[string]any{"label": "SSH", "repo_id": repo, "env": "staging", "kind": "docker-host", "channel": "ssh", "credential_id": credential.ID, "location": map[string]string{}}), "id", project)
	testutil.Call(t, h.SaveDeployTarget, wrongType).Want(http.StatusBadRequest)
	deleteReq := testutil.WithURLParams(newRequest("DELETE", "/api/workbench/credentials/"+credential.ID+"?expected_revision="+strconv.FormatInt(credential.Revision, 10), nil), "credentialId", credential.ID)
	testutil.Call(t, h.DeleteWorkbenchCredential, deleteReq).Want(http.StatusConflict)
	var leaked bool
	dbfx.QueryRow(t, `SELECT EXISTS(SELECT 1 FROM activity_log WHERE workspace_id=$1 AND details::text LIKE '%test-secret-1234%')`, testWorkspaceID).Scan(&leaked)
	if leaked {
		t.Fatal("audit leaked credential")
	}
	rotate := func(revision int64, status int) {
		t.Helper()
		req := testutil.WithURLParams(newRequest("PUT", "/api/workbench/credentials/"+credential.ID, map[string]any{"secret": "rotated-secret-5678", "expected_revision": revision}), "credentialId", credential.ID)
		testutil.Call(t, h.SaveWorkbenchCredential, req).Want(status)
	}
	notice := dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID,
		"title": "Rotate deployment key", "type": "review_requested", "severity": "action_required",
		"details": `{"source":"workbench_credential","credential_id":"` + credential.ID + `"}`,
	})
	rotate(credential.Revision, http.StatusOK)
	var archived bool
	dbfx.QueryRow(t, "SELECT archived FROM inbox_item WHERE id=$1", notice).Scan(&archived)
	if !archived {
		t.Fatal("rotation left the old expiry reminder pending")
	}
	rotate(credential.Revision, http.StatusConflict)
	var targetRevision int64
	dbfx.QueryRow(t, `SELECT revision FROM deploy_targets WHERE id=$1`, target.ID).Scan(&targetRevision)
	if targetRevision != target.Revision+1 {
		t.Fatal("rotation did not invalidate target checks")
	}
	shortSecret := testutil.WithURLParams(newRequest("PUT", "/api/workbench/credentials/"+credential.ID, map[string]any{"secret": "123", "expected_revision": credential.Revision + 1}), "credentialId", credential.ID)
	testutil.Call(t, h.SaveWorkbenchCredential, shortSecret).Want(http.StatusOK).JSON(&credential)
	if credential.LastFour != "" {
		t.Fatal("short credential exposed through masked suffix")
	}
}

func TestWorkbenchCredentialsRequireConfiguredEncryption(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	h := *testHandler
	h.WorkbenchSecretBox = nil
	testutil.Call(t, h.SaveWorkbenchCredential, newRequest("POST", "/api/workbench/credentials", map[string]string{"secret": "test-only-secret"})).Want(http.StatusServiceUnavailable)
}
