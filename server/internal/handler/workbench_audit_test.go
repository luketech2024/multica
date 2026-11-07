package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestWorkbenchAuditExportPaginatesAndScopesNinetyDays(t *testing.T) {
	workspace := dbfx.Workspace(t, "Audit export", "audit-"+uuid.NewString())
	dbfx.Member(t, workspace, testUserID, "owner")
	actor := dbfx.User(t, "\t=HYPERLINK(\"bad\")", "csv-"+uuid.NewString()+"@example.test")
	stamp := time.Now().UTC().Add(-time.Hour)
	dbfx.Cleanup(t, "DELETE FROM activity_log WHERE workspace_id=$1", workspace)
	dbfx.Exec(t, `INSERT INTO activity_log(workspace_id,actor_type,actor_id,action,details,created_at)
        SELECT $1,'member',$2,'credential_rotated',jsonb_build_object('label', '部署,密钥 "引用"', 'number', n),$3
        FROM generate_series(1,1001) n`, workspace, actor, stamp)
	dbfx.Insert(t, "activity_log", testutil.Cols{"workspace_id": workspace, "action": "credential_created", "created_at": stamp.Add(-91 * 24 * time.Hour)})
	dbfx.Insert(t, "activity_log", testutil.Cols{"workspace_id": workspace, "action": "credential_created", "created_at": stamp.Add(48 * time.Hour)})
	dbfx.Insert(t, "activity_log", testutil.Cols{"workspace_id": workspace, "action": "title_changed"})
	dbfx.Insert(t, "activity_log", testutil.Cols{"workspace_id": testWorkspaceID, "action": "credential_created", "details": []byte(`{"label":"Other workspace"}`)})
	result := testutil.Call(t, inboxWorkspaceHandler(testHandler.ExportWorkbenchAudit), inboxRequest(http.MethodGet, "/api/workbench/audit.csv", workspace)).Want(http.StatusOK)
	if result.Header().Get("Content-Type") != "text/csv; charset=utf-8" || result.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("export headers=%v", result.Header())
	}
	if result.Header().Get("Content-Length") != strconv.Itoa(result.Body.Len()) || !strings.Contains(result.Header().Get("Content-Disposition"), ".csv") {
		t.Fatal("export is not a complete downloadable CSV")
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(result.Body.String(), "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1003 {
		t.Fatalf("CSV rows=%d, want export metadata + header + 1001 scoped rows", len(rows))
	}
	if rows[0][0] != "exported_at" || rows[0][2] != "range_start" || rows[0][7] != "configuration" {
		t.Fatal("export range and timestamp are missing")
	}
	seen := map[string]bool{}
	for _, row := range rows[2:] {
		if seen[row[0]] {
			t.Fatalf("duplicate audit %s", row[0])
		}
		seen[row[0]] = true
		if row[4] != "'\t=HYPERLINK(\"bad\")" || row[6] != "credential_rotated" || !strings.Contains(row[7], "部署,密钥") {
			t.Fatalf("CSV values escaped incorrectly: %q", row)
		}
	}
	dbfx.Exec(t, "UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2", workspace, testUserID)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ExportWorkbenchAudit), inboxRequest(http.MethodGet, "/api/workbench/audit.csv", workspace)).Want(http.StatusForbidden)
}

