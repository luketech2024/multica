package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestOneSpecWorkflowHTTP(t *testing.T) {
	if testHandler == nil {
		t.Fatal("database required")
	}
	project := dbfx.Project(t, "OneSpec documentation")
	config := testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"enabled": true, "expected_revision": 0}), "id", project)
	testutil.Call(t, testHandler.SaveOneSpecProject, config).Want(http.StatusOK)
	var repoID string
	dbfx.QueryRow(t, `INSERT INTO project_resource (project_id,workspace_id,resource_type,resource_ref,created_by) VALUES ($1,$2,'git_repo','{"url":"https://example.test/repo.git"}',$3) RETURNING id`, project, testWorkspaceID, testUserID).Scan(&repoID)
	issue := createIssueWithPropertyBody(t, map[string]any{"title": "Document upstream changes", "project_id": project})
	var state oneSpecResponse
	get := func() {
		t.Helper()
		testutil.Call(t, testHandler.GetOneSpecFlow, testutil.WithURLParams(newRequest("GET", "/", nil), "id", issue.ID)).Want(200).JSON(&state)
	}
	get()
	if state.Flow == nil || !state.CanConfirm {
		t.Fatal("creation did not capture workflow and confirmer")
	}
	call := func(action string, input map[string]any, status int) {
		t.Helper()
		input["expected_revision"] = state.Revision
		req := testutil.WithURLParams(newRequest("POST", "/", input), "id", issue.ID, "action", action)
		response := testutil.Call(t, testHandler.UpdateOneSpecFlow, req).Want(status)
		if status == 200 {
			response.JSON(&state)
		}
	}
	testutil.Call(t, testHandler.SaveOneSpecProject, testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"enabled": true, "expected_revision": 1, "overview_repository_id": repoID}), "id", project)).Want(200)
	call("refresh-configuration", map[string]any{}, 200)
	if state.Flow.OverviewRepositoryID != repoID || state.Flow.Generation != 1 {
		t.Fatal("unstarted workflow did not refresh its configuration")
	}
	call("kind", map[string]any{"kind": "documentation"}, 200)
	call("advance", map[string]any{}, 409)
	for _, stage := range []string{"requirements", "branch", "documentation", "testing", "merge", "merging"} {
		artifact := workbench.OneSpecArtifact{Stage: stage, Summary: "Verified " + stage, Refs: []workbench.OneSpecArtifactRef{{RepositoryID: repoID, Path: "onespec/changes/documentation.md", Commit: strings.Repeat("a", 40), Digest: strings.Repeat("b", 64)}}}
		call("artifact", map[string]any{"artifact": artifact}, 200)
		call("refresh-configuration", map[string]any{}, 409)
		if workbench.OneSpecNeedsConfirmation(stage) {
			call("decision", map[string]any{"stage": stage, "version": 99, "approved": true}, 409)
			call("decision", map[string]any{"stage": stage, "version": 1, "approved": true}, 200)
		}
		call("advance", map[string]any{}, 200)
	}
	if state.Flow.Phase != "acceptance" || state.Flow.Status != "awaiting_confirmation" {
		t.Fatalf("acceptance not prepared: %+v", state.Flow)
	}
	call("decision", map[string]any{"stage": "acceptance", "version": 1, "approved": true}, 200)
	call("advance", map[string]any{}, 200)
	var status string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, issue.ID).Scan(&status)
	if status != "done" || state.Flow.Status != "completed" {
		t.Fatalf("workflow did not complete: %s %+v", status, state.Flow)
	}
	testutil.Call(t, testHandler.SaveOneSpecProject, testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"enabled": false, "expected_revision": 0}), "id", project)).Want(409)
}

func TestOneSpecConfirmedPlanAndRevision(t *testing.T) {
	project := dbfx.Project(t, "OneSpec plan")
	testutil.Call(t, testHandler.SaveOneSpecProject, testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"enabled": true, "expected_revision": 0}), "id", project)).Want(200)
	var repoID string
	dbfx.QueryRow(t, `INSERT INTO project_resource (project_id,workspace_id,resource_type,resource_ref,created_by) VALUES ($1,$2,'git_repo','{"url":"https://example.test/repo.git"}',$3) RETURNING id`, project, testWorkspaceID, testUserID).Scan(&repoID)
	issue := createIssueWithPropertyBody(t, map[string]any{"title": "Implement verified change", "project_id": project, "acceptance_criteria": map[string]any{"v": 1, "items": []map[string]any{{"text": "The report includes every row"}}}})
	var state oneSpecResponse
	get := func() {
		t.Helper()
		testutil.Call(t, testHandler.GetOneSpecFlow, testutil.WithURLParams(newRequest("GET", "/", nil), "id", issue.ID)).Want(200).JSON(&state)
	}
	get()
	call := func(action string, input map[string]any, status int) {
		t.Helper()
		input["expected_revision"] = state.Revision
		testutil.Call(t, testHandler.UpdateOneSpecFlow, testutil.WithURLParams(newRequest("POST", "/", input), "id", issue.ID, "action", action)).Want(status)
		get()
	}
	artifact := func(stage string, tasks []workbench.PlanTask) workbench.OneSpecArtifact {
		return workbench.OneSpecArtifact{Stage: stage, Summary: "Verified " + stage, Tasks: tasks, Refs: []workbench.OneSpecArtifactRef{{RepositoryID: repoID, Path: "onespec/changes/test/" + stage + ".md", Commit: strings.Repeat("a", 40), Digest: strings.Repeat("b", 64)}}}
	}
	for _, stage := range []string{"requirements", "design"} {
		call("artifact", map[string]any{"artifact": artifact(stage, nil)}, 200)
		call("decision", map[string]any{"stage": stage, "version": 1, "approved": true}, 200)
		call("advance", map[string]any{}, 200)
	}
	tasks := []workbench.PlanTask{{Key: "dev", Title: "Implement report", Role: "dev", RepoID: repoID, Covers: []string{"A-01"}, DependsOn: []string{}, Expected: []string{"verification"}}}
	call("artifact", map[string]any{"artifact": artifact("plan", tasks)}, 200)
	apply := func(payload []workbench.PlanTask, status int) {
		t.Helper()
		var revision int64
		dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, issue.ID).Scan(&revision)
		testutil.Call(t, testHandler.SaveExecutionPlan, testutil.WithURLParams(newRequest("POST", "/", map[string]any{"expected_revision": revision, "version": 1, "idempotency_key": "onespec-plan", "tasks": payload}), "id", issue.ID)).Want(status)
	}
	apply(tasks, 409)
	call("decision", map[string]any{"stage": "plan", "version": 1, "approved": true}, 200)
	changed := append([]workbench.PlanTask{}, tasks...)
	changed[0].Title = "Unconfirmed extra scope"
	apply(changed, 409)
	apply(tasks, 200)
	apply(tasks, 200)
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, issue.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("plan retry created %d children", count)
	}
	call("advance", map[string]any{}, 200)
	if state.Flow.Phase != "branch" {
		t.Fatal("Git preparation was skipped")
	}
	call("reopen", map[string]any{"stage": "design"}, 200)
	if state.Flow.Approved("plan") {
		t.Fatal("old plan confirmation survived design revision")
	}
	apply(tasks, 409)
}

