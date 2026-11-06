package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestRequirementsAuditFailureRollsBackIssue(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	id := dbfx.Issue(t, "Audit rollback", testutil.Cols{"status": "todo"})
	ctx := context.Background()
	before, err := testHandler.Queries.GetIssue(ctx, parseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = testHandler.updateIssueAtomically(ctx, parseUUID(testWorkspaceID), db.UpdateIssueParams{
		ID: parseUUID(id), AcceptanceCriteria: []byte(`{"v":1,"items":[{"id":"A-01","text":"Report"}]}`),
	}, nil, nil, nil, nil, "", false)
	if err == nil || !strings.Contains(err.Error(), "requirements audit requires an authenticated actor") {
		t.Fatalf("expected audit failure, got %v", err)
	}
	after, err := testHandler.Queries.GetIssue(ctx, parseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision || !bytes.Equal(after.AcceptanceCriteria, before.AcceptanceCriteria) {
		t.Fatal("failed audit committed the requirements or revision")
	}
}

func TestRequirementsListProjectionsKeepConfirmation(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	projectID := dbfx.Project(t, "Requirements projection")
	raw := []byte(`{"v":1,"version":1,"items":[{"id":"A-01","text":"Report"}],"bounds":["No billing"],"gate":{"spec":{"passed_by":"member","passed_at":"2026-11-06T00:00:00Z"}}}`)
	id := dbfx.Issue(t, "Confirmed list task", testutil.Cols{"project_id": projectID, "acceptance_criteria": raw, "external_ticket_no": "LIST-27"})
	assertCriteria := func(issues []IssueResponse) {
		t.Helper()
		for _, issue := range issues {
			if issue.ID != id {
				continue
			}
			criteria, err := workbench.ParseRequirements(issue.AcceptanceCriteria)
			if issue.ExternalTicketNo != "LIST-27" {
				t.Fatal("list lost external ticket number")
			}
			if err != nil || criteria.Gate.Spec == nil || len(criteria.Bounds) != 1 {
				t.Fatalf("list lost criteria: %s", issue.AcceptanceCriteria)
			}
			return
		}
		t.Fatal("created issue missing from list")
	}
	var list struct {
		Issues []IssueResponse `json:"issues"`
	}
	testutil.Call(t, testHandler.ListIssues, newRequest("GET", "/api/issues?project_id="+projectID, nil)).Want(http.StatusOK).JSON(&list)
	assertCriteria(list.Issues)
	var grouped GroupedIssuesResponse
	testutil.Call(t, testHandler.ListGroupedIssues, newRequest("GET", "/api/issues/grouped?group_by=assignee&project_id="+projectID, nil)).Want(http.StatusOK).JSON(&grouped)
	var issues []IssueResponse
	for _, group := range grouped.Groups {
		issues = append(issues, group.Issues...)
	}
	assertCriteria(issues)
}

func TestRequirementsPersistenceRevisionAndLock(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	id := dbfx.Issue(t, "Requirements persistence", testutil.Cols{"status": "todo"})
	var revision int64
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id = $1`, id).Scan(&revision)
	write := func(body map[string]any, status int) IssueResponse {
		t.Helper()
		result := testutil.Call(t, testHandler.UpdateIssue, testutil.WithURLParams(newRequest(http.MethodPut, "/api/issues/"+id, body), "id", id)).Want(status)
		var out IssueResponse
		if status == http.StatusOK {
			result.JSON(&out)
		}
		return out
	}
	input := map[string]any{"v": 1, "items": []map[string]any{{"text": "User can export a report"}}, "bounds": []string{"No billing"}}
	write(map[string]any{"acceptance_criteria": input}, http.StatusBadRequest)
	saved := write(map[string]any{"expected_revision": revision, "acceptance_criteria": input}, http.StatusOK)
	criteria, err := workbench.ParseRequirements(saved.AcceptanceCriteria)
	if err != nil || len(criteria.Items) != 1 || criteria.Items[0].ID != "A-01" {
		t.Fatalf("saved criteria: %s, %v", saved.AcceptanceCriteria, err)
	}
	write(map[string]any{"expected_revision": revision, "acceptance_criteria": input}, http.StatusConflict)
	criteria.Items[0].Checked = &workbench.Check{At: "forged", By: "forged"}
	confirmed := write(map[string]any{"expected_revision": saved.Revision, "acceptance_criteria": criteria, "confirm_requirements": true}, http.StatusOK)
	criteria, err = workbench.ParseRequirements(confirmed.AcceptanceCriteria)
	if err != nil || criteria.Gate.Spec == nil || criteria.Items[0].Checked.By != testUserID {
		t.Fatalf("confirmation: %s, %v", confirmed.AcceptanceCriteria, err)
	}
	write(map[string]any{"expected_revision": confirmed.Revision, "acceptance_criteria": input}, http.StatusBadRequest)
	var persisted []byte
	dbfx.QueryRow(t, `SELECT acceptance_criteria FROM issue WHERE id = $1`, id).Scan(&persisted)
	reloaded, err := workbench.ParseRequirements(persisted)
	if err != nil || reloaded.Gate.Spec == nil || len(reloaded.Bounds) != 1 {
		t.Fatalf("persisted criteria: %s, %v", persisted, err)
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM activity_log WHERE issue_id = $1 AND action = 'requirements_updated'`, id).Scan(&count)
	if count != 2 {
		t.Fatalf("audit events = %d, want 2 committed edits", count)
	}
}

func TestRequirementsCreatedAtomicallyWithIssue(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	input := map[string]any{"v": 1, "items": []map[string]any{{"text": "Export report", "checked": map[string]string{"at": "forged", "by": "forged"}}}, "bounds": []string{"No payments"}}
	created := createIssueWithPropertyBody(t, map[string]any{"title": "Create with requirements", "acceptance_criteria": input})
	criteria, err := workbench.ParseRequirements(created.AcceptanceCriteria)
	if err != nil || len(criteria.Items) != 1 || criteria.Items[0].ID != "A-01" || criteria.Items[0].Checked != nil {
		t.Fatalf("created criteria: %s, %v", created.AcceptanceCriteria, err)
	}
	var persisted []byte
	dbfx.QueryRow(t, `SELECT acceptance_criteria FROM issue WHERE id=$1`, created.ID).Scan(&persisted)
	stored, err := workbench.ParseRequirements(persisted)
	if err != nil || len(stored.Bounds) != 1 {
		t.Fatalf("stored criteria: %s, %v", persisted, err)
	}
	testutil.Call(t, testHandler.CreateIssue, newRequest(http.MethodPost, "/api/issues?workspace_id="+testWorkspaceID,
		map[string]any{"title": "Invalid running requirements", "status": "in_progress", "acceptance_criteria": input})).Want(http.StatusBadRequest)
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title='Invalid running requirements'`, testWorkspaceID).Scan(&count)
	if count != 0 {
		t.Fatal("failed requirements create left an issue behind")
	}
}

func TestRequirementsExecutionGateCannotBeBypassedByStatusWriter(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	criteria := workbench.Requirements{V: 1, Items: []workbench.Item{{ID: "A-01", Text: "Export report", Origin: "user"}}}
	raw, _ := json.Marshal(criteria)
	id := dbfx.Issue(t, "Guarded execution", testutil.Cols{"status": "todo", "acceptance_criteria": raw, "assignee_type": "member", "assignee_id": testUserID})
	move := func(status string) error {
		_, err := testHandler.Queries.UpdateIssueStatus(context.Background(), db.UpdateIssueStatusParams{ID: parseUUID(id), WorkspaceID: parseUUID(testWorkspaceID), Status: status})
		return err
	}
	assertBlocked := func(err error) {
		t.Helper()
		var constraint *pgconn.PgError
		if !errors.As(err, &constraint) || constraint.ConstraintName != "workbench_execution_gate" {
			t.Fatalf("expected gate rejection, got %v", err)
		}
	}
	assertBlocked(move("in_progress"))
	customKey := "wb_started_" + strings.ReplaceAll(id, "-", "")[:20]
	if _, err := testPool.Exec(context.Background(), `INSERT INTO issue_status (workspace_id, key, name, description, category, color, position) VALUES ($1,$2,'Workbench executing','','started','#000000',1)`, testWorkspaceID, customKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_status WHERE workspace_id=$1 AND key=$2`, testWorkspaceID, customKey)
	})
	assertBlocked(move(customKey))
	_, insertErr := testPool.Exec(context.Background(), `INSERT INTO issue (workspace_id,number,title,status,creator_type,creator_id,acceptance_criteria) SELECT workspace_id,number+1000000,'Invalid inserted execution','in_progress',creator_type,creator_id,acceptance_criteria FROM issue WHERE id=$1`, id)
	assertBlocked(insertErr)
	criteria.Gate.Spec = &workbench.GateCheck{PassedAt: "2026-11-06T00:00:00Z", PassedBy: testUserID}
	criteria.Items[0].Checked = &workbench.Check{At: "2026-11-06T00:00:00Z", By: testUserID}
	persist := func() {
		t.Helper()
		raw, _ := json.Marshal(criteria)
		if _, err := testPool.Exec(context.Background(), `UPDATE issue SET acceptance_criteria=$1 WHERE id=$2`, raw, id); err != nil {
			t.Fatal(err)
		}
	}
	persist()
	assertBlocked(move("in_progress"))
	other := dbfx.Issue(t, "Unrelated task", testutil.Cols{})
	criteria.Items[0].Covers = []string{other}
	persist()
	assertBlocked(move("in_progress"))
	child := dbfx.Issue(t, "Covered subtask", testutil.Cols{"parent_issue_id": id})
	criteria.Items[0].Covers = []string{child}
	persist()
	if err := move("in_progress"); err != nil {
		t.Fatal(err)
	}
	assertBlocked(move("done"))
	if err := move("in_review"); err != nil {
		t.Fatal(err)
	}
	assertBlocked(move("done"))
	criteria.Gate.Accept = &workbench.GateCheck{PassedAt: "2026-11-06T00:00:00Z", PassedBy: testUserID}
	persist()
	if err := move("done"); err != nil {
		t.Fatal(err)
	}
}
