package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func securityFixture(t *testing.T) (string, string, string, workbench.SecurityScanInput) {
	t.Helper()
	project, repo := deployConfigFixture(t)
	parent := dbfx.Issue(t, "Security parent", testutil.Cols{"project_id": project, "status": "todo"})
	handoff, _ := json.Marshal(map[string]any{"workbench_handoff": workbench.Handoff{V: 2, RequirementVersion: 1, Role: "sec", RepoID: repo,
		Covers: []string{"A-01"}, DependsOn: []string{}, Expected: []string{"security report"}}})
	child := dbfx.Issue(t, "Security scan", testutil.Cols{"project_id": project, "parent_issue_id": parent, "status": "done", "properties": handoff})
	roles := workbench.DefaultProjectRoles()
	roles.Roles["sec"] = workbench.ProjectRole{Enabled: true}
	criteria := workbench.Requirements{V: 1, Version: 1, NextItemNo: 2,
		Items: []workbench.Item{{ID: "A-01", Text: "Check export safety", Covers: []string{child}, Checked: &workbench.Check{At: "confirmed", By: testUserID}}}, Bounds: []string{},
		Gate: workbench.Gates{Spec: &workbench.GateCheck{PassedAt: "confirmed", PassedBy: testUserID}},
		Team: &workbench.ProjectTeamSnapshot{ProjectRoleConfiguration: workbench.ProjectRoleConfiguration{Config: roles}},
		Plan: &workbench.ExecutionPlan{Version: 1, Issues: []string{child}},
	}
	raw, _ := json.Marshal(criteria)
	dbfx.Exec(t, "UPDATE issue SET acceptance_criteria=$1, status='in_progress', assignee_type='member', assignee_id=$3 WHERE id=$2", raw, parent, testUserID)
	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(child))
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM security_checks WHERE project_id=$1", project)
	dbfx.Cleanup(t, "DELETE FROM security_scans WHERE project_id=$1", project)
	dbfx.Cleanup(t, "DELETE FROM activity_log WHERE issue_id=$1 OR details->>'project_id'=$2", child, project)
	return project, repo, parent, workbench.SecurityScanInput{IssueID: child, IssueRevision: issue.Revision, RepoID: repo,
		RunID: "scan-1", ArtifactVersion: "v1", Commit: strings.Repeat("a", 40), ScannedAt: time.Now().Add(-time.Second),
		Status: "completed", ReportURL: "https://example.test/security/report", Categories: append([]string{}, workbench.SecurityCategories...),
		Findings: []workbench.SecurityFinding{
			{Fingerprint: "sql-input", Severity: "high", Category: "injection", Title: "Unsafe SQL input", CWE: "CWE-89", Location: "query.go:10", Recommendation: "Use bound parameters"},
			{Fingerprint: "verbose-error", Severity: "low", Category: "authz", Title: "Verbose error", Location: "handler.go:20", Recommendation: "Remove internal details"},
		},
	}
}

func TestSecurityScanConcurrentIdentityAndDecisionPermissions(t *testing.T) {
	project, _, _, input := securityFixture(t)
	var workers sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			response := httptest.NewRecorder()
			testHandler.RecordSecurityScan(response, newRequest("POST", "/api/security/scans", input))
			responses <- response
		}()
	}
	workers.Wait()
	close(responses)
	var scanID string
	for response := range responses {
		if response.Code != 200 {
			t.Fatalf("concurrent scan=%d %s", response.Code, response.Body.String())
		}
		var scan struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &scan); err != nil {
			t.Fatal(err)
		}
		if scanID != "" && scanID != scan.ID {
			t.Fatal("concurrent scan duplicated evidence")
		}
		scanID = scan.ID
	}
	checks, err := testHandler.Queries.ListSecurityChecks(context.Background(), db.ListSecurityChecksParams{
		WorkspaceID: parseUUID(testWorkspaceID), ProjectID: parseUUID(project), BeforeID: parseUUID("ffffffff-ffff-ffff-ffff-ffffffffffff"),
	})
	if err != nil || len(checks) != 2 {
		t.Fatalf("checks=%d err=%v", len(checks), err)
	}
	var low db.SecurityCheck
	for _, check := range checks {
		if check.Severity == "low" {
			low = check
		}
	}
	user := dbfx.User(t, "Security member", "security-member-"+project+"@example.test")
	dbfx.Member(t, testWorkspaceID, user, "member")
	body := map[string]any{"action": "accept", "expected_revision": low.Revision, "reason": "Reviewed", "days": 7}
	request := testutil.WithURLParams(newRequestAs(user, "PATCH", "/api/security/checks/"+uuidToString(low.ID), body), "checkId", uuidToString(low.ID))
	testutil.Call(t, testHandler.UpdateSecurityCheck, request).Want(403)
	outsider := dbfx.User(t, "Security outsider", "security-outsider-"+project+"@example.test")
	request = testutil.WithURLParams(newRequestAs(outsider, "GET", "/api/security/checks/"+uuidToString(low.ID), nil), "checkId", uuidToString(low.ID))
	testutil.Call(t, testHandler.GetSecurityCheck, request).Want(404)
	request = testutil.WithURLParams(newRequestAs(outsider, "GET", "/api/security/scans/"+scanID, nil), "scanId", scanID)
	testutil.Call(t, testHandler.GetSecurityScan, request).Want(404)
	request = testutil.WithURLParams(newRequest("PATCH", "/api/security/checks/"+uuidToString(low.ID), body), "checkId", uuidToString(low.ID))
	testutil.Call(t, testHandler.UpdateSecurityCheck, request).Want(200)
	request = testutil.WithURLParams(newRequest("PATCH", "/api/security/checks/"+uuidToString(low.ID), body), "checkId", uuidToString(low.ID))
	testutil.Call(t, testHandler.UpdateSecurityCheck, request).Want(409)
	input.RunID = "new-artifact"
	input.ArtifactVersion = "v2"
	input.ScannedAt = time.Now()
	input.Findings[1].Severity = "high"
	testutil.Call(t, testHandler.RecordSecurityScan, newRequest("POST", "/api/security/scans", input)).Want(200)
	reopened, err := testHandler.Queries.GetSecurityCheck(context.Background(), db.GetSecurityCheckParams{WorkspaceID: parseUUID(testWorkspaceID), ID: low.ID})
	if err != nil || reopened.Status != "unresolved" || reopened.AcceptedUntil.Valid || reopened.AcceptanceReason != "" {
		t.Fatalf("new high risk retained its acceptance: %+v, err=%v", reopened, err)
	}
}