func TestOneSpecTeamReuseAndRuntimeSnapshot(t *testing.T) {
	runtime := handlerTestRuntimeID(t)
	var defaults oneSpecConfiguration
	testutil.Call(t, testHandler.GetOneSpecWorkspace, testutil.WithURLParams(newRequest("GET", "/", nil), "id", testWorkspaceID)).Want(200).JSON(&defaults)
	testutil.Call(t, testHandler.SaveOneSpecWorkspace, testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"default_runtime_id": runtime, "expected_revision": defaults.Revision}), "id", testWorkspaceID)).Want(200)
	first := dbfx.Project(t, "OneSpec team first")
	second := dbfx.Project(t, "OneSpec team second")
	var firstTeam, secondTeam ProjectRolesResponse
	for _, project := range []string{first, second} {
		testutil.Call(t, testHandler.SaveOneSpecProject, testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"enabled": true, "expected_revision": 0}), "id", project)).Want(200)
	}
	testutil.Call(t, testHandler.ProvisionOneSpecTeam, testutil.WithURLParams(newRequest("POST", "/", nil), "id", first)).Want(200).JSON(&firstTeam)
	testutil.Call(t, testHandler.ProvisionOneSpecTeam, testutil.WithURLParams(newRequest("POST", "/", nil), "id", second)).Want(200).JSON(&secondTeam)
	if *firstTeam.Config.Roles["dev"].AgentID != *secondTeam.Config.Roles["dev"].AgentID {
		t.Fatal("projects did not reuse platform agents")
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent WHERE workspace_id=$1 AND runtime_id=$2 AND system_key LIKE 'onespec:%'`, testWorkspaceID, runtime).Scan(&count)
	if count != 7 {
		t.Fatalf("expected seven platform roles, got %d", count)
	}
	issue := createIssueWithPropertyBody(t, map[string]any{"title": "Keep runtime snapshot", "project_id": first})
	testutil.Call(t, testHandler.SaveOneSpecProject, testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"enabled": false, "expected_revision": 1}), "id", first)).Want(200)
	var state oneSpecResponse
	testutil.Call(t, testHandler.GetOneSpecFlow, testutil.WithURLParams(newRequest("GET", "/", nil), "id", issue.ID)).Want(200).JSON(&state)
	if state.Flow.RuntimeID != runtime || *state.Flow.Team.Config.Roles["dev"].AgentID != *firstTeam.Config.Roles["dev"].AgentID {
		t.Fatal("existing task snapshot changed")
	}
}

func TestOneSpecConfirmationAuthority(t *testing.T) {
	project := dbfx.Project(t, "OneSpec confirmation authority")
	testutil.Call(t, testHandler.SaveOneSpecProject, testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"enabled": true, "expected_revision": 0}), "id", project)).Want(200)
	issue := createIssueWithPropertyBody(t, map[string]any{"title": "Confirm as creator or lead", "project_id": project})
	other := dbfx.User(t, "Other confirmer", "onespec-confirm@example.test")
	dbfx.Member(t, testWorkspaceID, other, "member")
	var state oneSpecResponse
	testutil.Call(t, testHandler.GetOneSpecFlow, testutil.WithURLParams(newRequest("GET", "/", nil), "id", issue.ID)).Want(200).JSON(&state)
	request := testutil.WithURLParams(newRequest("POST", "/", map[string]any{"expected_revision": state.Revision, "kind": "documentation"}), "id", issue.ID, "action", "kind")
	request.Header.Set("X-User-ID", other)
	testutil.Call(t, testHandler.UpdateOneSpecFlow, request).Want(403)
	dbfx.Exec(t, `UPDATE project SET lead_type='member',lead_id=$1 WHERE id=$2`, other, project)
	request = testutil.WithURLParams(newRequest("POST", "/", map[string]any{"expected_revision": state.Revision, "kind": "documentation"}), "id", issue.ID, "action", "kind")
	request.Header.Set("X-User-ID", other)
	testutil.Call(t, testHandler.UpdateOneSpecFlow, request).Want(200)
}
