package handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestDeliveryAcceptanceCannotBeReusedAfterReopening(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	parent := dbfx.Issue(t, "Reopen accepted delivery", testutil.Cols{"assignee_type": "member", "assignee_id": testUserID})
	child := dbfx.Issue(t, "Delivery proof", testutil.Cols{"parent_issue_id": parent})
	criteria := workbench.Requirements{V: 1, Version: 1,
		Items: []workbench.Item{{ID: "A-01", Text: "Export", Covers: []string{child}, Checked: &workbench.Check{At: "confirmed", By: testUserID}}},
		Gate:  workbench.Gates{Spec: &workbench.GateCheck{PassedAt: "confirmed", PassedBy: testUserID}},
	}
	raw, _ := json.Marshal(criteria)
	dbfx.Exec(t, `UPDATE issue SET acceptance_criteria=$1,status='in_review' WHERE id=$2`, raw, parent)
	criteria.Gate.Accept = &workbench.GateCheck{PassedAt: "accepted", PassedBy: testUserID}
	raw, _ = json.Marshal(criteria)
	dbfx.Exec(t, `UPDATE issue SET acceptance_criteria=$1,status='done' WHERE id=$2`, raw, parent)
	dbfx.Exec(t, `UPDATE issue SET status='in_progress' WHERE id=$1`, parent)
	current, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(parent))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := workbench.ParseRequirements(current.AcceptanceCriteria)
	if parsed.Gate.Accept != nil {
		t.Fatal("reopening retained the accepted conclusion")
	}
	dbfx.Exec(t, `UPDATE issue SET status='in_review' WHERE id=$1`, parent)
	_, err = testPool.Exec(context.Background(), `UPDATE issue SET status='done' WHERE id=$1`, parent)
	var constraint *pgconn.PgError
	if !errors.As(err, &constraint) || constraint.ConstraintName != "workbench_execution_gate" {
		t.Fatalf("completion reused an old acceptance: %v", err)
	}
}
