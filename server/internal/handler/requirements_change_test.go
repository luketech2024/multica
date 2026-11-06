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

func TestRequirementsChangeStopsExecutionBeforeUnlocking(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	ctx := context.Background()
	parent := dbfx.Issue(t, "Requirement change", testutil.Cols{"assignee_type": "member", "assignee_id": testUserID})
	child := dbfx.Issue(t, "Covered work", testutil.Cols{"parent_issue_id": parent})
	criteria := workbench.Requirements{V: 1, Version: 1, NextItemNo: 2, Items: []workbench.Item{{ID: "A-01", Text: "Version one", Origin: "user", Covers: []string{child}, Checked: &workbench.Check{At: "before", By: testUserID}}}, Bounds: []string{"No billing"}, Gate: workbench.Gates{Spec: &workbench.GateCheck{PassedAt: "before", PassedBy: testUserID}}}
	raw, _ := json.Marshal(criteria)
	if _, err := testPool.Exec(ctx, `UPDATE issue SET acceptance_criteria=$1,status='in_progress' WHERE id=$2`, raw, parent); err != nil {
		t.Fatal(err)
	}
	agent := dbfx.Agent(t, "Change executor", testRuntimeID)
	run := dbfx.Task(t, agent, testutil.Cols{"issue_id": child, "runtime_id": testRuntimeID, "status": "running", "started_at": testutil.Raw("now()")})
	read := func() db.Issue {
		t.Helper()
		value, err := testHandler.Queries.GetIssue(ctx, parseUUID(parent))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	act := func(action string, revision int64, status int) IssueResponse {
		t.Helper()
		request := testutil.WithURLParams(newRequest("POST", "/api/issues/"+parent+"/requirements-change", map[string]any{"action": action, "reason": "Adjust export behavior", "expected_revision": revision}), "id", parent)
		response := testutil.Call(t, testHandler.ChangeIssueRequirements, request).Want(status)
		var issue IssueResponse
		if status == http.StatusOK {
			response.JSON(&issue)
		}
		return issue
	}
	before := read()
	paused := act("start", before.Revision, http.StatusOK)
	stored, _ := workbench.ParseRequirements(paused.AcceptanceCriteria)
	if paused.Status != "paused" || stored.Change == nil || stored.Change.Phase != "stopping" {
		t.Fatalf("unexpected pause: %s", paused.AcceptanceCriteria)
	}
	act("start", before.Revision, http.StatusConflict)
	act("edit", paused.Revision, http.StatusConflict)
	act("reconcile", paused.Revision, http.StatusConflict)
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status='running' WHERE id=$1`, run); err == nil {
		t.Fatal("paused child run resumed through the task status writer")
	}
	if err := testHandler.Queries.AcknowledgeWorkbenchStop(ctx, db.AcknowledgeWorkbenchStopParams{ID: parseUUID(run), Confirmed: false}); err != nil {
		t.Fatal(err)
	}
	unknown := act("reconcile", paused.Revision, http.StatusOK)
	stored, _ = workbench.ParseRequirements(unknown.AcceptanceCriteria)
	if stored.Change.Phase != "unknown" {
		t.Fatal("unconfirmed cleanup was treated as stopped")
	}
	act("edit", unknown.Revision, http.StatusConflict)
	act("cancel", unknown.Revision, http.StatusConflict)
	if err := testHandler.Queries.AcknowledgeWorkbenchStop(ctx, db.AcknowledgeWorkbenchStopParams{ID: parseUUID(run), Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	settled := act("reconcile", unknown.Revision, http.StatusOK)
	edited := act("edit", settled.Revision, http.StatusOK)
	stored, _ = workbench.ParseRequirements(edited.AcceptanceCriteria)
	if stored.Version != 2 || stored.Items[0].Checked != nil || stored.Gate.Spec != nil || len(stored.Items[0].Covers) != 0 {
		t.Fatal("previous checks or coverage survived")
	}
	restored := act("cancel", edited.Revision, http.StatusOK)
	stored, _ = workbench.ParseRequirements(restored.AcceptanceCriteria)
	if restored.Status != "in_progress" || stored.Change != nil || stored.Items[0].Text != "Version one" || stored.Gate.Spec == nil {
		t.Fatal("cancel did not restore confirmed baseline")
	}
	if len(stored.ResumeRuns) != 0 || stored.ResumeError != "" {
		t.Fatalf("cancel failed to resume execution: %s", restored.AcceptanceCriteria)
	}
	act("resume", restored.Revision, http.StatusOK)
	var resumedCount int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE rerun_of_task_id=$1`, run).Scan(&resumedCount)
	if resumedCount != 1 {
		t.Fatalf("resume retry created %d runs, want one", resumedCount)
	}
	paused = act("start", restored.Revision, http.StatusOK)
	edited = act("edit", paused.Revision, http.StatusOK)
	stored, _ = workbench.ParseRequirements(edited.AcceptanceCriteria)
	stored.Items[0].Text = "Version three"
	write := func(input workbench.Requirements, revision int64, confirm bool) IssueResponse {
		t.Helper()
		request := testutil.WithURLParams(newRequest("PUT", "/api/issues/"+parent, map[string]any{"acceptance_criteria": input, "expected_revision": revision, "confirm_requirements": confirm}), "id", parent)
		var response IssueResponse
		testutil.Call(t, testHandler.UpdateIssue, request).Want(http.StatusOK).JSON(&response)
		return response
	}
	saved := write(stored, edited.Revision, false)
	stored, _ = workbench.ParseRequirements(saved.AcceptanceCriteria)
	stored.Items[0].Checked = &workbench.Check{}
	confirmed := write(stored, saved.Revision, true)
	stored, _ = workbench.ParseRequirements(confirmed.AcceptanceCriteria)
	if confirmed.Status != "backlog" || stored.Change != nil || stored.Gate.Spec == nil || !stored.NeedsResplit {
		t.Fatal("reconfirmation did not return to scheduling")
	}
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status='running' WHERE id=$1`, run); err == nil {
		t.Fatal("old subtask ran before the new requirements were replanned")
	}
}
