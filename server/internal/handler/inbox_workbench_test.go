package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestWorkbenchInboxActionTracksTaskIndependentlyOfReadState(t *testing.T) {
	workspace := dbfx.Workspace(t, "Workbench inbox", "workbench-inbox-"+uuid.NewString())
	dbfx.Member(t, workspace, testUserID, "owner")
	criteria := workbench.Requirements{V: 1, Version: 1, NextItemNo: 2,
		Items: []workbench.Item{{ID: "A-01", Text: "Export", Covers: []string{}}}, Bounds: []string{},
		Team: &workbench.ProjectTeamSnapshot{ProjectRoleConfiguration: workbench.ProjectRoleConfiguration{Config: workbench.DefaultProjectRoles()}},
	}
	raw, _ := json.Marshal(criteria)
	issue := dbfx.Issue(t, "Pending requirement", testutil.Cols{
		"workspace_id": workspace, "assignee_type": "member", "assignee_id": testUserID, "acceptance_criteria": raw,
	})
	notice := dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": workspace, "recipient_type": "member", "recipient_id": testUserID, "type": "issue_assigned",
		"severity": "action_required", "issue_id": issue, "title": "Review requirement", "read": true,
	})
	read := func(want string) {
		t.Helper()
		var items []InboxItemResponse
		testutil.Call(t, inboxWorkspaceHandler(testHandler.ListInbox), inboxRequest(http.MethodGet, "/api/inbox", workspace)).Want(http.StatusOK).JSON(&items)
		if len(items) != 1 || items[0].ID != notice || !items[0].Workbench || items[0].WorkbenchAction != want || !items[0].Read {
			t.Fatalf("action projection = %+v, want %q with read retained", items, want)
		}
	}
	read("requirements")
	for _, state := range []struct{ status, action string }{{"in_review", "delivery"}, {"blocked", "blocked"}, {"cancelled", ""}} {
		dbfx.Exec(t, "UPDATE issue SET status=$1 WHERE id=$2", state.status, issue)
		read(state.action)
	}
	dbfx.Exec(t, "UPDATE issue SET status='in_review',assignee_type=NULL,assignee_id=NULL WHERE id=$1", issue)
	read("")
}
