package handler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestIssueCompletionTimeTracksTransitionsAndSurvivesEdits(t *testing.T) {
	issue := dbfx.Issue(t, "Completion clock", testutil.Cols{"status": "todo"})
	read := func() pgtype.Timestamptz {
		var result pgtype.Timestamptz
		dbfx.QueryRow(t, "SELECT completed_at FROM issue WHERE id=$1", issue).Scan(&result)
		return result
	}
	if read().Valid {
		t.Fatal("unfinished issue has completion time")
	}
	dbfx.Exec(t, "UPDATE issue SET status='done' WHERE id=$1", issue)
	original := read()
	if !original.Valid {
		t.Fatal("completion time was not captured")
	}
	dbfx.Exec(t, "UPDATE issue SET title='Renamed', updated_at=now() WHERE id=$1", issue)
	if !read().Time.Equal(original.Time) {
		t.Fatal("renaming reset completion time")
	}
	dbfx.Exec(t, "UPDATE issue SET completed_at=now()+interval '1 day' WHERE id=$1", issue)
	if !read().Time.Equal(original.Time) {
		t.Fatal("client-controlled completion time accepted")
	}
	custom := createTestCustomStatus(t, "completion_test_done", "done")
	dbfx.Exec(t, "UPDATE issue SET status=$2 WHERE id=$1", issue, custom.Key)
	if !read().Time.Equal(original.Time) {
		t.Fatal("terminal status rename reset completion time")
	}
	dbfx.Exec(t, "UPDATE issue SET status='in_progress' WHERE id=$1", issue)
	if read().Valid {
		t.Fatal("reopened issue retained completion time")
	}
	dbfx.Exec(t, "UPDATE issue SET status='done' WHERE id=$1", issue)
	if !read().Time.After(original.Time) {
		t.Fatal("second completion reused the old timestamp")
	}
}

func TestIssueTableCompletionDateUsesCompletionInsteadOfUpdate(t *testing.T) {
	project := dbfx.Project(t, "Completion filter")
	before := time.Now().UTC().Add(-time.Minute)
	issue := dbfx.Issue(t, "Completed", testutil.Cols{"status": "done", "project_id": project})
	dbfx.Exec(t, "UPDATE issue SET updated_at=now()+interval '1 day' WHERE id=$1", issue)
	var result struct {
		Total int64 `json:"total"`
	}
	query := map[string]any{
		"scope":   map[string]any{"kind": "project", "project_id": project},
		"filters": map[string]any{"date": map[string]string{"field": "completed_at", "start": before.Format(time.RFC3339Nano), "end": time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano)}},
		"sort":    map[string]string{"field": "created_at", "direction": "asc"},
	}
	input := map[string]any{"query": query, "group": map[string]string{"kind": "none"}, "page": map[string]int{"limit": 10}}
	testutil.Call(t, testHandler.ListIssueTableRows, newRequest("POST", "/api/issues/table/rows", input)).Want(200).JSON(&result)
	if result.Total != 1 {
		t.Fatalf("completion date returned %d rows", result.Total)
	}
	dbfx.Exec(t, "UPDATE issue SET status='todo' WHERE id=$1", issue)
	testutil.Call(t, testHandler.ListIssueTableRows, newRequest("POST", "/api/issues/table/rows", input)).Want(200).JSON(&result)
	if result.Total != 0 {
		t.Fatal("reopened issue still counted as completed")
	}
}

func TestWorkbenchBandCountsAcrossPagesAndUsesMemberAssignment(t *testing.T) {
	project := dbfx.Project(t, "Status band")
	for _, status := range []string{"in_progress", "in_review", "done"} {
		dbfx.Issue(t, status, testutil.Cols{"project_id": project, "status": status, "assignee_type": "member", "assignee_id": testUserID})
	}
	for range 14 {
		dbfx.Issue(t, "Review", testutil.Cols{"project_id": project, "status": "in_review", "assignee_type": "member", "assignee_id": testUserID})
	}
	agent := dbfx.Agent(t, "Status band agent", dbfx.Runtime(t, "Status band runtime"))
	dbfx.Issue(t, "Agent running", testutil.Cols{"project_id": project, "status": "in_progress", "assignee_type": "agent", "assignee_id": agent})
	dbfx.Issue(t, "Unassigned running", testutil.Cols{"project_id": project, "status": "in_progress"})
	for _, tc := range []struct {
		band  string
		mine  bool
		count int64
	}{{"running", true, 1}, {"running", false, 3}, {"review", true, 15}, {"today", true, 1}} {
		query := map[string]any{
			"scope":   map[string]string{"kind": "project", "project_id": project},
			"filters": map[string]any{"workbench": map[string]any{"band": tc.band, "only_mine": tc.mine, "start": time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), "end": time.Now().UTC().Add(time.Hour).Format(time.RFC3339)}},
			"sort":    map[string]string{"field": "created_at", "direction": "asc"},
		}
		var rows struct {
			Total int64 `json:"total"`
		}
		input := map[string]any{"query": query, "group": map[string]string{"kind": "none"}, "page": map[string]int{"limit": 1}}
		testutil.Call(t, testHandler.ListIssueTableRows, newRequest("POST", "/api/issues/table/rows", input)).Want(200).JSON(&rows)
		if rows.Total != tc.count {
			t.Fatalf("%s mine=%v count=%d want=%d", tc.band, tc.mine, rows.Total, tc.count)
		}
		var facets struct {
			Total int64 `json:"total"`
		}
		testutil.Call(t, testHandler.ListIssueTableFacets, newRequest("POST", "/api/issues/table/facets", map[string]any{"query": query, "facets": []map[string]string{{"kind": "status"}}, "include_total": true})).Want(200).JSON(&facets)
		if facets.Total != rows.Total {
			t.Fatal("count and list disagree")
		}
	}
}
