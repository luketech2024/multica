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

func TestLegacyDeliveryRecoveryRequiresEvidenceAndKeepsReleaseBlockersVisible(t *testing.T) {
	project := dbfx.Project(t, "Recovered delivery")
	repo := dbfx.Insert(t, "project_resource", testutil.Cols{"project_id": project, "workspace_id": testWorkspaceID, "resource_type": "github_repo", "resource_ref": `{"url":"https://example.test/site.git","ref":"main"}`})
	roles := workbench.DefaultProjectRoles()
	for _, role := range []string{"test", "sec", "ops"} {
		roles.Roles[role] = workbench.ProjectRole{Enabled: false}
	}
	config, _ := json.Marshal(workbench.ProjectRoleConfiguration{Config: roles, Revision: 1})
	dbfx.Insert(t, "project_resource", testutil.Cols{"project_id": project, "workspace_id": testWorkspaceID, "resource_type": "workbench_roles", "resource_ref": config})
	parent := dbfx.Issue(t, "Legacy completed task", testutil.Cols{"project_id": project, "status": "done", "acceptance_criteria": []byte(`[]`)})
	child := dbfx.Issue(t, "Implemented site", testutil.Cols{"project_id": project, "parent_issue_id": parent, "status": "done"})
	ctx := context.Background()
	current, _ := testHandler.Queries.GetIssue(ctx, parseUUID(parent))
	childIssue, _ := testHandler.Queries.GetIssue(ctx, parseUUID(child))
	var review deliveryReviewState
	testutil.Call(t, testHandler.ReviewIssueDelivery, testutil.WithURLParams(newRequest("GET", "/api/issues/"+parent+"/delivery-review", nil), "id", parent)).Want(http.StatusOK).JSON(&review)
	if len(review.Blockers) != 1 || review.Blockers[0].Code != "plan" {
		t.Fatalf("legacy review did not expose missing plan: %+v", review)
	}
	var candidates struct {
		Candidates []json.RawMessage `json:"candidates"`
		Blocked    []struct {
			ID       string                     `json:"id"`
			Blockers []workbench.ReleaseBlocker `json:"blockers"`
		} `json:"blocked"`
	}
	testutil.Call(t, testHandler.ListReleaseCandidates, newRequest("GET", "/api/releases/candidates?project_id="+project+"&env=staging", nil)).Want(http.StatusOK).JSON(&candidates)
	if len(candidates.Candidates) != 0 || len(candidates.Blocked) != 1 || candidates.Blocked[0].ID != parent || candidates.Blocked[0].Blockers[0].Code != "acceptance" {
		t.Fatalf("legacy task was silently filtered: %+v", candidates)
	}
	input := map[string]any{"action": "recover", "expected_revision": current.Revision, "recovery": map[string]any{
		"items": []string{"Homepage displays the contact email"}, "tasks": []map[string]any{{"issue_id": child, "revision": childIssue.Revision, "role": "dev", "repo_id": repo, "covers": []string{"A-01"}}},
	}}
	request := func() *http.Request {
		return testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/delivery-review", input), "id", parent)
	}
	agent := request()
	agent.Header.Set("X-Actor-Source", "task_token")
	agent.Header.Set("X-Agent-ID", testUserID)
	testutil.Call(t, testHandler.ReviewIssueDelivery, agent).Want(http.StatusForbidden)
	failing := *testHandler
	failing.TxStarter = rollbackOnCommitTxStarter{pool: testPool}
	testutil.Call(t, failing.ReviewIssueDelivery, request()).Want(http.StatusInternalServerError)
	unchanged, _ := testHandler.Queries.GetIssue(ctx, parseUUID(child))
	if unchanged.Revision != childIssue.Revision {
		t.Fatal("failed recovery changed the child")
	}
	var recovered IssueResponse
	testutil.Call(t, testHandler.ReviewIssueDelivery, request()).Want(http.StatusOK).JSON(&recovered)
	criteria, _ := workbench.ParseRequirements(recovered.AcceptanceCriteria)
	if recovered.Status != "in_review" || criteria.Gate.Accept != nil || len(criteria.Plan.Issues) != 1 || len(criteria.Delivery) != 0 {
		t.Fatalf("recovery fabricated acceptance or evidence: %+v", criteria)
	}
	if !workbench.RequirementsReady(criteria) || criteria.Gate.Spec == nil || criteria.Gate.Spec.PassedBy != testUserID {
		t.Fatal("recovery did not record the member's scope confirmation")
	}
	testutil.Call(t, testHandler.ReviewIssueDelivery, request()).Want(http.StatusConflict)
	testutil.Call(t, testHandler.ReviewIssueDelivery, testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/delivery-review", map[string]any{"action": "accept", "expected_revision": recovered.Revision}), "id", parent)).Want(http.StatusConflict)
	childIssue, _ = testHandler.Queries.GetIssue(ctx, parseUUID(child))
	evidence := workbench.DeliveryEvidence{IssueID: child, IssueRevision: childIssue.Revision, Commit: strings.Repeat("a", 40), Passed: true, Summary: "Contact link checked in browser", Reports: []workbench.DeliveryReport{{Name: "delivery", URL: "https://example.test/reports/contact"}}, Changes: &workbench.DeliveryChanges{Files: 1, Additions: 1, Deletions: 1}, PreviewURL: "https://preview.example.test/contact"}
	var saved IssueResponse
	testutil.Call(t, testHandler.SaveDeliveryEvidence, testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/delivery-evidence", map[string]any{"expected_revision": recovered.Revision, "evidence": evidence}), "id", parent)).Want(http.StatusOK).JSON(&saved)
	testutil.Call(t, testHandler.ReviewIssueDelivery, testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/delivery-review", map[string]any{"action": "accept", "expected_revision": saved.Revision}), "id", parent)).Want(http.StatusOK)
}
