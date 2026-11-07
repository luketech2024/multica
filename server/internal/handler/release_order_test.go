package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func TestReleaseWorkspaceDeletion(t *testing.T) {
	token := uuidToString(dbid.NewV7())
	workspace := dbfx.Workspace(t, "Release cleanup", "release-cleanup-"+token)
	dbfx.Member(t, workspace, testUserID, "owner")
	order := dbfx.Insert(t, "workbench_release_orders", testutil.Cols{
		"id":           uuidToString(dbid.NewV7()),
		"workspace_id": workspace, "project_id": token, "number": 1, "env": "staging", "request_id": token, "request_hash": "digest", "manifest": "{}", "created_by": testUserID,
	})
	dbfx.Exec(t, `INSERT INTO workbench_release_counters (workspace_id,next_number) VALUES ($1,2)`, workspace)
	dbfx.Exec(t, `INSERT INTO workbench_release_reservations (workspace_id,release_id,repo_id,merge_commit,env) VALUES ($1,$2,$3,'commit','staging')`, workspace, order, token)
	testutil.Call(t, testHandler.DeleteWorkspace, testutil.WithURLParams(newRequest("DELETE", "/api/workspaces/"+workspace, nil), "id", workspace)).Want(http.StatusNoContent)
	for _, table := range []string{"workbench_release_orders", "workbench_release_counters", "workbench_release_reservations"} {
		var count int
		dbfx.QueryRow(t, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1", workspace).Scan(&count)
		if count != 0 {
			t.Fatalf("workspace deletion retained %s", table)
		}
	}
}
