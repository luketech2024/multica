package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestPlatformQuestionsSurviveTurnAndRecordAnswerOnce(t *testing.T) {
	for _, provider := range []string{"cursor", "claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			f := newSupplementFixture(t, provider, "running", false)
			project := dbfx.Project(t, "Question project")
			dbfx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issueID, project)
			dbfx.Cleanup(t, `DELETE FROM task_user_input WHERE issue_id=$1`, f.issueID)
			dbfx.Exec(t, `UPDATE issue SET assignee_type='agent',assignee_id=$2,acceptance_criteria='{"v":1,"phase":"ready","draft_revision":1,"ready_revision":1,"items":[],"bounds":[],"team":{}}' WHERE id=$1`, f.issueID, f.agentID)
			question := protocol.UserInputRequest{Key: "environment", Questions: []protocol.UserInputQuestion{{ID: "env", Question: "Choose environment", IsOther: true, Options: []protocol.UserInputOption{{Label: "Staging", Description: "Use the test server", Recommended: true}}}}}
			req := withURLParam(newRequest("POST", "/", question), "id", f.issueID)
			req.Header.Set("X-Actor-Source", "task_token")
			req.Header.Set("X-Agent-ID", f.agentID)
			req.Header.Set("X-Task-ID", f.taskID)
			var created struct {
				ID string `json:"id"`
			}
			testutil.Call(t, testHandler.CreateIssueUserInput, req).Want(200).JSON(&created)
			dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, f.taskID)
			if err := service.SettleTerminalTaskState(t.Context(), testHandler.Queries, db.AgentTaskQueue{ID: parseUUID(f.taskID)}); err != nil {
				t.Fatal(err)
			}
			answer := protocol.UserInputResponse{Answers: map[string]protocol.UserInputAnswer{"env": {Answers: []string{"Staging"}}}}
			for range 2 {
				testutil.Call(t, testHandler.AnswerUserInput, withURLParams(newRequest(http.MethodPost, "/", answer), "id", f.issueID, "requestId", created.ID)).Want(200)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND content LIKE '%Staging%'`, f.issueID); n != 1 {
				t.Fatalf("answer comments %d", n)
			}
			var revision int
			dbfx.QueryRow(t, `SELECT (acceptance_criteria->>'discussion_revision')::int FROM issue WHERE id=$1`, f.issueID).Scan(&revision)
			if revision != 1 {
				t.Fatalf("discussion revision %d", revision)
			}
		})
	}
}

func TestPlatformQuestionsReplacementAndCancellation(t *testing.T) {
	f := newSupplementFixture(t, "cursor", "running", false)
	dbfx.Cleanup(t, `DELETE FROM task_user_input WHERE issue_id=$1`, f.issueID)
	create := func(key string) string {
		req := withURLParam(newRequest("POST", "/", protocol.UserInputRequest{Key: key, Questions: []protocol.UserInputQuestion{{ID: "q", Question: "Choose", IsOther: true}}}), "id", f.issueID)
		req.Header.Set("X-Actor-Source", "task_token")
		req.Header.Set("X-Agent-ID", f.agentID)
		req.Header.Set("X-Task-ID", f.taskID)
		var row struct {
			ID string `json:"id"`
		}
		testutil.Call(t, testHandler.CreateIssueUserInput, req).Want(200).JSON(&row)
		return row.ID
	}
	first := create("first")
	second := create("second")
	if first == second {
		t.Fatal("replacement reused request")
	}
	if create("first") != first {
		t.Fatal("retry changed request identity")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM task_user_input WHERE id=$1 AND status='pending'`, second); n != 1 {
		t.Fatal("old retry cancelled the replacement")
	}
	answer := protocol.UserInputResponse{Answers: map[string]protocol.UserInputAnswer{"q": {Answers: []string{"yes"}}}}
	testutil.Call(t, testHandler.AnswerUserInput, withURLParams(newRequest("POST", "/", answer), "id", f.issueID, "requestId", first)).Want(409)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='cancelled' WHERE id=$1`, f.taskID)
	if err := service.SettleTerminalTaskState(t.Context(), testHandler.Queries, db.AgentTaskQueue{ID: parseUUID(f.taskID)}); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, testHandler.AnswerUserInput, withURLParams(newRequest("POST", "/", answer), "id", f.issueID, "requestId", second)).Want(409)
}

func TestPlatformQuestionsPauseAfterTurnCompleted(t *testing.T) {
	f := newSupplementFixture(t, "cursor", "completed", false)
	dbfx.Cleanup(t, `DELETE FROM task_user_input WHERE issue_id=$1`, f.issueID)
	dbfx.Exec(t, `UPDATE issue SET acceptance_criteria='{"v":1,"items":[],"bounds":[]}' WHERE id=$1`, f.issueID)
	var questionID string
	dbfx.QueryRow(t, `INSERT INTO task_user_input(task_id,workspace_id,issue_id,request_key,request) VALUES($1,$2,$3,'paused','{"origin":"platform","key":"paused","questions":[{"id":"q","question":"Which environment?","isOther":true}]}') RETURNING id`, f.taskID, testWorkspaceID, f.issueID).Scan(&questionID)
	issue, err := testHandler.Queries.GetIssue(t.Context(), parseUUID(f.issueID))
	if err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, testHandler.ControlIssueExecution, withURLParam(newRequest("POST", "/", map[string]any{"action": "pause", "expected_revision": issue.Revision}), "id", f.issueID)).Want(200)
	if n := dbfx.Count(t, `SELECT count(*) FROM task_user_input WHERE id=$1 AND status='cancelled'`, questionID); n != 1 {
		t.Fatal("completed turn left an active question after pause")
	}
	answer := protocol.UserInputResponse{Answers: map[string]protocol.UserInputAnswer{"q": {Answers: []string{"test"}}}}
	testutil.Call(t, testHandler.AnswerUserInput, withURLParams(newRequest("POST", "/", answer), "id", f.issueID, "requestId", questionID)).Want(409)
}

func TestPlatformExecutionQuestionResumesChildOnce(t *testing.T) {
	f := newSupplementFixture(t, "cursor", "running", false)
	parent := dbfx.Issue(t, "Deploy application")
	dbfx.Exec(t, `UPDATE issue SET acceptance_criteria=jsonb_build_object('v',1,'version',1,'plan',jsonb_build_object('version',1,'issues',jsonb_build_array($2::text))) WHERE id=$1`, parent, f.issueID)
	dbfx.Exec(t, `UPDATE issue SET parent_issue_id=$2,assignee_type='agent',assignee_id=$3,status='in_progress',properties='{"workbench_handoff":{"v":2,"role":"ops","requirement_version":1,"covers":["A-01"],"expected":["report"]}}' WHERE id=$1`, f.issueID, parent, f.agentID)
	dbfx.Cleanup(t, `DELETE FROM task_user_input WHERE issue_id=$1`, f.issueID)
	dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id=$1 AND id<>$2`, f.issueID, f.taskID)
	request := protocol.UserInputRequest{Key: "container-runtime", Questions: []protocol.UserInputQuestion{{ID: "runtime", Question: "Choose container runtime", IsOther: true, Options: []protocol.UserInputOption{{Label: "Docker", Description: "Install Docker"}, {Label: "Podman", Description: "Use existing runtime"}}}}}
	req := withURLParam(newRequest("POST", "/", request), "id", f.issueID)
	req.Header.Set("X-Actor-Source", "task_token")
	req.Header.Set("X-Agent-ID", f.agentID)
	req.Header.Set("X-Task-ID", f.taskID)
	var created struct {
		ID string `json:"id"`
	}
	testutil.Call(t, testHandler.CreateIssueUserInput, req).Want(200).JSON(&created)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, f.taskID)
	if err := service.SettleTerminalTaskState(t.Context(), testHandler.Queries, db.AgentTaskQueue{ID: parseUUID(f.taskID)}); err != nil {
		t.Fatal(err)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM task_user_input WHERE id=$1 AND status='pending'`, created.ID); n != 1 {
		t.Fatal("execution ended without retaining its pending question")
	}
	answer := protocol.UserInputResponse{Answers: map[string]protocol.UserInputAnswer{"runtime": {Answers: []string{"Docker"}}}}
	for range 2 {
		testutil.Call(t, testHandler.AnswerUserInput, withURLParams(newRequest(http.MethodPost, "/", answer), "id", f.issueID, "requestId", created.ID)).Want(200)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND id<>$2 AND agent_id=$3`, f.issueID, f.taskID, f.agentID); n != 1 {
		t.Fatalf("expected one child continuation, got %d", n)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, parent); n != 0 {
		t.Fatal("answer woke the parent instead of the execution child")
	}
}
