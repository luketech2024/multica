package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestConfirmedPRMergeCommit(t *testing.T) {
	for _, tc := range []struct{ name, state, sha, want string }{
		{"merged", "merged", strings.Repeat("a", 40), strings.Repeat("a", 40)},
		{"sha256", "merged", strings.Repeat("b", 64), strings.Repeat("b", 64)},
		{"temporary", "open", strings.Repeat("a", 40), ""},
		{"closed", "closed", strings.Repeat("a", 40), ""},
		{"short", "merged", "abcdef", ""},
		{"invalid", "merged", strings.Repeat("z", 40), ""},
		{"absent", "merged", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := confirmedPRMergeCommit(tc.state, tc.sha); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGitHubMergeEvidenceSurvivesMetadataAndOldEvents(t *testing.T) {
	ctx := context.Background()
	_, installationID := setupPRTestIssue(t, ctx, "merge-evidence-secret")
	sha := strings.Repeat("b", 40)
	for _, step := range []struct {
		name, state, updated, base, commit, want string
		merged                                   bool
	}{
		{"temporary", "open", "2026-11-01T00:00:00Z", "main", sha, "", false},
		{"merged", "closed", "2026-11-02T00:00:00Z", "main", sha, sha, true},
		{"metadata", "closed", "2026-11-03T00:00:00Z", "", "", sha, true},
		{"old", "open", "2026-11-01T00:00:00Z", "wrong", "", sha, false},
	} {
		t.Run(step.name, func(t *testing.T) {
			var payload ghPullRequestPayload
			raw, err := json.Marshal(map[string]any{
				"action": "edited",
				"pull_request": map[string]any{
					"number": 82, "title": "Merge evidence", "state": step.state, "merged": step.merged,
					"html_url": "https://github.com/acme/evidence/pull/82", "head": map[string]string{"ref": "feat/evidence", "sha": sha},
					"base": map[string]string{"ref": step.base}, "merge_commit_sha": step.commit,
					"created_at": "2026-11-01T00:00:00Z", "updated_at": step.updated,
				},
				"repository": map[string]any{"name": "evidence", "owner": map[string]string{"login": "acme"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatal(err)
			}
			testHandler.mirrorPullRequestForWorkspace(ctx, parseUUID(testWorkspaceID), installationID, &payload, prLinkPolicy{})
			row, err := testHandler.Queries.GetGitHubPullRequest(ctx, db.GetGitHubPullRequestParams{
				WorkspaceID: parseUUID(testWorkspaceID), RepoOwner: "acme", RepoName: "evidence", PrNumber: 82,
			})
			if err != nil {
				t.Fatal(err)
			}
			got := githubPullRequestToResponse(row, false)
			if got.BaseBranch != "main" || got.MergeCommitSHA != step.want || got.HeadSHA != sha {
				t.Fatalf("wrong merge evidence: %+v", got)
			}
		})
	}
}

func TestVCSWebhookMergeEvidenceSurvivesMetadataAndOldEvents(t *testing.T) {
	ctx := context.Background()
	box := withVCSBox(t)
	connID := seedVCSConnection(t, ctx, box, "forgejo", "https://merge-evidence.test")
	issue := newVCSIssue(t, "Merge evidence")
	t.Cleanup(func() { cleanupVCS(ctx, issue.ID) })
	sha := strings.Repeat("a", 40)
	for _, step := range []struct {
		name, state, updated, base, commit, want string
		merged                                   bool
	}{
		{"temporary", "open", "2026-11-01T00:00:00Z", "main", sha, "", false},
		{"merged", "closed", "2026-11-02T00:00:00Z", "main", sha, sha, true},
		{"metadata", "closed", "2026-11-03T00:00:00Z", "", "", sha, true},
		{"old", "open", "2026-11-01T00:00:00Z", "wrong", "", sha, false},
	} {
		t.Run(step.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{
				"action": "edited",
				"pull_request": map[string]any{
					"number": 81, "title": "Fix " + issue.Identifier, "state": step.state, "merged": step.merged,
					"html_url": "https://merge-evidence.test/acme/app/pulls/81", "head": map[string]string{"ref": "feat/evidence", "sha": sha},
					"base": map[string]string{"ref": step.base}, "merge_commit_sha": step.commit,
					"created_at": "2026-11-01T00:00:00Z", "updated_at": step.updated,
				},
				"repository": map[string]any{"name": "app", "owner": map[string]string{"username": "acme"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			req := vcsWebhookReq(connID, map[string]string{"X-Gitea-Event": "pull_request", "X-Gitea-Signature": giteaSig(raw)}, raw)
			testutil.Call(t, testHandler.HandleVCSWebhook, req).Want(http.StatusAccepted)
			rows, err := testHandler.Queries.ListVCSPullRequestsByIssue(ctx, parseUUID(issue.ID))
			if err != nil || len(rows) != 1 {
				t.Fatalf("rows=%d err=%v", len(rows), err)
			}
			got := vcsPullRequestRowToResponse(rows[0])
			if got.BaseBranch != "main" || got.MergeCommitSHA != step.want || got.HeadSHA != sha {
				t.Fatalf("wrong merge evidence: %+v", got)
			}
		})
	}
}
