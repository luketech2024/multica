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

func TestRequirementsDiscussionCreation(t *testing.T) {
	project := dbfx.Project(t, "Discussion creation")
	agent := dbfx.Agent(t, "Discussion owner", testRuntimeID)
	for _, description := range []string{"", "Deploy the existing website in a test environment and verify the homepage"} {
		issue := createIssueWithPropertyBody(t, map[string]any{"title": "Discuss " + description, "description": description, "project_id": project, "assignee_type": "agent", "assignee_id": agent})
		dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id=$1`, issue.ID)
		criteria, err := workbench.ParseRequirements(issue.AcceptanceCriteria)
		if err != nil || criteria.Phase != "clarifying" || len(criteria.Items) != 0 {
			t.Fatal("creation skipped discussion", err, criteria)
		}
		var count int
		dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue.ID).Scan(&count)
		if description == "" && count != 0 {
			t.Fatal("title-only creation invoked the explicit agent")
		}
		if description != "" && count != 1 {
			t.Fatalf("initial content queued %d runs, want one", count)
		}
		if issue.AssigneeType == nil || *issue.AssigneeType != "agent" {
			t.Fatal("explicit discussion assignment lost")
		}
	}
}

func TestRequirementsDiscussionCommentCoverage(t *testing.T) {
	project := dbfx.Project(t, "Discussion revision")
	criteria := workbench.Requirements{V: 1, Version: 1, Phase: "ready", DraftRevision: 1, ReadyRevision: 1,
		Items: []workbench.Item{{ID: "A-01", Text: "Open the homepage", Checked: &workbench.Check{By: testUserID, At: "now"}}},
		Team:  &workbench.ProjectTeamSnapshot{ProjectID: project}}
	raw, _ := json.Marshal(criteria)
	id := dbfx.Issue(t, "Review homepage", testutil.Cols{"project_id": project, "acceptance_criteria": raw})
	read := func() (workbench.Requirements, int64) {
		t.Helper()
		issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(id))
		if err != nil {
			t.Fatal(err)
		}
		c, err := workbench.ParseRequirements(issue.AcceptanceCriteria)
		if err != nil {
			t.Fatal(err)
		}
		return c, issue.Revision
	}
	_, oldRevision := read()
	var comment CommentResponse
	testutil.Call(t, testHandler.CreateComment, testutil.WithURLParams(newRequest("POST", "/", map[string]any{"content": "Use a test environment"}), "id", id)).Want(201).JSON(&comment)
	current, revision := read()
	if current.DiscussionRevision != 1 || workbench.RequirementsReady(current) {
		t.Fatal("message did not invalidate confirmation", current)
	}
	testutil.Call(t, testHandler.UpdateIssue, testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"expected_revision": oldRevision, "confirm_requirements": true}), "id", id)).Want(http.StatusConflict)
	testutil.Call(t, testHandler.UpdateIssue, testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"expected_revision": revision, "confirm_requirements": true}), "id", id)).Want(http.StatusBadRequest)
	testutil.Call(t, testHandler.UpdateComment, testutil.WithURLParams(newRequest("PUT", "/", map[string]any{"content": "Use staging instead"}), "id", id, "commentId", comment.ID)).Want(200)
	current, _ = read()
	if current.DiscussionRevision != 2 {
		t.Fatalf("edit revision = %d", current.DiscussionRevision)
	}
	testutil.Call(t, testHandler.DeleteComment, testutil.WithURLParams(newRequest("DELETE", "/", nil), "id", id, "commentId", comment.ID)).Want(204)
	current, _ = read()
	if current.DiscussionRevision != 3 {
		t.Fatalf("delete revision = %d", current.DiscussionRevision)
	}
	testutil.Call(t, testHandler.CreateComment, testutil.WithURLParams(newRequest("POST", "/", map[string]any{"content": "/note pause"}), "id", id)).Want(201)
	current, _ = read()
	if current.DiscussionRevision != 3 {
		t.Fatal("control note changed requirements")
	}
}

func TestRequirementsDiscussionRunMustReadBeforeSubmission(t *testing.T) {
	project := dbfx.Project(t, "Discussion read receipt")
	agent := dbfx.Agent(t, "Discussion reader", testRuntimeID)
	raw := `{"v":1,"phase":"clarifying","items":[],"bounds":[],"team":{"project_id":"` + project + `"}}`
	id := dbfx.Issue(t, "Clarify deployment", testutil.Cols{"project_id": project, "acceptance_criteria": raw, "assignee_type": "agent", "assignee_id": agent})
	run := dbfx.Task(t, agent, testutil.Cols{"issue_id": id, "runtime_id": testRuntimeID, "status": "running"})
	request := func(method string, body any) *http.Request {
		r := testutil.WithURLParams(newRequest(method, "/", body), "id", id)
		r.Header.Set("X-Actor-Source", "task_token")
		r.Header.Set("X-Agent-ID", agent)
		r.Header.Set("X-Task-ID", run)
		return r
	}
	submit := func(coverage int, status int) {
		t.Helper()
		issue, _ := testHandler.Queries.GetIssue(context.Background(), parseUUID(id))
		body := map[string]any{"expected_revision": issue.Revision, "acceptance_criteria": map[string]any{"v": 1, "phase": "ready", "covered_discussion_revision": coverage, "items": []map[string]any{{"text": "Open the test homepage"}}, "bounds": []string{}}}
		response := testutil.Call(t, testHandler.UpdateIssue, request("PUT", body)).Want(status)
		if status == http.StatusConflict {
			var failure map[string]any
			response.JSON(&failure)
			message, _ := failure["error"].(string)
			if !strings.Contains(message, "multica issue comment list "+id+" --full --output json") {
				t.Fatalf("missing actionable full-discussion command: %v", failure)
			}
		}
	}
	submit(0, 409)
	for _, query := range []string{"roots_only=true&summary=true", "fold=true"} {
		r := request("GET", nil)
		r.URL.RawQuery = query
		testutil.Call(t, testHandler.ListComments, r).Want(200)
		submit(0, 409)
	}
	testutil.Call(t, testHandler.ListComments, request("GET", nil)).Want(200)
	submit(0, 200)
	testutil.Call(t, testHandler.CreateComment, testutil.WithURLParams(newRequest("POST", "/", map[string]any{"content": "Test environment only", "suppress_agent_ids": []string{agent}}), "id", id)).Want(201)
	submit(1, 409)
	testutil.Call(t, testHandler.ListComments, request("GET", nil)).Want(200)
	submit(1, 200)
	issue, _ := testHandler.Queries.GetIssue(context.Background(), parseUUID(id))
	if issue.AssigneeType.String != "agent" || uuidToString(issue.AssigneeID) != agent {
		t.Fatal("submission lost the discussion agent")
	}
}

func TestRequirementsDiscussionSupplementInvalidatesReview(t *testing.T) {
	f := newSupplementFixture(t, "codex", "running", true)
	project := dbfx.Project(t, "Steered discussion")
	dbfx.Exec(t, `UPDATE issue SET project_id=$2, acceptance_criteria='{"v":1,"phase":"ready","draft_revision":1,"ready_revision":1,"items":[{"id":"A-01","text":"Verify homepage"}],"bounds":[]}' WHERE id=$1`, f.issueID, project)
	supplementRequest(t, f, "11111111-1111-4111-8111-111111111111", "Use the staging environment").Want(201)
	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(f.issueID))
	if err != nil {
		t.Fatal(err)
	}
	criteria, err := workbench.ParseRequirements(issue.AcceptanceCriteria)
	if err != nil || criteria.DiscussionRevision != 1 || workbench.RequirementsReady(criteria) {
		t.Fatal("supplement did not invalidate review", err, criteria)
	}
}
