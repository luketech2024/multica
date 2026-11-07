package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWorkbenchCredentialInboxActionTracksExpiryAndPermission(t *testing.T) {
	workspace := dbfx.Workspace(t, "Credential inbox", "credential-inbox-"+uuid.NewString())
	dbfx.Member(t, workspace, testUserID, "owner")
	expires := time.Now().UTC().Truncate(time.Microsecond).Add(6 * 24 * time.Hour)
	credential := dbfx.Insert(t, "credentials", testutil.Cols{
		"workspace_id": workspace, "label": "Deployment key", "kind": "token", "usage": []string{"deploy"},
		"secret_enc": []byte("encrypted"), "wrapped_key": []byte("wrapped"), "last_four": "1234", "expires_at": expires,
	})
	details, _ := json.Marshal(map[string]string{"source": "workbench_credential", "credential_id": credential, "expires_at": expires.Format(time.RFC3339Nano)})
	notice := dbfx.Insert(t, "inbox_item", testutil.Cols{
		"workspace_id": workspace, "recipient_type": "member", "recipient_id": testUserID,
		"title": "Rotate key", "type": "review_requested", "severity": "action_required", "details": details, "read": true,
	})
	read := func(want string) {
		t.Helper()
		var items []InboxItemResponse
		testutil.Call(t, inboxWorkspaceHandler(testHandler.ListInbox), inboxRequest("GET", "/api/inbox", workspace)).Want(200).JSON(&items)
		if len(items) != 1 || items[0].ID != notice || items[0].WorkbenchAction != want || !items[0].Read {
			t.Fatalf("credential action=%+v, want %s with read preserved", items, want)
		}
	}
	read("credential")
	dbfx.Exec(t, "UPDATE credentials SET expires_at=expires_at+interval '1 day' WHERE id=$1", credential)
	read("")
	dbfx.Exec(t, "UPDATE credentials SET expires_at=$2 WHERE id=$1", credential, expires)
	dbfx.Exec(t, "UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2", workspace, testUserID)
	read("")
}

func TestWorkbenchInboxCreationMarksAllSourcesAndQuietsRoutineStatus(t *testing.T) {
	workspace := dbfx.Workspace(t, "Workbench notice policy", "workbench-policy-"+uuid.NewString())
	dbfx.Member(t, workspace, testUserID, "owner")
	parent := dbfx.Issue(t, "Workbench parent", testutil.Cols{"workspace_id": workspace, "acceptance_criteria": []byte(`{"v":1,"team":{}}`)})
	child := dbfx.Issue(t, "Workbench child", testutil.Cols{"workspace_id": workspace, "properties": []byte(`{"workbench_handoff":{"v":2}}`)})
	native := dbfx.Issue(t, "Native issue", testutil.Cols{"workspace_id": workspace})
	for _, tc := range []struct {
		name, issue, kind, severity string
		workbench, read             bool
	}{
		{"parent status", parent, "status_changed", "info", true, true},
		{"child mention", child, "mentioned", "info", true, false},
		{"parent assignment", parent, "issue_assigned", "action_required", true, false},
		{"review", parent, "review_requested", "action_required", true, false},
		{"urgent status", parent, "status_changed", "action_required", true, false},
		{"native", native, "status_changed", "info", false, false},
		{"announcement", "", "system", "info", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := db.CreateInboxItemParams{WorkspaceID: parseUUID(workspace), RecipientID: parseUUID(testUserID), RecipientType: "member",
				Type: tc.kind, Severity: tc.severity, Title: tc.name, Details: []byte(`{"comment_id":"kept"}`)}
			if tc.issue != "" {
				params.IssueID = parseUUID(tc.issue)
			}
			item, err := testHandler.Queries.CreateInboxItem(context.Background(), params)
			if err != nil {
				t.Fatal(err)
			}
			dbfx.Cleanup(t, "DELETE FROM inbox_item WHERE id=$1", item.ID)
			var details map[string]string
			if err := json.Unmarshal(item.Details, &details); err != nil {
				t.Fatal(err)
			}
			if (details["workbench"] == "true") != tc.workbench || item.Read != tc.read || details["comment_id"] != "kept" {
				t.Fatalf("notice read=%v details=%v, want workbench=%v read=%v", item.Read, details, tc.workbench, tc.read)
			}
		})
	}
}

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
