package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWorkbenchInboxPrunePreservesWindowAndNativeNotices(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	fx := testutil.New(pool, "", "")
	fx.WorkspaceID = fx.Workspace(t, "Inbox retention", "retention-"+uuid.NewString())
	fx.UserID = fx.User(t, "Inbox retention", "retention-"+uuid.NewString()+"@example.test")
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	issue := fx.Issue(t, "Existing workbench notice", testutil.Cols{"acceptance_criteria": []byte(`{"v":1,"team":{}}`)})
	plan := time.Now().UTC().Truncate(time.Hour)
	cutoff := plan.Add(-WorkbenchInboxRetention)
	for _, tc := range []struct {
		name, details              string
		at                         time.Time
		legacy, archived, retained bool
	}{
		{"expired", `{"workbench":"true"}`, cutoff.Add(-time.Second), false, false, false},
		{"expired archive", `{"workbench":"true"}`, cutoff.Add(-time.Hour), false, true, false},
		{"delivery source", `{"source":"workbench_delivery"}`, cutoff.Add(-time.Hour), false, false, false},
		{"legacy", `{}`, cutoff.Add(-time.Hour), true, false, false},
		{"boundary", `{"workbench":"true"}`, cutoff, false, false, true},
		{"fresh", `{"workbench":"true"}`, plan, false, false, true},
		{"native", `{}`, cutoff.Add(-time.Hour), false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cols := testutil.Cols{"workspace_id": fx.WorkspaceID, "recipient_type": "member", "recipient_id": fx.UserID,
				"type": "mentioned", "severity": "info", "title": tc.name, "created_at": tc.at, "archived": tc.archived, "details": []byte(tc.details)}
			if tc.legacy {
				cols["issue_id"] = issue
			}
			id := fx.Insert(t, "inbox_item", cols)
			job := WorkbenchInboxPruneJob(db.New(pool))
			if _, err := job.Handler(ctx, HandlerInput{PlanTime: plan}); err != nil {
				t.Fatal(err)
			}
			var exists bool
			if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM inbox_item WHERE id=$1)", id).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if exists != tc.retained {
				t.Fatalf("retained=%v, want %v", exists, tc.retained)
			}
			result, err := job.Handler(ctx, HandlerInput{PlanTime: plan})
			if err != nil || result.RowsAffected != 0 {
				t.Fatalf("repeat=%+v, err=%v", result, err)
			}
		})
	}
}
