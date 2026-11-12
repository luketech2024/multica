package protocol

import (
	"fmt"
	"strings"
)

type UserInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type UserInputQuestion struct {
	ID       string            `json:"id"`
	Header   string            `json:"header"`
	Question string            `json:"question"`
	IsOther  bool              `json:"isOther"`
	IsSecret bool              `json:"isSecret"`
	Options  []UserInputOption `json:"options"`
}

type UserInputRequest struct {
	Key       string              `json:"key"`
	ThreadID  string              `json:"threadId"`
	TurnID    string              `json:"turnId"`
	Questions []UserInputQuestion `json:"questions"`
}

type UserInputAnswer struct {
	Answers []string `json:"answers"`
}
type UserInputResponse struct {
	Answers map[string]UserInputAnswer `json:"answers"`
}

func (r UserInputRequest) Validate() error {
	if r.Key == "" || len(r.Key) > 512 || r.ThreadID == "" || r.TurnID == "" || len(r.Questions) == 0 || len(r.Questions) > 3 {
		return fmt.Errorf("invalid user input request")
	}
	seen := map[string]bool{}
	for _, q := range r.Questions {
		if strings.TrimSpace(q.ID) == "" || seen[q.ID] || strings.TrimSpace(q.Question) == "" || len(q.Question) > 10000 || len(q.Options) > 20 {
			return fmt.Errorf("invalid question")
		}
		// Secrets belong in managed credentials, never in a shared task discussion.
		if q.IsSecret {
			return fmt.Errorf("secret questions require managed credentials")
		}
		seen[q.ID] = true
	}
	return nil
}

func (r UserInputRequest) ValidateResponse(response UserInputResponse) error {
	if len(response.Answers) != len(r.Questions) {
		return fmt.Errorf("answer every question")
	}
	for _, q := range r.Questions {
		answer, ok := response.Answers[q.ID]
		if !ok || len(answer.Answers) != 1 || strings.TrimSpace(answer.Answers[0]) == "" || len(answer.Answers[0]) > 10000 {
			return fmt.Errorf("invalid answer")
		}
		if len(q.Options) > 0 && !q.IsOther {
			found := false
			for _, option := range q.Options {
				if option.Label == answer.Answers[0] {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("select an available option")
			}
		}
	}
	return nil
}
