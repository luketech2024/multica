package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func controlExecution(t *testing.T, id, action string, revision int64, status int) IssueResponse {
	t.Helper()
	req := testutil.WithURLParams(newRequest("POST", "/api/issues/"+id+"/execution-control", map[string]any{"action": action, "expected_revision": revision}), "id", id)
	result := testutil.Call(t, testHandler.ControlIssueExecution, req).Want(status)
	var issue IssueResponse
	if status == http.StatusOK {
		result.JSON(&issue)
	}
	return issue
}

func TestExecutionControlPreservesUnstartedRequirements(t *testing.T) {
	ctx := context.Background()
	criteria := workbench.Requirements{V: 1, Version: 3, NextItemNo: 4,
		Items: []workbench.Item{{ID: "A-03", Text: "Unconfirmed work"}}, Bounds: []string{"No billing"}}
	raw, _ := json.Marshal(criteria)
	id := dbfx.Issue(t, "Pause before planning", testutil.Cols{"acceptance_criteria": string(raw), "properties": `{"custom":"keep"}`})
	before, err := testHandler.Queries.GetIssue(ctx, parseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	paused := controlExecution(t, id, "pause", before.Revision, http.StatusOK)
	if paused.Status != "paused" {
		t.Fatal("manual pause altered requirement data")
	}
	assertJSONEqual(t, paused.AcceptanceCriteria, string(before.AcceptanceCriteria))
	var record workbench.ExecutionPause
	if json.Unmarshal(paused.WorkbenchPause, &record) != nil || record.Phase != "paused" {
		t.Fatal("pause response omitted its stop state")
	}
	var loaded IssueResponse
	testutil.Call(t, testHandler.GetIssue, testutil.WithURLParams(newRequest("GET", "/api/issues/"+id, nil), "id", id)).Want(http.StatusOK).JSON(&loaded)
	if string(loaded.WorkbenchPause) != string(paused.WorkbenchPause) {
		t.Fatal("pause did not survive reload")
	}
	controlExecution(t, id, "resume", before.Revision, http.StatusConflict)
	if _, err = testPool.Exec(ctx, "UPDATE issue SET status='todo' WHERE id=$1", id); err == nil {
		t.Fatal("ordinary status update bypassed pause reconciliation")
	}
	resumed := controlExecution(t, id, "resume", paused.Revision, http.StatusOK)
	if resumed.Status != "todo" || len(resumed.WorkbenchPause) != 0 {
		t.Fatal("resumption skipped confirmation or changed requirements")
	}
	assertJSONEqual(t, resumed.AcceptanceCriteria, string(before.AcceptanceCriteria))
	var custom string
	dbfx.QueryRow(t, "SELECT properties->>'custom' FROM issue WHERE id=$1", id).Scan(&custom)
	if custom != "keep" {
		t.Fatal("execution control replaced custom properties")
	}
}

func TestExecutionControlAcknowledgesAndResumesOriginalRunsOnce(t *testing.T) {
	ctx := context.Background()
	project := dbfx.Insert(t, "project", testutil.Cols{"workspace_id": testWorkspaceID, "title": "Pause project"})
	parent := dbfx.Issue(t, "Pause running plan", testutil.Cols{"project_id": project, "assignee_type": "member", "assignee_id": testUserID})
	agent := dbfx.Agent(t, "Pause executor", testRuntimeID)
	handoff, _ := json.Marshal(map[string]any{"workbench_handoff": workbench.Handoff{V: 2, RequirementVersion: 1, Role: "test", Covers: []string{"A-01"}, DependsOn: []string{}, MaxRetry: 2, Expected: []string{"report"}}})
	child := dbfx.Issue(t, "Covered execution", testutil.Cols{"project_id": project, "parent_issue_id": parent, "assignee_type": "agent", "assignee_id": agent, "properties": string(handoff)})
	criteria, _ := json.Marshal(workbench.Requirements{V: 1, Version: 1, NextItemNo: 2,
		Items: []workbench.Item{{ID: "A-01", Text: "Confirmed work", Checked: &workbench.Check{By: testUserID, At: "confirmed"}, Covers: []string{child}}},
		Gate:  workbench.Gates{Spec: &workbench.GateCheck{PassedAt: "confirmed", PassedBy: testUserID}}, Plan: &workbench.ExecutionPlan{Version: 1, Issues: []string{child}},
	})
	dbfx.Exec(t, "UPDATE issue SET acceptance_criteria=$1,status='in_progress' WHERE id=$2", criteria, parent)
	run := dbfx.Task(t, agent, testutil.Cols{"issue_id": child, "runtime_id": testRuntimeID, "status": "running", "session_id": "pause-session", "branch_name": "feat/pause-test", "started_at": testutil.Raw("now()")})
	dbfx.Cleanup(t, "DELETE FROM agent_task_queue WHERE issue_id=$1", child)
	before, err := testHandler.Queries.GetIssue(ctx, parseUUID(parent))
	if err != nil {
		t.Fatal(err)
	}
	paused := controlExecution(t, parent, "pause", before.Revision, http.StatusOK)
	var record workbench.ExecutionPause
	_ = json.Unmarshal(paused.WorkbenchPause, &record)
	if record.Phase != "stopping" {
		t.Fatal("running executor was treated as stopped")
	}
	controlExecution(t, parent, "resume", paused.Revision, http.StatusConflict)
	if _, err = testPool.Exec(ctx, "UPDATE agent_task_queue SET status='running' WHERE id=$1", run); err == nil {
		t.Fatal("child run ignored the parent pause")
	}
	if err = testHandler.Queries.AcknowledgeWorkbenchStop(ctx, db.AcknowledgeWorkbenchStopParams{ID: parseUUID(run), Confirmed: false}); err != nil {
		t.Fatal(err)
	}
	unknown := controlExecution(t, parent, "reconcile", paused.Revision, http.StatusOK)
	_ = json.Unmarshal(unknown.WorkbenchPause, &record)
	if record.Phase != "unknown" {
		t.Fatal("unknown stop result was lost")
	}
	controlExecution(t, parent, "resume", unknown.Revision, http.StatusConflict)
	if err = testHandler.Queries.AcknowledgeWorkbenchStop(ctx, db.AcknowledgeWorkbenchStopParams{ID: parseUUID(run), Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	settled := controlExecution(t, parent, "reconcile", unknown.Revision, http.StatusOK)
	resumed := controlExecution(t, parent, "resume", settled.Revision, http.StatusOK)
	if resumed.Status != "in_progress" || len(resumed.WorkbenchPause) != 0 {
		t.Fatalf("resume did not restore the original execution state: status=%s, pause=%s", resumed.Status, resumed.WorkbenchPause)
	}
	assertJSONEqual(t, resumed.AcceptanceCriteria, string(before.AcceptanceCriteria))
	controlExecution(t, parent, "resume", settled.Revision, http.StatusConflict)
	controlExecution(t, parent, "resume", resumed.Revision, http.StatusConflict)
	var count int
	dbfx.QueryRow(t, "SELECT count(*) FROM agent_task_queue WHERE rerun_of_task_id=$1", run).Scan(&count)
	if count != 1 {
		t.Fatalf("resume created %d runs, want one original-run continuation", count)
	}
	// A child has no requirement document, but still supports a durable pause.
	childBefore, err := testHandler.Queries.GetIssue(ctx, parseUUID(child))
	if err != nil {
		t.Fatal(err)
	}
	childPaused := controlExecution(t, child, "pause", childBefore.Revision, http.StatusOK)
	if childPaused.Status != "paused" {
		t.Fatal("subtask pause was not persisted")
	}
	if _, err = testPool.Exec(ctx, "UPDATE issue SET status='in_progress' WHERE id=$1", child); err == nil {
		t.Fatal("subtask resumed without execution control")
	}
	childResumed := controlExecution(t, child, "resume", childPaused.Revision, http.StatusOK)
	if childResumed.Status != childBefore.Status || len(childResumed.WorkbenchPause) != 0 {
		t.Fatal("subtask did not resume")
	}
}
