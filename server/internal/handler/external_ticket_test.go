package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestExternalTicketCreateEditClearAndDuplicate(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	first := createIssueWithPropertyBody(t, map[string]any{"title": "Ticket one", "external_ticket_no": "XQ-3482"})
	second := createIssueWithPropertyBody(t, map[string]any{"title": "Ticket two", "external_ticket_no": "XQ-3482"})
	if first.ID == second.ID || first.Identifier == second.Identifier || first.ExternalTicketNo != "XQ-3482" || second.ExternalTicketNo != first.ExternalTicketNo {
		t.Fatal("external ticket replaced internal identity or rejected duplicates")
	}
	var duplicates struct {
		Count int64 `json:"count"`
	}
	testutil.Call(t, testHandler.CountExternalTicketDuplicates, newRequest("GET", "/api/issues/external-ticket-duplicates?value=XQ-3482&exclude_id="+first.ID, nil)).Want(http.StatusOK).JSON(&duplicates)
	if duplicates.Count < 1 {
		t.Fatal("duplicate hint did not find the other task")
	}
	update := func(value string, revision int64, status int) IssueResponse {
		t.Helper()
		request := testutil.WithURLParams(newRequest("PATCH", "/api/issues/"+first.ID, map[string]any{"external_ticket_no": value, "expected_revision": revision}), "id", first.ID)
		result := testutil.Call(t, testHandler.UpdateIssue, request).Want(status)
		var issue IssueResponse
		if status == http.StatusOK {
			result.JSON(&issue)
		}
		return issue
	}
	updated := update("OTHER-7", first.Revision, http.StatusOK)
	if updated.Identifier != first.Identifier || updated.ExternalTicketNo != "OTHER-7" {
		t.Fatal("external update changed system number")
	}
	update("STALE", first.Revision, http.StatusConflict)
	update("INVALID SPACE", updated.Revision, http.StatusBadRequest)
	cleared := update("", updated.Revision, http.StatusOK)
	if cleared.ExternalTicketNo != "" || cleared.Identifier != first.Identifier {
		t.Fatal("clear changed internal identity")
	}
	var audits int
	dbfx.QueryRow(t, `SELECT count(*) FROM activity_log WHERE issue_id=$1 AND action='external_ticket_updated'`, first.ID).Scan(&audits)
	if audits != 2 {
		t.Fatalf("expected two committed audit entries, got %d", audits)
	}
	var persisted string
	dbfx.QueryRow(t, `SELECT external_ticket_no FROM issue WHERE id=$1`, first.ID).Scan(&persisted)
	if persisted != "" {
		t.Fatal("clear was not persisted")
	}
	testutil.Call(t, testHandler.CreateIssue, newRequest("POST", "/api/issues", map[string]any{"title": "Invalid child reference", "parent_issue_id": first.ID, "external_ticket_no": "CHILD-1"})).Want(http.StatusBadRequest)
}

func TestExternalTicketAuditFailureRollsBack(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	id := dbfx.Issue(t, "Audit rollback", testutil.Cols{"external_ticket_no": "ORIGINAL"})
	_, _, _, err := testHandler.updateIssueAtomically(context.Background(), parseUUID(testWorkspaceID), db.UpdateIssueParams{ID: parseUUID(id), ExternalTicketNo: pgtype.Text{String: "NEW", Valid: true}}, nil, nil, nil, nil, "", false)
	if err == nil {
		t.Fatal("missing actor audit must fail")
	}
	var persisted string
	dbfx.QueryRow(t, `SELECT external_ticket_no FROM issue WHERE id=$1`, id).Scan(&persisted)
	if persisted != "ORIGINAL" {
		t.Fatal("failed audit committed field")
	}
}
