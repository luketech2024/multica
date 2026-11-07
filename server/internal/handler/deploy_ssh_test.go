package handler

import (
	"bytes"
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestSSHCheckAuditsCredentialUseAndRejectsExpiredCredential(t *testing.T) {
	project, repo := deployConfigFixture(t)
	box, _ := secretbox.New(bytes.Repeat([]byte{12}, 32))
	h := *testHandler
	h.WorkbenchSecretBox = box
	var credential struct {
		ID string `json:"id"`
	}
	secret := "invalid-test-key-no-network"
	testutil.Call(t, h.SaveWorkbenchCredential, newRequest("POST", "/api/workbench/credentials", map[string]any{
		"label": "SSH check", "kind": "ssh-key", "usage": []string{"deploy"}, "secret": secret, "project_id": project,
	})).Want(200).JSON(&credential)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "DELETE FROM deploy_targets WHERE credential_id=$1", credential.ID)
		_, _ = testPool.Exec(context.Background(), "DELETE FROM credentials WHERE id=$1", credential.ID)
	})
	var target DeployTargetResponse
	testutil.Call(t, h.SaveDeployTarget, withURLParam(newRequest("POST", "/api/projects/targets", map[string]any{
		"label": "SSH staging", "repo_id": repo, "env": "staging", "kind": "service-process", "channel": "ssh", "credential_id": credential.ID,
		"location": map[string]string{"host": "127.0.0.1", "port": "22", "username": "deploy", "host_key": "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "dir": "/srv/app", "start_command": "server", "health_url": "https://example.test/health"},
	}), "id", project)).Want(200).JSON(&target)
	check := func(status int) {
		result := testutil.Call(t, h.CheckDeployTarget, withURLParams(newRequest("POST", "/api/projects/check", map[string]any{"expected_revision": target.Revision}), "id", project, "targetId", target.ID)).Want(status)
		if status == 200 {
			result.JSON(&target)
		}
	}
	check(200)
	result := workbench.ParseDeployCheck(target.LastCheck)
	if result.OK || result.Status != "failed" {
		t.Fatal("invalid private key passed")
	}
	var count int
	var details []byte
	dbfx.QueryRow(t, "SELECT count(*) FROM activity_log WHERE action='credential_used' AND details->>'credential_id'=$1", credential.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("credential use audit count: %d", count)
	}
	dbfx.QueryRow(t, "SELECT details FROM activity_log WHERE action='credential_used' AND details->>'credential_id'=$1", credential.ID).Scan(&details)
	if bytes.Contains(details, []byte(secret)) || !bytes.Contains(details, []byte("target_check")) {
		t.Fatal("invalid credential audit details")
	}
	dbfx.Exec(t, "UPDATE credentials SET expires_at=now()-interval '1 minute' WHERE id=$1", credential.ID)
	check(400)
	dbfx.QueryRow(t, "SELECT count(*) FROM activity_log WHERE action='credential_used' AND details->>'credential_id'=$1", credential.ID).Scan(&count)
	if count != 1 {
		t.Fatal("expired credential was used")
	}
}
