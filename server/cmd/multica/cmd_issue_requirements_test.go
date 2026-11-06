package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestRunIssueCreateStructuredRequirements(t *testing.T) {
	t.Chdir(t.TempDir())
	const project = "11111111-1111-4111-8111-111111111111"
	for _, scenario := range []struct {
		name      string
		flags     []string
		wantItems int
		wantError string
	}{
		{name: "empty project enters clarification"},
		{name: "criteria preserve punctuation", flags: []string{"--criterion", "导出姓名,账期和金额", "--criterion", "Downloaded file opens", "--boundary", "No billing changes", "--external-ticket", "REQ-42"}, wantItems: 2},
		{name: "explicit unassignment", flags: []string{"--unassigned"}},
		{name: "invalid initial state", flags: []string{"--status", "in_progress"}, wantError: "todo or backlog"},
		{name: "empty item rejected", flags: []string{"--criterion", " "}, wantError: "1 to 120"},
		{name: "long item rejected", flags: []string{"--criterion", strings.Repeat("字", 121)}, wantError: "1 to 120"},
		{name: "invalid external number", flags: []string{"--external-ticket", "REQ/42"}, wantError: "32 letters"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var body struct {
				ProjectID          string                 `json:"project_id"`
				Criteria           workbench.Requirements `json:"acceptance_criteria"`
				External           string                 `json:"external_ticket_no"`
				UseProjectAssignee *bool                  `json:"use_project_assignee"`
			}
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/issues" || r.Method != http.MethodPost {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				posts++
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]any{"id": "created", "title": "Export report"})
			}))
			defer server.Close()
			t.Setenv("MULTICA_SERVER_URL", server.URL)
			t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
			t.Setenv("MULTICA_TOKEN", "mat_test-token")
			cmd := newIssueCreateTestCmd()
			if err := cmd.ParseFlags(append([]string{"--title", "Export report", "--project", project}, scenario.flags...)); err != nil {
				t.Fatal(err)
			}
			err := runIssueCreate(cmd, nil)
			if scenario.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), scenario.wantError) || posts != 0 {
					t.Fatalf("error=%v, posts=%d", err, posts)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if posts != 1 || body.ProjectID != project || body.Criteria.V != 1 || len(body.Criteria.Items) != scenario.wantItems {
				t.Fatalf("invalid request %+v", body)
			}
			if scenario.wantItems > 0 {
				if body.Criteria.Items[0].Text != "导出姓名,账期和金额" || body.Criteria.Items[0].ID != "" || body.Criteria.Items[0].Checked != nil || body.Criteria.Bounds[0] != "No billing changes" || body.External != "REQ-42" {
					t.Fatalf("lost or fabricated requirement fields %+v", body)
				}
			}
			if scenario.name == "explicit unassignment" && (body.UseProjectAssignee == nil || *body.UseProjectAssignee) {
				t.Fatal("explicit unassignment was lost")
			}
		})
	}
}
