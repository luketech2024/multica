package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestRequirementsDocumentVerifiedVersionConfirmation(t *testing.T) {
	f := newSupplementFixture(t, "cursor", "running", false)
	project := dbfx.Project(t, "Readable requirements")
	var repo string
	dbfx.QueryRow(t, `INSERT INTO project_resource(project_id,workspace_id,resource_type,resource_ref,created_by) VALUES($1,$2,'git_repo','{"url":"https://example.test/prd.git"}',$3) RETURNING id`, project, testWorkspaceID, testUserID).Scan(&repo)
	testutil.Call(t, testHandler.SaveOneSpecProject, withURLParam(newRequest("PUT", "/", map[string]any{"enabled": true, "expected_revision": 0, "overview_repository_id": repo}), "id", project)).Want(200)
	dbfx.Exec(t, `UPDATE issue SET project_id=$2,assignee_type='agent',assignee_id=$3,acceptance_criteria='{"v":1,"version":1,"phase":"ready","draft_revision":1,"ready_revision":1,"items":[{"id":"A-01","text":"Serve the homepage","checked":{"at":"","by":""}}],"bounds":[]}' WHERE id=$1`, f.issueID, project, f.agentID)
	issue, err := testHandler.Queries.GetIssue(t.Context(), parseUUID(f.issueID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.InitializeOneSpecFlow(t.Context(), testHandler.Queries, issue, "feature"); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM onespec_flow WHERE issue_id=$1`, f.issueID)
	var state oneSpecResponse
	read := func() {
		t.Helper()
		testutil.Call(t, testHandler.GetOneSpecFlow, withURLParam(newRequest("GET", "/", nil), "id", f.issueID)).Want(200).JSON(&state)
	}
	read()
	content := "# Homepage\nServe the homepage.\n"
	digest := sha256.Sum256([]byte(content))
	ref := workbench.OneSpecArtifactRef{RepositoryID: repo, Path: "onespec/changes/task/prd.md", Commit: strings.Repeat("a", 40), Digest: hex.EncodeToString(digest[:])}
	artifact := workbench.OneSpecArtifact{Stage: "requirements", Summary: "Homepage", Refs: []workbench.OneSpecArtifactRef{ref}, Document: &workbench.RequirementsDocument{Content: content, Ref: ref, Verified: true}}
	submit := func(agent bool, status int) {
		t.Helper()
		req := withURLParams(newRequest("POST", "/", map[string]any{"expected_revision": state.Revision, "artifact": artifact}), "id", f.issueID, "action", "artifact")
		if agent {
			req.Header.Set("X-Actor-Source", "task_token")
			req.Header.Set("X-Agent-ID", f.agentID)
			req.Header.Set("X-Task-ID", f.taskID)
		}
		testutil.Call(t, testHandler.UpdateOneSpecFlow, req).Want(status)
		read()
	}
	submit(false, 403)
	submit(true, 200)
	decision := func(status int) {
		t.Helper()
		testutil.Call(t, testHandler.UpdateOneSpecFlow, withURLParams(newRequest("POST", "/", map[string]any{"expected_revision": state.Revision, "stage": "requirements", "version": 1, "approved": true}), "id", f.issueID, "action", "decision")).Want(status)
	}
	decision(409)
	verify := func(version int, message string, status int) {
		t.Helper()
		testutil.Call(t, testHandler.VerifyRequirementsDocument, withURLParam(newOwnerDaemonRequest("POST", "/", map[string]any{"version": version, "error": message}), "taskId", f.taskID)).Want(status)
		read()
	}
	verify(99, "", 409)
	verify(1, "file differs", 200)
	decision(409)
	verify(1, "", 200)
	issue, _ = testHandler.Queries.GetIssue(t.Context(), parseUUID(f.issueID))
	criteria, _ := workbench.ParseRequirements(issue.AcceptanceCriteria)
	if criteria.Items[0].Checked != nil {
		t.Fatal("new document retained a prior item check")
	}
	criteria.Items[0].Checked = &workbench.Check{}
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/", map[string]any{"expected_revision": issue.Revision, "acceptance_criteria": criteria}), "id", f.issueID)).Want(200)
	decision(200)
	read()
	testutil.Call(t, testHandler.UpdateOneSpecFlow, withURLParams(newRequest("POST", "/", map[string]any{"expected_revision": state.Revision}), "id", f.issueID, "action", "advance")).Want(200)
	read()
	if state.Flow.Phase != "design" {
		t.Fatal("confirmed feature skipped design")
	}
}
