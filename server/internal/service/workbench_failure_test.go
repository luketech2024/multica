package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func workbenchFailureFixture(t *testing.T, retries int) (principalFixture, db.Issue, string) {
	t.Helper()
	f, owner := newPrincipalFixture(t)
	ctx := context.Background()
	if err := f.q.SeedIssueStatusEntries(ctx, parseTestUUID(t, f.WorkspaceID)); err != nil {
		t.Fatal(err)
	}
	agent := f.privateAgentOwnedBy(t, owner, "budget")
	project := f.Insert(t, "project", testutil.Cols{"workspace_id": f.WorkspaceID, "title": "Retry budget"})
	parent := f.Issue(t, "Requirement", testutil.Cols{"project_id": project})
	handoff, _ := json.Marshal(map[string]any{"workbench_handoff": workbench.Handoff{V: 2, RequirementVersion: 1,
		Role: "test", Covers: []string{"A-01"}, DependsOn: []string{}, MaxRetry: retries, Expected: []string{"report"}}})
	child := f.Issue(t, "Test export", testutil.Cols{"project_id": project, "parent_issue_id": parent,
		"properties": string(handoff), "assignee_type": "agent", "assignee_id": agent})
	f.Cleanup(t, "DELETE FROM agent_task_queue WHERE issue_id=$1", child)
	criteria, _ := json.Marshal(workbench.Requirements{V: 1, Version: 1, NextItemNo: 2,
		Items: []workbench.Item{{ID: "A-01", Text: "Export", Covers: []string{child}, Checked: &workbench.Check{By: owner, At: "confirmed"}}},
		Gate:  workbench.Gates{Spec: &workbench.GateCheck{PassedAt: "confirmed", PassedBy: owner}},
		Plan:  &workbench.ExecutionPlan{Version: 1, Issues: []string{child}},
	})
	f.Exec(t, "UPDATE issue SET acceptance_criteria=$1 WHERE id=$2", criteria, parent)
	issue, err := f.q.GetIssue(ctx, parseTestUUID(t, child))
	if err != nil {
		t.Fatal(err)
	}
	return f, issue, parent
}

func TestWorkbenchRetryBudgetBlocksAfterFinalAttempt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		retries int
		reason  string
	}{
		{"two infrastructure retries", 2, "timeout"},
		{"network policy cannot extend one retry", 1, "agent_error.provider_network"},
		{"retry disabled", 0, "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, issue, _ := workbenchFailureFixture(t, tc.retries)
			ctx := context.Background()
			run, err := f.svc.TaskSvc.EnqueueTaskForIssue(ctx, issue)
			if err != nil {
				t.Fatal(err)
			}
			f.Exec(t, "UPDATE issue SET status='in_progress' WHERE id=$1", issue.ID)
			for attempt := 1; attempt <= tc.retries+1; attempt++ {
				if int(run.Attempt) != attempt || int(run.MaxAttempts) != tc.retries+1 {
					t.Fatalf("run budget = %d/%d, want %d/%d", run.Attempt, run.MaxAttempts, attempt, tc.retries+1)
				}
				f.Exec(t, "UPDATE agent_task_queue SET status='running',started_at=now() WHERE id=$1", run.ID)
				if _, err := f.svc.TaskSvc.FailTask(ctx, run.ID, "Export worker timed out", "", "", "", tc.reason, false, "", ""); err != nil {
					t.Fatal(err)
				}
				var status string
				f.QueryRow(t, "SELECT status FROM issue WHERE id=$1", issue.ID).Scan(&status)
				if attempt <= tc.retries {
					if status != "in_progress" {
						t.Fatal("recoverable execution failure changed business status")
					}
					var next string
					f.QueryRow(t, "SELECT id::text FROM agent_task_queue WHERE parent_task_id=$1", run.ID).Scan(&next)
					run, err = f.q.GetAgentTask(ctx, parseTestUUID(t, next))
					if err != nil {
						t.Fatal(err)
					}
				} else if status != "blocked" {
					t.Fatalf("exhausted task status = %s, want blocked", status)
				}
			}
			var count, audits int
			f.QueryRow(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", issue.ID).Scan(&count)
			f.QueryRow(t, "SELECT count(*) FROM activity_log WHERE issue_id=$1 AND action='status_changed' AND details->>'to'='blocked' AND details->>'reason' <> ''", issue.ID).Scan(&audits)
			if count != tc.retries+1 || audits != 1 {
				t.Fatalf("runs/audits = %d/%d, want %d/1", count, audits, tc.retries+1)
			}
		})
	}
}

func TestWorkbenchFailureDuringChangePreservesFailureWithoutRetry(t *testing.T) {
	f, issue, parent := workbenchFailureFixture(t, 2)
	ctx := context.Background()
	run, err := f.svc.TaskSvc.EnqueueTaskForIssue(ctx, issue)
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, "UPDATE agent_task_queue SET status='running',started_at=now() WHERE id=$1", run.ID)
	f.Exec(t, `UPDATE issue SET status='paused',acceptance_criteria=jsonb_set(acceptance_criteria,'{change}','{"phase":"stopping"}') WHERE id=$1`, parent)
	failed, err := f.svc.TaskSvc.FailTask(ctx, run.ID, "Runtime stopped", "", "", "", "timeout", false, "", "")
	if err != nil || failed.Status != "failed" {
		t.Fatalf("refused retry rolled back the failure: %v", err)
	}
	var count int
	f.QueryRow(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", issue.ID).Scan(&count)
	if count != 1 {
		t.Fatal("paused requirement created a new execution")
	}
}

func TestWorkbenchSweptFailureBlocksAndDoesNotRestart(t *testing.T) {
	f, issue, _ := workbenchFailureFixture(t, 0)
	ctx := context.Background()
	run, err := f.svc.TaskSvc.EnqueueTaskForIssue(ctx, issue)
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, "UPDATE issue SET status='in_progress' WHERE id=$1", issue.ID)
	f.Exec(t, "UPDATE agent_task_queue SET status='failed',failure_reason='timeout',error='timeout' WHERE id=$1", run.ID)
	run, err = f.q.GetAgentTask(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.svc.TaskSvc.HandleFailedTasks(ctx, []db.AgentTaskQueue{run})
	f.svc.TaskSvc.HandleFailedTasks(ctx, []db.AgentTaskQueue{run})
	var status string
	var count int
	f.QueryRow(t, "SELECT status FROM issue WHERE id=$1", issue.ID).Scan(&status)
	f.QueryRow(t, "SELECT count(*) FROM activity_log WHERE issue_id=$1 AND details->>'to'='blocked'", issue.ID).Scan(&count)
	if status != "blocked" || count != 1 {
		t.Fatalf("swept state/audits = %s/%d, want blocked/1", status, count)
	}
}
