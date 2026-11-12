package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestUserInputRoundTripAndSettlement(t *testing.T) {
	f := newSupplementFixture(t, "codex", "running", false)
	dbfx.Cleanup(t, `DELETE FROM task_user_input WHERE task_id=$1`, f.taskID)
	request := protocol.UserInputRequest{Key: "thread/turn/1", ThreadID: "thread", TurnID: "turn", Questions: []protocol.UserInputQuestion{{ID: "port", Header: "Port", Question: "Which port?", IsOther: true}}}
	create := func() *http.Request {
		return withURLParam(newOwnerDaemonRequest(http.MethodPost, "/questions", request), "taskId", f.taskID)
	}
	var created struct {
		ID      string                    `json:"id"`
		Request protocol.UserInputRequest `json:"request"`
	}
	testutil.Call(t, testHandler.CreateUserInput, create()).Want(200).JSON(&created)
	if created.Request.Questions[0].ID != "port" {
		t.Fatal("request was not encoded as JSON")
	}
	var replay struct {
		ID string `json:"id"`
	}
	testutil.Call(t, testHandler.CreateUserInput, create()).Want(200).JSON(&replay)
	if replay.ID != created.ID {
		t.Fatal("duplicate request created")
	}
	answer := protocol.UserInputResponse{Answers: map[string]protocol.UserInputAnswer{"port": {Answers: []string{"8080"}}}}
	answerRequest := func() *http.Request {
		return withURLParams(newRequest(http.MethodPost, "/answer", answer), "id", f.issueID, "requestId", created.ID)
	}
	testutil.Call(t, testHandler.AnswerUserInput, answerRequest()).Want(200)
	testutil.Call(t, testHandler.AnswerUserInput, answerRequest()).Want(200)
	var received struct {
		Status   string                     `json:"status"`
		Response protocol.UserInputResponse `json:"response"`
	}
	get := withURLParams(newOwnerDaemonRequest(http.MethodGet, "/question", nil), "taskId", f.taskID, "requestId", created.ID)
	testutil.Call(t, testHandler.GetUserInput, get).Want(200).JSON(&received)
	if received.Status != "answered" || received.Response.Answers["port"].Answers[0] != "8080" {
		t.Fatalf("unexpected answer %+v", received)
	}
	answer.Answers["port"] = protocol.UserInputAnswer{Answers: []string{"9090"}}
	testutil.Call(t, testHandler.AnswerUserInput, answerRequest()).Want(409)
	if err := service.SettleTerminalTaskState(t.Context(), testHandler.Queries, db.AgentTaskQueue{ID: parseUUID(f.taskID)}); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, testHandler.AnswerUserInput, answerRequest()).Want(409)
}

func TestUserInputRejectsInvalidAndEndedRequests(t *testing.T) {
	f := newSupplementFixture(t, "codex", "completed", false)
	request := protocol.UserInputRequest{Key: "t/1", ThreadID: "thread", TurnID: "turn", Questions: []protocol.UserInputQuestion{{ID: "q", Question: "Choose", IsOther: true}}}
	create := func() *http.Request {
		return withURLParam(newOwnerDaemonRequest(http.MethodPost, "/questions", request), "taskId", f.taskID)
	}
	testutil.Call(t, testHandler.CreateUserInput, create()).Want(409)
	request.Questions[0].IsSecret = true
	testutil.Call(t, testHandler.CreateUserInput, create()).Want(400)
}
