package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestWorkbenchMemberGrantAndRevokeAudit(t *testing.T) {
	workspace := dbfx.Workspace(t, "Membership audit", "membership-audit-"+uuid.NewString())
	dbfx.Member(t, workspace, testUserID, "owner")
	email := "grant-audit-" + uuid.NewString() + "@example.test"
	user := dbfx.User(t, "Invited member", email)
	invitation := dbfx.Insert(t, "workspace_invitation", testutil.Cols{
		"workspace_id": workspace, "inviter_id": testUserID, "invitee_email": email,
		"invitee_user_id": user, "role": "admin", "status": "pending", "expires_at": testutil.Raw("now()+interval '1 day'"),
	})
	dbfx.Cleanup(t, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", workspace, user)
	dbfx.Cleanup(t, "DELETE FROM activity_log WHERE workspace_id=$1", workspace)
	for range 2 {
		req := withURLParam(newRequest("POST", "/api/invitations/"+invitation+"/accept", nil), "id", invitation)
		req.Header.Set("X-User-ID", user)
		testutil.Call(t, testHandler.AcceptInvitation, req).Want(200)
	}
	var member string
	dbfx.QueryRow(t, "SELECT id FROM member WHERE workspace_id=$1 AND user_id=$2", workspace, user).Scan(&member)
	if _, err := testHandler.revokeAndRemoveMember(context.Background(), parseUUID(workspace), parseUUID(user), parseUUID(member), parseUUID(testUserID)); err != nil {
		t.Fatal(err)
	}
	var granted, revoked int
	dbfx.QueryRow(t, `SELECT count(*) FILTER (WHERE details->>'previous_role'='' AND details->>'role'='admin'),
		count(*) FILTER (WHERE details->>'previous_role'='admin' AND details->>'role'='')
		FROM activity_log WHERE workspace_id=$1 AND action='workspace_member_role_changed'`, workspace).Scan(&granted, &revoked)
	if granted != 1 || revoked != 1 {
		t.Fatalf("membership audit grant=%d revoke=%d, want 1 each", granted, revoked)
	}
}

func TestWorkbenchMemberRoleAuditIsAtomicAndSkipsNoOp(t *testing.T) {
	workspace := dbfx.Workspace(t, "Member audit", "member-audit-"+uuid.NewString())
	dbfx.Member(t, workspace, testUserID, "owner")
	user := dbfx.User(t, "Audit target", "member-audit-"+uuid.NewString()+"@example.test")
	member := dbfx.Member(t, workspace, user, "member")
	dbfx.Cleanup(t, "DELETE FROM activity_log WHERE workspace_id=$1", workspace)
	update := func(h *Handler, role string, status int) {
		req := withURLParams(newRequest("PATCH", "/api/workspaces/"+workspace+"/members/"+member, map[string]any{"role": role}), "id", workspace, "memberId", member)
		testutil.Call(t, h.UpdateMember, req).Want(status)
	}
	update(testHandler, "admin", 200)
	update(testHandler, "admin", 200)
	var count int
	dbfx.QueryRow(t, "SELECT count(*) FROM activity_log WHERE workspace_id=$1 AND action='workspace_member_role_changed'", workspace).Scan(&count)
	if count != 1 {
		t.Fatalf("role change audits=%d, want one real change", count)
	}
	var details []byte
	dbfx.QueryRow(t, "SELECT details FROM activity_log WHERE workspace_id=$1 AND action='workspace_member_role_changed'", workspace).Scan(&details)
	var change map[string]string
	if err := json.Unmarshal(details, &change); err != nil || change["previous_role"] != "member" || change["role"] != "admin" || change["user_id"] != user {
		t.Fatalf("incorrect role audit: %s (%v)", details, err)
	}
	failing := *testHandler
	failing.TxStarter = rollbackOnCommitTxStarter{pool: testPool}
	update(&failing, "member", 500)
	var role string
	dbfx.QueryRow(t, "SELECT role FROM member WHERE id=$1", member).Scan(&role)
	dbfx.QueryRow(t, "SELECT count(*) FROM activity_log WHERE workspace_id=$1 AND action='workspace_member_role_changed'", workspace).Scan(&count)
	if role != "admin" || count != 1 {
		t.Fatalf("failed transaction changed role or audit: role=%s audits=%d", role, count)
	}
	dbfx.Exec(t, "UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2", workspace, testUserID)
	update(testHandler, "member", 403)
}
