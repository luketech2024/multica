package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestWorkbenchReviewReconcilesOnceAndPreservesSuspendedTasks(t *testing.T) {
	f, owner := newPrincipalFixture(t)
	ctx := context.Background()
	if err := f.q.SeedIssueStatusEntries(ctx, parseTestUUID(t, f.WorkspaceID)); err != nil {
		t.Fatal(err)
	}
	agent := f.privateAgentOwnedBy(t, owner, "review")
	project := f.Insert(t, "project", testutil.Cols{"workspace_id": f.WorkspaceID, "title": "Review aggregation"})
	parent := f.Issue(t, "Review parent", testutil.Cols{"project_id": project, "status": "in_progress", "assignee_type": "agent", "assignee_id": agent})
	handoff, _ := json.Marshal(map[string]any{"workbench_handoff": workbench.Handoff{
		V: 2, RequirementVersion: 1, Role: "test", Covers: []string{"A-01"}, DependsOn: []string{}, Expected: []string{"report"},
	}})
	child := f.Issue(t, "Test report", testutil.Cols{"project_id": project, "parent_issue_id": parent, "status": "done", "properties": handoff})
	childIssue, err := f.q.GetIssue(ctx, parseTestUUID(t, child))
	if err != nil {
		t.Fatal(err)
	}
	roles := workbench.DefaultProjectRoles()
	for _, role := range []string{"dev", "sec", "ops"} {
		roles.Roles[role] = workbench.ProjectRole{}
	}
	criteria := workbench.Requirements{V: 1, Version: 1, NextItemNo: 2,
		Items: []workbench.Item{{ID: "A-01", Text: "Export", Checked: &workbench.Check{By: owner, At: "now"}, Covers: []string{child}}},
		Gate:  workbench.Gates{Spec: &workbench.GateCheck{PassedAt: "now", PassedBy: owner}},
		Team:  &workbench.ProjectTeamSnapshot{ProjectRoleConfiguration: workbench.ProjectRoleConfiguration{Config: roles}},
		Plan:  &workbench.ExecutionPlan{Version: 1, Issues: []string{child}},
		Delivery: map[string]workbench.DeliveryEvidence{child: {
			IssueID: child, IssueRevision: childIssue.Revision, Version: 1, Role: "test", Passed: true, Summary: "passed",
			DescriptionHash: workbench.DeliveryDescriptionHash(""), RecordedAt: "now", RecordedBy: owner,
			Reports: []workbench.DeliveryReport{{Name: "report", URL: "https://example.test/report"}},
		}},
	}
	raw, _ := json.Marshal(criteria)
	f.Exec(t, "UPDATE issue SET acceptance_criteria=$1 WHERE id=$2", raw, parent)
	f.Cleanup(t, "DELETE FROM inbox_item WHERE issue_id=$1", parent)
	for _, suspend := range []string{
		`UPDATE issue SET status='paused',acceptance_criteria=jsonb_set(acceptance_criteria,'{change}','{"phase":"paused"}') WHERE id=$1`,
		"UPDATE issue SET status='cancelled' WHERE id=$1",
		`UPDATE issue SET properties='{"workbench_pause":{"phase":"stopping"}}' WHERE id=$1`,
		`UPDATE issue SET acceptance_criteria=jsonb_set(acceptance_criteria,'{needs_resplit}','true') WHERE id=$1`,
		`UPDATE issue SET acceptance_criteria=jsonb_set(acceptance_criteria,'{delivery}','{}') WHERE id=$1`,
	} {
		f.Exec(t, suspend, parent)
		if err := f.svc.TaskSvc.ReconcileWorkbenchReviews(ctx); err != nil {
			t.Fatal(err)
		}
		var count int
		f.QueryRow(t, "SELECT count(*) FROM inbox_item WHERE issue_id=$1", parent).Scan(&count)
		if count != 0 {
			t.Fatal("suspended or incomplete delivery was sent for review")
		}
		f.Exec(t, "UPDATE issue SET status='in_progress',properties='{}',acceptance_criteria=$1 WHERE id=$2", raw, parent)
	}
	var runtimeID string
	f.QueryRow(t, "SELECT runtime_id::text FROM agent WHERE id=$1", agent).Scan(&runtimeID)
	run := f.Task(t, agent, testutil.Cols{"issue_id": parent, "runtime_id": runtimeID})
	if err := f.svc.TaskSvc.ReconcileWorkbenchReviews(ctx); err != nil {
		t.Fatal(err)
	}
	stillRunning, err := f.q.GetIssue(ctx, parseTestUUID(t, parent))
	if err != nil || stillRunning.Status != "in_progress" {
		t.Fatal("active execution was sent for acceptance")
	}
	f.Exec(t, "UPDATE agent_task_queue SET status='completed' WHERE id=$1", run)
	f.Exec(t, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", f.WorkspaceID, owner)
	if err := f.svc.TaskSvc.ReconcileWorkbenchReviews(ctx); err != nil {
		t.Fatal(err)
	}
	unassignedReview, err := f.q.GetIssue(ctx, parseTestUUID(t, parent))
	if err != nil || unassignedReview.Status != "in_progress" {
		t.Fatal("review was assigned to a removed member")
	}
	f.Member(t, f.WorkspaceID, owner, "owner")
	var workers sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			errors <- f.svc.TaskSvc.ReconcileWorkbenchReviews(ctx)
		}()
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.TaskSvc.ReconcileWorkbenchReviews(ctx); err != nil {
		t.Fatal(err)
	}
	updated, err := f.q.GetIssue(ctx, parseTestUUID(t, parent))
	if err != nil || updated.Status != "in_review" || updated.AssigneeType.String != "member" || updated.AssigneeID != parseTestUUID(t, owner) {
		t.Fatalf("review assignment not persisted: %+v %v", updated, err)
	}
	var notifications, activities int
	f.QueryRow(t, "SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND type='review_requested' AND recipient_id=$2", parent, owner).Scan(&notifications)
	f.QueryRow(t, "SELECT count(*) FROM activity_log WHERE issue_id=$1 AND details->>'source'='workbench_delivery'", parent).Scan(&activities)
	if notifications != 1 || activities != 1 {
		t.Fatalf("concurrent/replayed reconciliation duplicated effects: notifications=%d activities=%d", notifications, activities)
	}
	securityHandoff, _ := json.Marshal(map[string]any{"workbench_handoff": workbench.Handoff{
		V: 2, RequirementVersion: 1, Role: "sec", Covers: []string{"A-01"}, DependsOn: []string{}, Expected: []string{"report"},
	}})
	security := f.Issue(t, "Security report", testutil.Cols{"project_id": project, "parent_issue_id": parent, "status": "done", "properties": securityHandoff})
	securityIssue, err := f.q.GetIssue(ctx, parseTestUUID(t, security))
	if err != nil {
		t.Fatal(err)
	}
	criteria.Team.Config.Roles["sec"] = workbench.ProjectRole{Enabled: true}
	criteria.Plan.Issues = append(criteria.Plan.Issues, security)
	criteria.Items[0].Covers = append(criteria.Items[0].Covers, security)
	criteria.Delivery[security] = workbench.DeliveryEvidence{
		IssueID: security, IssueRevision: securityIssue.Revision, Version: 1, Role: "sec", Passed: false, Summary: "Security failed",
		DescriptionHash: workbench.DeliveryDescriptionHash(""), RecordedAt: "now", RecordedBy: owner,
		Reports: []workbench.DeliveryReport{{Name: "report", URL: "https://example.test/security"}},
	}
	raw, _ = json.Marshal(criteria)
	f.Exec(t, "UPDATE issue SET acceptance_criteria=$1 WHERE id=$2", raw, parent)
	for range 2 {
		if err := f.svc.TaskSvc.ReconcileWorkbenchReviews(ctx); err != nil {
			t.Fatal(err)
		}
	}
	blocked, err := f.q.GetIssue(ctx, parseTestUUID(t, parent))
	if err != nil || blocked.Status != "blocked" {
		t.Fatal("conflicting fresh role reports did not block acceptance")
	}
	f.QueryRow(t, "SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND details->>'reason'='test_security_conflict'", parent).Scan(&notifications)
	if notifications != 1 {
		t.Fatalf("expected one conflict notification, got %d", notifications)
	}
}
