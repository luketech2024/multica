package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestUserInputResolutionFollowsDelivery(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "delivered", true: "broken pipe"}[failed], func(t *testing.T) {
			var resolution string
			d := taskSupplementTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/resolve") {
					var body map[string]string
					_ = json.NewDecoder(r.Body).Decode(&body)
					resolution = body["status"]
					_, _ = w.Write([]byte(`{}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":"question","status":"answered","response":{"answers":{"port":{"answers":["8080"]}}}}`))
			})
			err := d.requestUserInput(t.Context(), "task", protocol.UserInputRequest{}, func(response protocol.UserInputResponse) error {
				if resolution != "" {
					t.Error("resolved before delivery")
				}
				if response.Answers["port"].Answers[0] != "8080" {
					t.Error("answer lost")
				}
				if failed {
					return errors.New("broken pipe")
				}
				return nil
			})
			want := "resolved"
			if failed {
				want = "cancelled"
			}
			if resolution != want || (err != nil) != failed {
				t.Fatalf("resolution=%s error=%v", resolution, err)
			}
		})
	}
}

func TestUserInputCancellationClosesQuestion(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var resolution string
	d := taskSupplementTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			cancel()
		}
		if strings.HasSuffix(r.URL.Path, "/resolve") {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			resolution = body["status"]
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"question","status":"pending","response":null}`))
	})

	err := d.requestUserInput(ctx, "task", protocol.UserInputRequest{}, func(protocol.UserInputResponse) error { t.Fatal("cancelled answer delivered"); return nil })
	if err == nil {
		t.Fatal("cancellation lost")
	}
	if resolution != "cancelled" {
		t.Fatalf("resolution=%s", resolution)
	}
}
