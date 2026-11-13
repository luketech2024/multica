package workbench

import (
	"strings"
	"testing"
)

func TestParseExecutionPlanRequest(t *testing.T) {
	valid := `{"expected_revision":3,"version":1,"idempotency_key":"deploy","tasks":[{"key":"analyze","expected":["report"]}]}`
	for _, tc := range []struct{ name, raw, want string }{
		{"valid", valid, ""},
		{"string deliverable", strings.Replace(valid, `["report"]`, `"report"`, 1), "tasks[].expected must be an array of strings"},
		{"missing deliverable", strings.Replace(valid, `["report"]`, `null`, 1), "tasks[0].expected"},
		{"wrong revision", strings.Replace(valid, `"expected_revision":3`, `"expected_revision":"3"`, 1), "expected_revision"},
		{"missing version", strings.Replace(valid, `"version":1`, `"version":0`, 1), "version must"},
		{"wrapped plan", `{"expected_revision":3,"version":1,"idempotency_key":"deploy","plan":{"tasks":[]}}`, "top-level array"},
		{"malformed", `{`, "invalid execution plan JSON"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseExecutionPlanRequest([]byte(tc.raw))
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}
