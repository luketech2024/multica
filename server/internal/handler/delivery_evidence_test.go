package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestDeliveryEvidencePersistsAttributionAndRejectsStaleRevisions(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	project := dbfx.Project(t, "Delivery evidence")
	parent := dbfx.Issue(t, "Review export", testutil.Cols{"project_id": project, "assignee_type": "member", "assignee_id": testUserID})
	handoff := workbench.Handoff{V: 2, RequirementVersion: 1, Role: "test", RepoID: "repository", Expected: []string{"report"}, DependsOn: []string{}}
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
	if recorded.RecordedBy != testUserID || recorded.Role != "test" || recorded.Version != 1 || recorded.RecordedAt == "" || recorded.RepoID != "repository" {
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
	accepted := review("accept", "", saved.Revision, http.StatusOK)
	if accepted.Status != "done" {
		t.Fatal("acceptance did not complete the task")
	}
	review("accept", "", saved.Revision, http.StatusConflict)
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
