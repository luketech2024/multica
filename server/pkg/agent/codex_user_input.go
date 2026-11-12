package agent

import (
	"context"
	"encoding/json"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (c *codexClient) requestUserInput(raw map[string]json.RawMessage) {
	id := raw["id"]
	if c.onUserInput == nil {
		c.respondError(id, -32601, "user input is unavailable for this execution")
		return
	}
	var request protocol.UserInputRequest
	if json.Unmarshal(raw["params"], &request) != nil {
		c.respondError(id, -32602, "invalid questions")
		return
	}
	request.Key = request.ThreadID + "/" + request.TurnID + "/" + string(id)
	if err := request.Validate(); err != nil {
		c.respondError(id, -32602, err.Error())
		return
	}
	if request.TurnID != c.activeTurnID() {
		c.respondError(id, -32602, "question belongs to an inactive turn")
		return
	}
	ctx, cancel := context.WithCancel(c.userInputCtx)
	key := string(id)
	c.userInputMu.Lock()
	if c.userInputs == nil {
		c.userInputs = make(map[string]context.CancelFunc)
	}
	if _, exists := c.userInputs[key]; exists {
		c.userInputMu.Unlock()
		cancel()
		return
	}
	c.userInputs[key] = cancel
	c.userInputMu.Unlock()
	c.waitingForUser.Add(1)
	go func() {
		defer func() {
			cancel()
			c.userInputMu.Lock()
			delete(c.userInputs, key)
			c.userInputMu.Unlock()
			c.waitingForUser.Add(-1)
		}()
		err := c.onUserInput(ctx, request, func(response protocol.UserInputResponse) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return c.respond(id, response)
		})
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.respondError(id, -32603, "could not collect user input")
			return
		}
		if c.onSemanticActivity != nil {
			c.onSemanticActivity("user_input_answered")
		}
	}()
}

func (c *codexClient) resolveUserInputNotification(raw map[string]json.RawMessage) {
	var method string
	_ = json.Unmarshal(raw["method"], &method)
	if method != "serverRequest/resolved" && method != "turn/completed" {
		return
	}
	var params struct {
		RequestID json.RawMessage `json:"requestId"`
		ThreadID  string          `json:"threadId"`
		Turn      struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	_ = json.Unmarshal(raw["params"], &params)
	if params.ThreadID != c.getThreadID() {
		return
	}
	if method == "turn/completed" && params.Turn.ID != c.activeTurnID() {
		return
	}
	c.userInputMu.Lock()
	defer c.userInputMu.Unlock()
	for key, cancel := range c.userInputs {
		if method == "turn/completed" || key == string(params.RequestID) {
			cancel()
		}
	}
}
