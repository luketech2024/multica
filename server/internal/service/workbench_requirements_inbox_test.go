package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestWorkbenchRequirementNoticesFollowVersionAssignmentAndConfirmation(t *testing.T) {
	f, owner := newPrincipalFixture(t)
	ctx := context.Background()
	parent := f.Issue(t, "Confirm requirements", testutil.Cols{"assignee_type": "member", "assignee_id": owner})
	f.Cleanup(t, "DELETE FROM inbox_item WHERE issue_id=$1", parent)
	criteria := workbench.Requirements{V: 1, Version: 1, NextItemNo: 2,
		Items: []workbench.Item{{ID: "A-01", Text: "Export all rows", Covers: []string{}}}, Bounds: []string{},
		Team: &workbench.ProjectTeamSnapshot{ProjectRoleConfiguration: workbench.ProjectRoleConfiguration{Config: workbench.DefaultProjectRoles()}},
	}
	write := func() {
		t.Helper()
		raw, err := json.Marshal(criteria)
		if err != nil {
			t.Fatal(err)
		}
		f.Exec(t, "UPDATE issue SET acceptance_criteria=$1 WHERE id=$2", raw, parent)
	}
	reconcile := func() {
		t.Helper()
		if err := f.svc.TaskSvc.ReconcileWorkbenchRequirements(ctx); err != nil {
			t.Fatal(err)
		}
	}
	assertCounts := func(total, unread int) {
		t.Helper()
		var got, gotUnread int
		f.QueryRow(t, "SELECT count(*), count(*) FILTER (WHERE NOT read) FROM inbox_item WHERE issue_id=$1 AND details->>'source'='workbench_requirements'", parent).Scan(&got, &gotUnread)
		if got != total || gotUnread != unread {
			t.Fatalf("notices=%d unread=%d, want %d/%d", got, gotUnread, total, unread)
		}
	}
	write()
	var workers sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		workers.Add(1)
		go func() { defer workers.Done(); errors <- f.svc.TaskSvc.ReconcileWorkbenchRequirements(ctx) }()
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertCounts(1, 1)
	f.Exec(t, "UPDATE inbox_item SET read=true WHERE issue_id=$1", parent)
	reconcile()
	assertCounts(1, 0)
	f.Exec(t, "UPDATE inbox_item SET read=false WHERE issue_id=$1", parent)
	mention := f.Insert(t, "inbox_item", testutil.Cols{"workspace_id": f.WorkspaceID, "recipient_type": "member", "recipient_id": owner,
		"issue_id": parent, "type": "mentioned", "severity": "info", "title": "Keep mention unread"})
	criteria.Gate.Spec = &workbench.GateCheck{PassedAt: "confirmed", PassedBy: owner}
	write()
	reconcile()
	assertCounts(1, 0)
	var read bool
	f.QueryRow(t, "SELECT read FROM inbox_item WHERE id=$1", mention).Scan(&read)
	if read {
		t.Fatal("confirmation consumed an unrelated mention")
	}
	criteria.Version = 2
	criteria.Gate.Spec = nil
	write()
	f.Exec(t, "UPDATE issue SET status='backlog' WHERE id=$1", parent)
	reconcile()
	assertCounts(2, 1)
	other := f.User(t, "Other reviewer", "reviewer-"+parent+"@example.test")
	f.Member(t, f.WorkspaceID, other, "member")
	f.Exec(t, "UPDATE issue SET assignee_id=$1 WHERE id=$2", other, parent)
	reconcile()
	assertCounts(3, 1)
	var recipient string
	f.QueryRow(t, "SELECT recipient_id::text FROM inbox_item WHERE issue_id=$1 AND details->>'source'='workbench_requirements' AND NOT read", parent).Scan(&recipient)
	if recipient != other {
		t.Fatal("requirement notice did not follow the new assignee")
	}
	f.Exec(t, "UPDATE issue SET status='cancelled' WHERE id=$1", parent)
	reconcile()
	assertCounts(3, 0)
	f.Exec(t, "UPDATE issue SET status='todo', assignee_type=NULL, assignee_id=NULL WHERE id=$1", parent)
	reconcile()
	assertCounts(3, 0)
}