func TestWorkbenchAuditDownloadOwnershipExpiryAndSnapshot(t *testing.T) {
	workspace := dbfx.Workspace(t, "Audit downloads", "audit-download-"+uuid.NewString())
	dbfx.Member(t, workspace, testUserID, "owner")
	dbfx.Cleanup(t, "DELETE FROM workbench_audit_export WHERE workspace_id=$1", workspace)
	dbfx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1", workspace)
	dbfx.Insert(t, "activity_log", testutil.Cols{"workspace_id": workspace, "action": "security_accept", "actor_type": "member", "actor_id": testUserID, "details": []byte(`{"reason":"Reviewed low risk"}`)})
	var result struct {
		ID        string    `json:"id"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	testutil.Call(t, inboxWorkspaceHandler(testHandler.CreateWorkbenchAuditExport), inboxRequest("POST", "/api/workbench/audit-exports?scope=security&days=7", workspace)).Want(201).JSON(&result)
	if result.ExpiresAt.Before(time.Now().Add(23*time.Hour)) || result.ExpiresAt.After(time.Now().Add(25*time.Hour)) {
		t.Fatal("export expiry is not 24 hours")
	}
	var noticeCount int
	dbfx.QueryRow(t, "SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND details->>'export_id'=$2", workspace, result.ID).Scan(&noticeCount)
	if noticeCount != 1 {
		t.Fatalf("export notices=%d", noticeCount)
	}
	request := testutil.WithURLParams(inboxRequest("GET", "/api/workbench/audit-exports/"+result.ID, workspace), "exportId", result.ID)
	first := testutil.Call(t, inboxWorkspaceHandler(testHandler.DownloadWorkbenchAuditExport), request).Want(200)
	if !strings.Contains(first.Body.String(), "security_accept") || !strings.Contains(first.Body.String(), "Reviewed low risk") {
		t.Fatal("download missed snapshot content")
	}
	dbfx.Insert(t, "activity_log", testutil.Cols{"workspace_id": workspace, "action": "security_recheck", "details": []byte(`{"later":true}`)})
	request = testutil.WithURLParams(inboxRequest("GET", "/api/workbench/audit-exports/"+result.ID, workspace), "exportId", result.ID)
	second := testutil.Call(t, inboxWorkspaceHandler(testHandler.DownloadWorkbenchAuditExport), request).Want(200)
	if first.Body.String() != second.Body.String() {
		t.Fatal("download changed after creation")
	}
	other := dbfx.User(t, "Other manager", "audit-manager-"+uuid.NewString()+"@example.test")
	dbfx.Member(t, workspace, other, "admin")
	request = testutil.WithURLParams(inboxRequest("GET", "/api/workbench/audit-exports/"+result.ID, workspace), "exportId", result.ID)
	request.Header.Set("X-User-ID", other)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.DownloadWorkbenchAuditExport), request).Want(404)
	dbfx.Exec(t, "UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2", workspace, testUserID)
	request = testutil.WithURLParams(inboxRequest("GET", "/api/workbench/audit-exports/"+result.ID, workspace), "exportId", result.ID)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.DownloadWorkbenchAuditExport), request).Want(403)
	dbfx.Exec(t, "UPDATE member SET role='owner' WHERE workspace_id=$1 AND user_id=$2", workspace, testUserID)
	dbfx.Exec(t, "UPDATE workbench_audit_export SET expires_at=now()-interval '1 second' WHERE id=$1", result.ID)
	request = testutil.WithURLParams(inboxRequest("GET", "/api/workbench/audit-exports/"+result.ID, workspace), "exportId", result.ID)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.DownloadWorkbenchAuditExport), request).Want(410)
	if _, err := testHandler.Queries.PruneWorkbenchAuditExports(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int
	dbfx.QueryRow(t, "SELECT count(*) FROM workbench_audit_export WHERE id=$1", result.ID).Scan(&count)
	if count != 0 {
		t.Fatal("expired export remained stored")
	}
}

func TestWorkbenchAuditListPaginatesSecurityAndRejectsInvalidFilters(t *testing.T) {
	workspace := dbfx.Workspace(t, "Audit page", "audit-page-"+uuid.NewString())
	dbfx.Member(t, workspace, testUserID, "owner")
	dbfx.Cleanup(t, "DELETE FROM activity_log WHERE workspace_id=$1", workspace)
	dbfx.Exec(t, `INSERT INTO activity_log(workspace_id,action,details,created_at)
        SELECT $1,'security_recheck','{}',now()-interval '1 hour' FROM generate_series(1,1001)`, workspace)
	dbfx.Insert(t, "activity_log", testutil.Cols{"workspace_id": workspace, "action": "credential_rotated"})
	var page struct {
		Records []json.RawMessage `json:"records"`
		Next    string            `json:"next_cursor"`
	}
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListWorkbenchAudit), inboxRequest("GET", "/api/workbench/audit?scope=security", workspace)).Want(200).JSON(&page)
	if len(page.Records) != 1000 || page.Next == "" {
		t.Fatalf("first page=%d cursor=%s", len(page.Records), page.Next)
	}
	var record struct {
		Details map[string]any `json:"details"`
	}
	if err := json.Unmarshal(page.Records[0], &record); err != nil || record.Details == nil {
		t.Fatalf("audit details must be a JSON object: %s (%v)", page.Records[0], err)
	}
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListWorkbenchAudit), inboxRequest("GET", "/api/workbench/audit?scope=security&cursor="+page.Next, workspace)).Want(200).JSON(&page)
	if len(page.Records) != 1 || page.Next != "" {
		t.Fatalf("last page=%d cursor=%s", len(page.Records), page.Next)
	}
	for _, query := range []string{"scope=invalid", "days=365", "cursor=bad"} {
		testutil.Call(t, inboxWorkspaceHandler(testHandler.ListWorkbenchAudit), inboxRequest("GET", "/api/workbench/audit?"+query, workspace)).Want(400)
	}
}

func TestAuditCSVCellTreatsFormulaPrefixesAsText(t *testing.T) {
	for _, value := range []string{"=1+1", "+cmd", "-cmd", "@SUM(A1)", " \n=1", "\t+1"} {
		if auditCSVCell(value) != "'"+value {
			t.Fatalf("formula prefix not escaped: %q", value)
		}
	}
	if auditCSVCell("普通文本,\"引用\"") != "普通文本,\"引用\"" {
		t.Fatal("ordinary text changed")
	}
}
