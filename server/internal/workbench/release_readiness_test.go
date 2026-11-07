package workbench

import (
	"strings"
	"testing"
)

func TestReleaseRepositoryIdentity(t *testing.T) {
	for raw, want := range map[string]string{
		"https://git.example/acme/app.git/":       "git.example/acme/app",
		"ssh://git@git.example/acme/app.git":      "git.example/acme/app",
		"git@git.example:acme/app.git":            "git.example/acme/app",
		"https://git.example:8443/group/sub/app":  "git.example:8443/group/sub/app",
		"https://other.example/acme/app":          "other.example/acme/app",
		"ssh://git@git.example:2222/acme/app.git": "",
		"https://git.example/acme/app?ref=main":   "",
		"/local/app":                              "",
	} {
		if got := repositoryIdentity(raw); got != want {
			t.Errorf("%s => %q, want %q", raw, got, want)
		}
	}
}

func TestReleaseReadinessBindsAcceptanceToRepositoryMerge(t *testing.T) {
	head, merge := strings.Repeat("a", 40), strings.Repeat("b", 40)
	for _, tc := range []struct {
		name   string
		change func(*Requirements, []ReleaseRepository, []ReleasePullRequest)
		want   string
	}{
		{"ready", func(_ *Requirements, _ []ReleaseRepository, _ []ReleasePullRequest) {}, ""},
		{"not accepted", func(c *Requirements, _ []ReleaseRepository, _ []ReleasePullRequest) { c.Gate.Accept = nil }, "acceptance"},
		{"wrong host", func(_ *Requirements, _ []ReleaseRepository, p []ReleasePullRequest) {
			p[0].RepoURL = "https://other.test/acme/app"
		}, "merge_missing"},
		{"wrong branch", func(_ *Requirements, _ []ReleaseRepository, p []ReleasePullRequest) { p[0].BaseBranch = "other" }, "merge_missing"},
		{"changed source", func(_ *Requirements, _ []ReleaseRepository, p []ReleasePullRequest) { p[0].HeadSHA = merge }, "merge_missing"},
		{"unmerged", func(_ *Requirements, _ []ReleaseRepository, p []ReleasePullRequest) { p[0].State = "open" }, "merge_missing"},
		{"missing hash", func(_ *Requirements, _ []ReleaseRepository, p []ReleasePullRequest) { p[0].MergeCommitSHA = "" }, "merge_missing"},
		{"unrelated task", func(_ *Requirements, _ []ReleaseRepository, p []ReleasePullRequest) { p[0].IssueID = "other" }, "merge_missing"},
		{"missing repository", func(_ *Requirements, r []ReleaseRepository, _ []ReleasePullRequest) { r[0].ID = "other" }, "repository_missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			criteria := Requirements{Gate: Gates{Accept: &GateCheck{}}, Review: &DeliveryReview{Action: "accept"}, Plan: &ExecutionPlan{Issues: []string{"child"}}, Delivery: map[string]DeliveryEvidence{"child": {IssueID: "child", RepoID: "repo", Commit: head}, "superseded": {IssueID: "superseded", RepoID: "old-repo", Commit: merge}}}
			repos := []ReleaseRepository{{ID: "repo", URL: "git@github.com:acme/app.git", Branch: "main"}}
			prs := []ReleasePullRequest{{ID: "pr", IssueID: "child", RepoURL: "https://github.com/acme/app", URL: "https://github.com/acme/app/pull/1", State: "merged", BaseBranch: "main", HeadSHA: head, MergeCommitSHA: merge}}
			tc.change(&criteria, repos, prs)
			result := CheckReleaseReadiness("parent", "done", criteria, repos, prs, true)
			if tc.want != "" {
				if len(result.Blockers) != 1 || result.Blockers[0].Code != tc.want {
					t.Fatalf("unexpected blockers: %+v", result)
				}
				return
			}
			if len(result.Blockers) != 0 || len(result.Merges) != 1 || result.Merges[0].MergeCommit != merge {
				t.Fatalf("missing merge: %+v", result)
			}
			prs = append(prs, prs[0])
			prs[1].IssueID = "parent"
			if got := CheckReleaseReadiness("parent", "done", criteria, repos, prs, true); len(got.Merges) != 1 || len(got.Blockers) != 0 {
				t.Fatal("duplicate parent/child links changed eligibility")
			}
			prs[1].ID = "other-pr"
			if got := CheckReleaseReadiness("parent", "done", criteria, repos, prs, true); len(got.Blockers) != 1 || got.Blockers[0].Code != "merge_ambiguous" {
				t.Fatal("ambiguous merges accepted")
			}
			if got := CheckReleaseReadiness("parent", "done", criteria, repos, prs, false); got.Blockers[0].Code != "acceptance" {
				t.Fatal("stale delivery accepted")
			}
		})
	}
}