func TestSecurityScanIdentityDecisionAndVerifiedRecheck(t *testing.T) {
	project, repo, _, input := securityFixture(t)
	record := func(report workbench.SecurityScanInput, status int) string {
		t.Helper()
		result := testutil.Call(t, testHandler.RecordSecurityScan, newRequest("POST", "/api/security/scans", report)).Want(status)
		var scan struct {
			ID string `json:"id"`
		}
		if status == 200 {
			result.JSON(&scan)
		}
		return scan.ID
	}
	scanID := record(input, 200)
	if again := record(input, 200); scanID != again {
		t.Fatal("repeated scan created another record")
	}
	conflicting := input
	conflicting.ArtifactVersion = "another-version"
	record(conflicting, 409)
	checks, err := testHandler.Queries.ListSecurityChecks(context.Background(), db.ListSecurityChecksParams{
		WorkspaceID: parseUUID(testWorkspaceID), ProjectID: parseUUID(project), BeforeID: parseUUID("ffffffff-ffff-ffff-ffff-ffffffffffff"),
	})
	if err != nil || len(checks) != 2 {
		t.Fatalf("checks=%d, err=%v", len(checks), err)
	}
	var high, low db.SecurityCheck
	for _, check := range checks {
		if check.Severity == "high" {
			high = check
		} else {
			low = check
		}
	}
	decide := func(check db.SecurityCheck, input map[string]any, status int) db.SecurityCheck {
		t.Helper()
		input["expected_revision"] = check.Revision
		request := testutil.WithURLParams(newRequest("PATCH", "/api/security/checks/"+uuidToString(check.ID), input), "checkId", uuidToString(check.ID))
		result := testutil.Call(t, testHandler.UpdateSecurityCheck, request).Want(status)
		var after SecurityCheckResponse
		if status == 200 {
			result.JSON(&after)
		}
		return after.SecurityCheck
	}
	decide(high, map[string]any{"action": "accept", "reason": "Reviewed", "days": 7}, 400)
	decide(low, map[string]any{"action": "accept", "reason": "", "days": 7}, 400)
	low = decide(low, map[string]any{"action": "accept", "reason": "Accepted with request rate limits", "days": 7}, 200)
	if low.Status != "accepted" || !low.AcceptedUntil.Valid {
		t.Fatal("low risk acceptance was not persisted")
	}
	high = decide(high, map[string]any{"action": "request_recheck", "artifact_version": "v2"}, 200)
	decide(high, map[string]any{"action": "recheck", "scan_id": scanID}, 409)
	clean := input
	clean.RunID, clean.ArtifactVersion, clean.Status, clean.Findings, clean.ScannedAt = "scan-2", "v2", "failed", []workbench.SecurityFinding{}, time.Now()
	failed := record(clean, 200)
	decide(high, map[string]any{"action": "recheck", "scan_id": failed}, 409)
	clean.RunID, clean.Status, clean.ScannedAt = "scan-3", "completed", time.Now()
	passed := record(clean, 200)
	high = decide(high, map[string]any{"action": "recheck", "scan_id": passed}, 200)
	if high.Status != "fixed" || uuidToString(high.FixedScanID) != passed {
		t.Fatal("recheck evidence was not bound to the fix")
	}
	dbfx.Exec(t, "UPDATE security_checks SET accepted_until=now()-interval '1 minute' WHERE id=$1", low.ID)
	var list struct {
		Checks []SecurityCheckResponse `json:"checks"`
	}
	testutil.Call(t, testHandler.ListSecurityChecks, newRequest("GET", "/api/security/checks?project_id="+project+"&status=unresolved", nil)).Want(200).JSON(&list)
	if len(list.Checks) != 1 || list.Checks[0].ID != low.ID || list.Checks[0].EffectiveStatus != "unresolved" {
		t.Fatal("expired acceptance remained accepted")
	}
	remove := testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+project+"/resources/"+repo, nil), "id", project, "resourceId", repo)
	testutil.Call(t, testHandler.DeleteProjectResource, remove).Want(409)
	change := testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project+"/resources/"+repo, map[string]any{"resource_ref": map[string]string{"url": "https://example.test/different.git"}}), "id", project, "resourceId", repo)
	testutil.Call(t, testHandler.UpdateProjectResource, change).Want(409)
	removeProject := testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+project, nil), "id", project)
	testutil.Call(t, testHandler.DeleteProject, removeProject).Want(204)
	remaining, err := testHandler.Queries.CountSecurityScansForRepository(context.Background(), db.CountSecurityScansForRepositoryParams{WorkspaceID: parseUUID(testWorkspaceID), RepoID: parseUUID(repo)})
	if err != nil || remaining != 0 {
		t.Fatal("project deletion retained unreachable scan records")
	}
}
