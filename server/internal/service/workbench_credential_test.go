package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCredentialExpiryRemindersDeduplicateAndFollowExpiry(t *testing.T) {
	f, _ := newPrincipalFixture(t)
	credential := f.Insert(t, "credentials", testutil.Cols{
		"workspace_id": f.WorkspaceID, "label": "Deployment key", "kind": "token", "usage": []string{"deploy"},
		"secret_enc": []byte("encrypted"), "wrapped_key": []byte("wrapped"), "last_four": "1234", "expires_at": time.Now().Add(6 * 24 * time.Hour),
	})
	f.Cleanup(t, "DELETE FROM inbox_item WHERE details->>'credential_id'=$1", credential)
	run := func() {
		t.Helper()
		var workers sync.WaitGroup
		results := make(chan error, 2)
		for range 2 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				results <- f.svc.TaskSvc.RemindExpiringCredentials(context.Background())
			}()
		}
		workers.Wait()
		close(results)
		for err := range results {
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	run()
	var count int
	f.QueryRow(t, "SELECT count(*) FROM inbox_item WHERE details->>'credential_id'=$1", credential).Scan(&count)
	if count != 1 {
		t.Fatalf("concurrent reminders=%d, want one", count)
	}
	f.Exec(t, "UPDATE inbox_item SET read=true WHERE details->>'credential_id'=$1", credential)
	run()
	f.QueryRow(t, "SELECT count(*) FROM inbox_item WHERE details->>'credential_id'=$1 AND read=false", credential).Scan(&count)
	if count != 0 {
		t.Fatal("repeated checks reset read state")
	}
	f.Exec(t, "UPDATE credentials SET expires_at=now()+interval '30 days' WHERE id=$1", credential)
	run()
	f.QueryRow(t, "SELECT count(*) FROM inbox_item WHERE details->>'credential_id'=$1", credential).Scan(&count)
	if count != 1 {
		t.Fatal("credential outside seven-day window produced reminder")
	}
	f.Exec(t, "UPDATE credentials SET expires_at=now()+interval '2 days' WHERE id=$1", credential)
	run()
	f.QueryRow(t, "SELECT count(*) FROM inbox_item WHERE details->>'credential_id'=$1", credential).Scan(&count)
	if count != 2 {
		t.Fatal("new expiry did not produce a fresh reminder")
	}
}
