package handler

import (
	"encoding/csv"
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
