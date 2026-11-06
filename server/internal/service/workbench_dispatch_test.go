package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestWorkbenchDispatchRecoversWithoutDuplicateRuns(t *testing.T) {
	f, owner := newPrincipalFixture(t)
	ctx := context.Background()
	if err := f.q.SeedIssueStatusEntries(ctx, parseTestUUID(t, f.WorkspaceID)); err != nil {
		t.Fatal(err)
	}
	agent := f.privateAgentOwnedBy(t, owner, "workbench")
	project := f.Insert(t, "project", testutil.Cols{"workspace_id": f.WorkspaceID, "title": "Workbench dispatch"})
	parent := f.Issue(t, "Confirmed requirement", testutil.Cols{"project_id": project})
	createChild := func(title string, depends []string) string {
		t.Helper()
		handoff, _ := json.Marshal(map[string]any{"workbench_handoff": workbench.Handoff{
			V: 2, RequirementVersion: 1, Role: "test", Covers: []string{"A-01"}, DependsOn: depends,
			SerialGroup: "checks", MaxRetry: 2, Expected: []string{"report"},
		}})
		id := f.Issue(t, title, testutil.Cols{"project_id": project, "parent_issue_id": parent,
			"properties": string(handoff), "assignee_type": "agent", "assignee_id": agent})
		f.Cleanup(t, "DELETE FROM agent_task_queue WHERE issue_id=$1", id)
		return id
	}
	first := createChild("First test", []string{})
	dependent := createChild("Dependent test", []string{first})
	serial := createChild("Same serial group", []string{})
	criteria, _ := json.Marshal(workbench.Requirements{V: 1, Version: 1, NextItemNo: 2,
		Items: []workbench.Item{{ID: "A-01", Text: "Check export", Covers: []string{first, dependent, serial}, Checked: &workbench.Check{By: owner, At: "confirmed"}}},
		Gate:  workbench.Gates{Spec: &workbench.GateCheck{PassedAt: "confirmed", PassedBy: owner}},
		Plan:  &workbench.ExecutionPlan{Version: 1, Issues: []string{first, dependent, serial}},
	})
	f.Exec(t, "UPDATE issue SET acceptance_criteria=$1 WHERE id=$2", criteria, parent)
	for _, suspended := range []struct {
		name string
		sql  string
	}{
		{"paused parent", `UPDATE issue SET status='paused',acceptance_criteria=jsonb_set(acceptance_criteria,'{change}','{"phase":"paused"}') WHERE id=$1`},
		{"new requirement awaiting replan", "UPDATE issue SET acceptance_criteria=jsonb_set(acceptance_criteria,'{needs_resplit}','true') WHERE id=$1"},
		{"cancelled parent", "UPDATE issue SET status='cancelled' WHERE id=$1"},
		{"resume still pending", `UPDATE issue SET properties=jsonb_set(properties,'{workbench_pause}','{"phase":"resuming"}') WHERE id=$1`},
	} {
		t.Run(suspended.name, func(t *testing.T) {
			f.Exec(t, suspended.sql, parent)
			if err := f.svc.TaskSvc.DispatchWorkbenchPlans(ctx); err != nil {
				t.Fatal(err)
			}
			var count int
			f.QueryRow(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id IN ($1,$2,$3)", first, dependent, serial).Scan(&count)
			if count != 0 {
				t.Fatal("non-executable plan was dispatched")
			}
			f.Exec(t, "UPDATE issue SET status='todo',acceptance_criteria=$1,properties=properties-'workbench_pause' WHERE id=$2", criteria, parent)
		})
	}
	f.Exec(t, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", f.WorkspaceID, owner)
	if err := f.svc.TaskSvc.DispatchWorkbenchPlans(ctx); err != nil {
		t.Fatal(err)
	}
	var unauthorized int
	f.QueryRow(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id IN ($1,$2,$3)", first, dependent, serial).Scan(&unauthorized)
	if unauthorized != 0 {
		t.Fatal("revoked dispatch permission still invoked an agent")
	}
	f.Member(t, f.WorkspaceID, owner, "owner")
	// A concurrent scheduler or a restart discovers the same durable plan.
	var workers sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			errors <- f.svc.TaskSvc.DispatchWorkbenchPlans(ctx)
		}()
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	f.QueryRow(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id IN ($1,$2,$3)", first, dependent, serial).Scan(&count)
	if count != 1 {
		t.Fatalf("concurrent dispatch created %d runs, want one serial reservation", count)
	}
	var running string
	f.QueryRow(t, "SELECT issue_id::text FROM agent_task_queue WHERE issue_id IN ($1,$2,$3)", first, dependent, serial).Scan(&running)
	if running == dependent {
		t.Fatal("dependent task dispatched before its predecessor completed")
	}
	// Completing both independent tasks releases the dependency even after the
	// original service instance has lost all in-memory notification state.
	for _, id := range []string{first, serial} {
		f.Exec(t, "UPDATE agent_task_queue SET status='completed' WHERE issue_id=$1", id)
		f.Exec(t, "UPDATE issue SET status='done' WHERE id=$1", id)
	}
	if err := f.svc.TaskSvc.DispatchWorkbenchPlans(ctx); err != nil {
		t.Fatal(err)
	}
	f.QueryRow(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", dependent).Scan(&count)
	if count != 1 {
		t.Fatalf("dependency completion created %d runs, want one", count)
	}
	f.Exec(t, "UPDATE agent_task_queue SET status='cancelled' WHERE issue_id=$1", dependent)
	if err := f.svc.TaskSvc.DispatchWorkbenchPlans(ctx); err != nil {
		t.Fatal(err)
	}
	f.QueryRow(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", dependent).Scan(&count)
	if count != 1 {
		t.Fatal("scheduler recreated a deliberately cancelled execution")
	}
}
