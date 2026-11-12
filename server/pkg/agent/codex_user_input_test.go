package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCodexUserInputWaitDoesNotTripInactivity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	path := writeFakeCodexAppServer(t, `
read line
echo '{"jsonrpc":"2.0","id":1,"result":{}}'
read line
read line
echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thread-1"}}}'
read line
echo '{"jsonrpc":"2.0","id":3,"result":{"turn":{"id":"turn-1"}}}'
echo '{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thread-1","turn":{"id":"turn-1"}}}'
echo '{"jsonrpc":"2.0","id":41,"method":"item/tool/requestUserInput","params":{"threadId":"thread-1","turnId":"turn-1","questions":[{"id":"port","header":"Port","question":"Which port?"}]}}'
read answer
case "$answer" in
  *8080*) ;;
  *) exit 2 ;;
esac
echo '{"jsonrpc":"2.0","method":"serverRequest/resolved","params":{"threadId":"thread-1","requestId":41}}'
echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thread-1","turnId":"turn-1","item":{"id":"message-1","type":"agentMessage","text":"Confirmed port 8080","phase":"final_answer"}}}'
echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed"}}}'
`)
	backend, err := New("codex", Config{ExecutablePath: path, Logger: slog.Default()})
	if err != nil {
		t.Fatal(err)
	}
	session, err := backend.Execute(t.Context(), "configure port", ExecOptions{
		Timeout: 5 * time.Second, SemanticInactivityTimeout: 80 * time.Millisecond, FirstTurnNoProgressTimeout: 80 * time.Millisecond,
		OnUserInput: func(ctx context.Context, request protocol.UserInputRequest, deliver func(protocol.UserInputResponse) error) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(400 * time.Millisecond):
			}
			return deliver(protocol.UserInputResponse{Answers: map[string]protocol.UserInputAnswer{"port": {Answers: []string{"8080"}}}})
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range session.Messages {
		}
	}()
	result := <-session.Result
	if result.Status != "completed" {
		t.Fatalf("waiting for user interrupted execution: %+v", result)
	}
}

func TestCodexUserInputAnswersOriginalRequest(t *testing.T) {
	c, stdin, _ := newTestCodexClient(t)
	c.userInputCtx = t.Context()
	c.threadID = "thread-1"
	c.setActiveTurnID("turn-1")
	ready := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	c.onUserInput = func(ctx context.Context, request protocol.UserInputRequest, deliver func(protocol.UserInputResponse) error) error {
		defer close(done)
		if request.Key != `thread-1/turn-1/"question-1"` {
			t.Errorf("wrong identity %q", request.Key)
		}
		close(ready)
		<-release
		return deliver(protocol.UserInputResponse{Answers: map[string]protocol.UserInputAnswer{"port": {Answers: []string{"8080"}}}})
	}
	c.handleLine(`{"id":"question-1","method":"item/tool/requestUserInput","params":{"threadId":"thread-1","turnId":"turn-1","questions":[{"id":"port","header":"Port","question":"Which port?","isOther":true}]}}`)
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("request blocked protocol reader")
	}
	if c.waitingForUser.Load() != 1 {
		t.Fatal("waiting state missing")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("answer was not delivered")
	}
	var response struct {
		ID     string                     `json:"id"`
		Result protocol.UserInputResponse `json:"result"`
	}
	lines := stdin.Lines()
	if len(lines) != 1 || json.Unmarshal([]byte(lines[0]), &response) != nil || response.ID != "question-1" || response.Result.Answers["port"].Answers[0] != "8080" {
		t.Fatalf("unexpected response %v", lines)
	}
}

func TestCodexUserInputResolutionCancelsWait(t *testing.T) {
	c, stdin, _ := newTestCodexClient(t)
	c.userInputCtx = t.Context()
	c.threadID = "thread-1"
	c.setActiveTurnID("turn-1")
	started := make(chan struct{})
	stopped := make(chan struct{})
	c.onUserInput = func(ctx context.Context, request protocol.UserInputRequest, deliver func(protocol.UserInputResponse) error) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	}
	c.handleLine(`{"id":17,"method":"item/tool/requestUserInput","params":{"threadId":"thread-1","turnId":"turn-1","questions":[{"id":"port","header":"Port","question":"Which port?"}]}}`)
	<-started
	c.handleLine(`{"method":"serverRequest/resolved","params":{"threadId":"thread-1","requestId":17}}`)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("resolved request still waiting")
	}
	if len(stdin.Lines()) != 0 {
		t.Fatal("answered a resolved request")
	}
}
