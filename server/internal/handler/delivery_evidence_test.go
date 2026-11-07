package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func TestDeliveryEvidencePersistsAttributionAndRejectsStaleRevisions(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	project := dbfx.Project(t, "Delivery evidence")
	repo := dbfx.Insert(t, "project_resource", testutil.Cols{
		"project_id": project, "workspace_id": testWorkspaceID, "resource_type": "github_repo",
		"resource_ref": `{"url":"https://github.com/acme/release-evidence","ref":"main"}`,
	})
	parent := dbfx.Issue(t, "Review export", testutil.Cols{"project_id": project, "assignee_type": "member", "assignee_id": testUserID})
	handoff := workbench.Handoff{V: 2, RequirementVersion: 1, Role: "test", RepoID: repo, Expected: []string{"report"}, DependsOn: []string{}}
	properties, _ := json.Marshal(map[string]any{"workbench_handoff": handoff})
	child := dbfx.Issue(t, "Verify export", testutil.Cols{"project_id": project, "parent_issue_id": parent, "status": "done", "properties": properties})
	roles := workbench.DefaultProjectRoles()
	for _, role := range []string{"dev", "sec", "ops"} {
		roles.Roles[role] = workbench.ProjectRole{Enabled: false}
	}
	criteria := workbench.Requirements{V: 1, Version: 1, Items: []workbench.Item{{ID: "A-01", Text: "Export all rows", Checked: &workbench.Check{At: "confirmed", By: testUserID}, Covers: []string{child}}}, Bounds: []string{},
		Gate: workbench.Gates{Spec: &workbench.GateCheck{PassedAt: "confirmed", PassedBy: testUserID}},
		Plan: &workbench.ExecutionPlan{Version: 1, Issues: []string{child}},
		Team: &workbench.ProjectTeamSnapshot{ProjectRoleConfiguration: workbench.ProjectRoleConfiguration{Config: roles}},
	}
	raw, _ := json.Marshal(criteria)
	dbfx.Exec(t, `UPDATE issue SET acceptance_criteria=$1,status='in_review' WHERE id=$2`, raw, parent)
	current, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(parent))
	if err != nil {
		t.Fatal(err)
	}
	childIssue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(child))
	if err != nil {
		t.Fatal(err)
	}
	evidence := workbench.DeliveryEvidence{IssueID: child, IssueRevision: childIssue.Revision, Summary: "Export tests passed", Passed: true,
		Commit: strings.Repeat("a", 40), Reports: []workbench.DeliveryReport{{Name: "report", URL: "https://example.test/test-results"}},
		RecordedBy: "forged", Role: "dev", Version: 99,
	}
	call := func(revision int64, code int) IssueResponse {
		t.Helper()
		req := testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/delivery-evidence", map[string]any{"expected_revision": revision, "evidence": evidence}), "id", parent)
		result := testutil.Call(t, testHandler.SaveDeliveryEvidence, req).Want(code)
		var response IssueResponse
		if code == http.StatusOK {
			result.JSON(&response)
		}
		return response
	}
	saved := call(current.Revision, http.StatusOK)
	parsed, err := workbench.ParseRequirements(saved.AcceptanceCriteria)
	if err != nil {
		t.Fatal(err)
	}
	recorded := parsed.Delivery[child]
	if recorded.RecordedBy != testUserID || recorded.Role != "test" || recorded.Version != 1 || recorded.RecordedAt == "" || recorded.RepoID != repo {
		t.Fatalf("evidence attribution was not server-owned: %+v", recorded)
	}
	call(current.Revision, http.StatusConflict)
	dbfx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, child)
	call(saved.Revision, http.StatusConflict)
	var refreshed IssueResponse
	testutil.Call(t, testHandler.GetIssue, testutil.WithURLParams(newRequest("GET", "/api/issues/"+parent, nil), "id", parent)).Want(http.StatusOK).JSON(&refreshed)
	parsed, _ = workbench.ParseRequirements(refreshed.AcceptanceCriteria)
	if parsed.Delivery[child].Summary != evidence.Summary {
		t.Fatal("refresh lost the saved delivery evidence")
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM activity_log WHERE issue_id=$1 AND action='requirements_updated'`, parent).Scan(&count)
	if count != 1 {
		t.Fatalf("expected one atomic evidence audit, got %d", count)
	}
	var readiness deliveryReviewState
	read := func() {
		t.Helper()
		testutil.Call(t, testHandler.ReviewIssueDelivery, testutil.WithURLParams(newRequest("GET", "/api/issues/"+parent+"/delivery-review", nil), "id", parent)).Want(http.StatusOK).JSON(&readiness)
	}
	review := func(action, reason string, revision int64, code int) IssueResponse {
		t.Helper()
		req := testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/delivery-review", map[string]any{"expected_revision": revision, "action": action, "reason": reason}), "id", parent)
		result := testutil.Call(t, testHandler.ReviewIssueDelivery, req).Want(code)
		var response IssueResponse
		if code == http.StatusOK {
			result.JSON(&response)
		}
		return response
	}
	read()
	if len(readiness.Blockers) != 1 || readiness.Blockers[0].Code != "evidence_stale" {
		t.Fatalf("review failed to detect a stale report: %+v", readiness)
	}
	review("accept", "", saved.Revision, http.StatusConflict)
	evidence.IssueRevision++
	saved = call(saved.Revision, http.StatusOK)
	read()
	if len(readiness.Blockers) != 0 {
		t.Fatalf("fresh evidence was not ready: %+v", readiness)
	}
	dbfx.Exec(t, `UPDATE issue SET assignee_type=NULL,assignee_id=NULL WHERE id=$1`, parent)
	review("accept", "", saved.Revision, http.StatusForbidden)
	dbfx.Exec(t, `UPDATE issue SET assignee_type='member',assignee_id=$1 WHERE id=$2`, testUserID, parent)
	notice := dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID,
		"issue_id": parent, "title": "Review export", "type": "review_requested", "severity": "action_required",
		"details": `{"source":"workbench_delivery"}`,
	})
	mention := dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": testUserID,
		"issue_id": parent, "title": "Export mention", "type": "mentioned", "severity": "info",
	})
	accepted := review("accept", "", saved.Revision, http.StatusOK)
	if accepted.Status != "done" {
		t.Fatal("acceptance did not complete the task")
	}
	var noticeRead, mentionRead bool
	dbfx.QueryRow(t, "SELECT read FROM inbox_item WHERE id=$1", notice).Scan(&noticeRead)
	dbfx.QueryRow(t, "SELECT read FROM inbox_item WHERE id=$1", mention).Scan(&mentionRead)
	if !noticeRead || mentionRead {
		t.Fatal("acceptance must read its pending notification while preserving unrelated mentions")
	}
	review("accept", "", saved.Revision, http.StatusConflict)
	readRelease := func() workbench.ReleaseReadiness {
		t.Helper()
		var result workbench.ReleaseReadiness
		req := testutil.WithURLParams(newRequest("GET", "/api/issues/"+parent+"/release-readiness", nil), "id", parent)
		testutil.Call(t, testHandler.GetIssueReleaseReadiness, req).Want(http.StatusOK).JSON(&result)
		return result
	}
	if result := readRelease(); len(result.Blockers) != 1 || result.Blockers[0].Code != "merge_missing" {
		t.Fatalf("missing merge should block release: %+v", result)
	}
	pr := dbfx.Insert(t, "github_pull_request", testutil.Cols{
		"workspace_id": testWorkspaceID, "installation_id": int64(12345), "repo_owner": "acme", "repo_name": "release-evidence", "pr_number": 1,
		"title": "Release evidence", "state": "merged", "html_url": "https://github.com/acme/release-evidence/pull/1",
		"head_sha": evidence.Commit, "base_branch": "main", "merge_commit_sha": strings.Repeat("b", 40),
		"pr_created_at": "2026-11-07T00:00:00Z", "pr_updated_at": "2026-11-07T00:00:00Z",
	})
	dbfx.Exec(t, `INSERT INTO issue_pull_request (issue_id,pull_request_id) VALUES ($1,$2)`, child, pr)
	if result := readRelease(); len(result.Blockers) != 0 || len(result.Merges) != 1 || result.Merges[0].MergeCommit != strings.Repeat("b", 40) {
		t.Fatalf("accepted source and merged child should match: %+v", result)
	}
	review("undo", "", accepted.Revision, http.StatusConflict)
	t.Cleanup(func() {
		dbfx.Exec(t, `DELETE FROM workbench_release_reservations WHERE release_id IN (SELECT id FROM workbench_release_orders WHERE project_id=$1)`, project)
		dbfx.Exec(t, `DELETE FROM workbench_release_orders WHERE project_id=$1`, project)
	})
	input := workbench.ReleaseOrderInput{ProjectID: project, Env: "staging", RequestID: uuidToString(dbid.NewV7()), Tasks: []workbench.ReleaseTaskInput{{ID: parent, ExpectedRevision: accepted.Revision}}}
	candidateCount := func(env string) int {
		t.Helper()
		var result struct {
			Candidates []json.RawMessage `json:"candidates"`
		}
		testutil.Call(t, testHandler.ListReleaseCandidates, newRequest("GET", "/api/releases/candidates?project_id="+project+"&env="+env, nil)).Want(http.StatusOK).JSON(&result)
		return len(result.Candidates)
	}
	if candidateCount("staging") != 1 {
		t.Fatal("accepted merged task missing from candidates")
	}
	createRelease := func(code int) map[string]any {
		t.Helper()
		response := testutil.Call(t, testHandler.CreateReleaseOrder, newRequest("POST", "/api/releases", input)).Want(code)
		var result map[string]any
		response.JSON(&result)
		return result
	}
	failing := *testHandler
	failing.TxStarter = rollbackOnCommitTxStarter{pool: testPool}
	testutil.Call(t, failing.CreateReleaseOrder, newRequest("POST", "/api/releases", input)).Want(http.StatusInternalServerError)
	if candidateCount("staging") != 1 {
		t.Fatal("failed transaction reserved a candidate")
	}
	agentRequest := newRequest("POST", "/api/releases", input)
	agentRequest.Header.Set("X-Actor-Source", "task_token")
	agentRequest.Header.Set("X-Agent-ID", testUserID)
	testutil.Call(t, testHandler.CreateReleaseOrder, agentRequest).Want(http.StatusForbidden)
	order := createRelease(http.StatusCreated)
	if candidateCount("staging") != 0 || candidateCount("prod") != 1 {
		t.Fatal("candidate reservations were not isolated by environment")
	}
	if replay := createRelease(http.StatusOK); replay["id"] != order["id"] {
		t.Fatal("idempotent replay created a different release")
	}
	input.Env = "prod"
	createRelease(http.StatusConflict)
	input.RequestID = uuidToString(dbid.NewV7())
	production := createRelease(http.StatusCreated)
	if production["id"] == order["id"] || production["env"] != "prod" {
		t.Fatal("cross-environment promotion did not create a separate order")
	}
	input.RequestID = uuidToString(dbid.NewV7())
	createRelease(http.StatusConflict)
	var stored map[string]any
	testutil.Call(t, testHandler.GetReleaseOrder, testutil.WithURLParams(newRequest("GET", "/api/releases/"+order["id"].(string), nil), "releaseId", order["id"].(string))).Want(http.StatusOK).JSON(&stored)
	if stored["identifier"] != order["identifier"] || stored["manifest"] == nil {
		t.Fatal("release snapshot did not survive reread")
	}
	verifyReleaseConfirmations(t, production["id"].(string), project)
	outsider := dbfx.User(t, "Release outsider", "release-outsider-"+project+"@example.test")
	outsiderRequest := testutil.WithURLParams(newRequest("GET", "/api/releases/"+order["id"].(string), nil), "releaseId", order["id"].(string))
	outsiderRequest.Header.Set("X-User-ID", outsider)
	testutil.Call(t, testHandler.GetReleaseOrder, outsiderRequest).Want(http.StatusNotFound)
	testutil.Call(t, testHandler.DeleteProject, testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+project, nil), "id", project)).Want(http.StatusConflict)
	dbfx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1 AND pull_request_id=$2`, child, pr)
	review("undo", "", accepted.Revision, http.StatusConflict)
	dbfx.Exec(t, `DELETE FROM workbench_release_reservations WHERE release_id IN (SELECT id FROM workbench_release_orders WHERE project_id=$1)`, project)
	dbfx.Exec(t, `DELETE FROM workbench_release_orders WHERE project_id=$1`, project)
	undone := review("undo", "", accepted.Revision, http.StatusOK)
	if undone.Status != "in_review" {
		t.Fatal("undo did not restore review")
	}
	review("reject", "", undone.Revision, http.StatusConflict)
	rejected := review("reject", "Fix export", undone.Revision, http.StatusOK)
	parsed, _ = workbench.ParseRequirements(rejected.AcceptanceCriteria)
	if rejected.Status != "in_progress" || parsed.Gate.Accept != nil || parsed.Delivery != nil || parsed.Review.Reason != "Fix export" {
		t.Fatal("requesting changes retained current delivery conclusions")
	}
}
