package workbench

import (
	"strings"
	"testing"
	"time"
)

func oneSpecTestArtifact(stage string) OneSpecArtifact {
	return OneSpecArtifact{Stage: stage, Summary: "Verified change", Refs: []OneSpecArtifactRef{{RepositoryID: "repository", Path: "onespec/changes/task/design.md", Commit: strings.Repeat("a", 40), Digest: strings.Repeat("b", 64)}}}
}
func TestOneSpecConfirmationVersionAndArchiveExecution(t *testing.T) {
	flow := OneSpecFlow{Kind: "feature", Phase: "design", Status: "preparing"}
	now := time.Now()
	if err := flow.Submit(oneSpecTestArtifact("design"), "developer", now); err != nil {
		t.Fatal(err)
	}
	if err := flow.Advance(); err == nil {
		t.Fatal("advanced without confirmation")
	}
	if err := flow.Decide("design", 1, true, "", "creator", now); err != nil {
		t.Fatal(err)
	}
	if err := flow.Submit(oneSpecTestArtifact("design"), "developer", now); err != nil {
		t.Fatal(err)
	}
	if flow.Approved("design") {
		t.Fatal("old confirmation approved new artifact")
	}
	if err := flow.Decide("design", 1, true, "", "creator", now); err == nil {
		t.Fatal("stale confirmation accepted")
	}
	if err := flow.Decide("design", 2, false, "", "creator", now); err == nil {
		t.Fatal("empty rejection accepted")
	}
	if err := flow.Decide("design", 2, true, "", "creator", now); err != nil {
		t.Fatal(err)
	}
	if err := flow.Advance(); err != nil || flow.Phase != "plan" {
		t.Fatalf("advance: %v, %s", err, flow.Phase)
	}
	flow.Phase = "archive"
	if err := flow.Submit(oneSpecTestArtifact("archive"), "developer", now); err != nil {
		t.Fatal(err)
	}
	if err := flow.Decide("archive", 1, true, "", "creator", now); err != nil {
		t.Fatal(err)
	}
	if err := flow.Advance(); err != nil || flow.Phase != "archiving" || flow.Status == "completed" {
		t.Fatalf("archive confirmation skipped execution: %+v %v", flow, err)
	}
	if err := flow.Reopen("design"); err != nil {
		t.Fatal(err)
	}
	if flow.Approved("design") || flow.Approved("archive") {
		t.Fatal("downstream confirmations survived reopening")
	}
}
func TestOneSpecEvidenceAndBranchValidation(t *testing.T) {
	for _, path := range []string{"../secret", "/root/a", "a/../b", "C:\\file", "onespec\\file"} {
		flow := OneSpecFlow{Kind: "feature", Phase: "testing"}
		a := oneSpecTestArtifact("testing")
		a.Refs[0].Path = path
		if err := flow.Submit(a, "test", time.Now()); err == nil {
			t.Fatalf("accepted path %q", path)
		}
	}
	flow := OneSpecFlow{Kind: "feature", Phase: "testing"}
	if err := flow.Submit(oneSpecTestArtifact("testing"), "test", time.Now()); err != nil || flow.Status != "ready" {
		t.Fatal("testing should publish evidence without an extra human gate")
	}
	for _, test := range []struct{ kind, id, description, date, want string }{{"development", "123", "", "", "release_123"}, {"hotfix", "123", "", "20261111", "hotfix_20261111123"}, {"documentation", "", "upstream-api", "", "docs_upstream-api"}, {"upstream", "", "", "20261111", "main_20261111"}} {
		got, err := OneSpecBranch(test.kind, test.id, test.description, test.date)
		if err != nil || got != test.want {
			t.Fatalf("%+v: %s %v", test, got, err)
		}
	}
	if _, err := OneSpecBranch("documentation", "", "../bad", ""); err == nil {
		t.Fatal("unsafe branch accepted")
	}
}
func TestOneSpecGitIdempotency(t *testing.T) {
	flow := OneSpecFlow{}
	op := OneSpecGitOperation{Worktree: "/tmp/onespec-docs", Key: "docs", Kind: "documentation", RepositoryID: "repo", Branch: "docs_upstream", BaseCommit: strings.Repeat("a", 40), Status: "prepared", Summary: "Prepared"}
	if err := flow.RecordGit(op, ""); err != nil {
		t.Fatal(err)
	}
	if err := flow.RecordGit(op, ""); err != nil || len(flow.GitOperations) != 1 {
		t.Fatal("retry duplicated operation")
	}
	changed := op
	changed.Worktree = "/tmp/other-task"
	if err := flow.RecordGit(changed, ""); err == nil {
		t.Fatal("operation changed its worktree ownership")
	}
	op.Status = "completed"
	if err := flow.RecordGit(op, ""); err != nil {
		t.Fatal(err)
	}
	op.Branch = "docs_other"
	if err := flow.RecordGit(op, ""); err == nil {
		t.Fatal("completed operation overwritten")
	}
}
func TestOneSpecLegacyRoles(t *testing.T) {
	if _, err := ParseProjectRoles([]byte(`{"v":1}`)); err == nil {
		t.Fatal("missing roles accepted")
	}
	raw := []byte(`{"v":1,"roles":{"sched":{"enabled":true},"product":{"enabled":true},"dev":{},"test":{},"sec":{},"ops":{}}}`)
	roles, err := ParseProjectRoles(raw)
	if err != nil || roles.Roles["docs"].Enabled || len(roles.Roles) != 8 {
		t.Fatalf("legacy roles: %+v %v", roles, err)
	}
}
