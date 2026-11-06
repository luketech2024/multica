package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestSecurityAcceptanceReminderAndExpiry(t *testing.T) {
	f, _ := newPrincipalFixture(t)
	project := f.Insert(t, "project", testutil.Cols{"workspace_id": f.WorkspaceID, "title": "Security expiry"})
	repo := f.Insert(t, "project_resource", testutil.Cols{"workspace_id": f.WorkspaceID, "project_id": project, "resource_type": "github_repo", "resource_ref": `{"url":"https://example.test/security.git"}`})
	issue := f.Issue(t, "Security scan", testutil.Cols{"project_id": project})
	check := f.Insert(t, "security_checks", testutil.Cols{"workspace_id": f.WorkspaceID, "project_id": project, "repo_id": repo,
		"issue_id": issue, "fingerprint": "expiry-risk", "severity": "low", "category": "authz", "title": "Review diagnostic details",
		"location": "api.go:1", "recommendation": "Reduce details", "artifact_version": "v1", "first_scan_id": issue, "last_scan_id": issue,
		"last_seen_at": time.Now(), "status": "accepted", "acceptance_reason": "Controlled access", "accepted_until": time.Now().Add(48 * time.Hour)})
	f.Cleanup(t, "DELETE FROM inbox_item WHERE details->>'check_id'=$1", check)
	f.Cleanup(t, "DELETE FROM activity_log WHERE details->>'check_id'=$1", check)
	runTwice := func() {
		t.Helper()
		var workers sync.WaitGroup
		results := make(chan error, 2)
		for range 2 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				results <- f.svc.TaskSvc.ReconcileSecurityAcceptances(context.Background())
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
	runTwice()
	var count int
	f.QueryRow(t, "SELECT count(*) FROM inbox_item WHERE details->>'check_id'=$1 AND details->>'source'='workbench_security'", check).Scan(&count)
	if count != 1 {
		t.Fatalf("reminders=%d, want 1", count)
	}
	f.Exec(t, "UPDATE security_checks SET accepted_until=now()-interval '1 minute' WHERE id=$1", check)
	f.Exec(t, "DELETE FROM issue WHERE id=$1", issue)
	runTwice()
	var status string
	f.QueryRow(t, "SELECT status FROM security_checks WHERE id=$1", check).Scan(&status)
	if status != "unresolved" {
		t.Fatalf("expired status=%s", status)
	}
	f.QueryRow(t, "SELECT count(*) FROM activity_log WHERE details->>'check_id'=$1 AND action='security_acceptance_expired'", check).Scan(&count)
	if count != 1 {
		t.Fatalf("expiry audit=%d, want 1", count)
	}
}
