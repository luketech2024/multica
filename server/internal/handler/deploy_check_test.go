package handler

import (
	"context"
	"sync"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestDeployCheckClaimsOnceAndRejectsStaleReports(t *testing.T) {
	project, repo := deployConfigFixture(t)
	runtime := dbfx.Runtime(t, "Deployment runtime")
	var target DeployTargetResponse
	create := withURLParam(newRequest("POST", "/api/projects/"+project+"/deploy-targets", map[string]any{
		"label": "Staging service", "repo_id": repo, "env": "staging", "kind": "service-process", "channel": "runtime", "runtime_id": runtime,
		"location": map[string]string{"host": "configured-host", "dir": "/srv/service", "start_command": "service-server", "health_url": "https://example.test/health"},
	}), "id", project)
	testutil.Call(t, testHandler.SaveDeployTarget, create).Want(200).JSON(&target)
	request := func(revision int64, status int) {
		req := withURLParams(newRequest("POST", "/api/projects/"+project+"/deploy-targets/"+target.ID+"/check", map[string]any{"expected_revision": revision}), "id", project, "targetId", target.ID)
		result := testutil.Call(t, testHandler.CheckDeployTarget, req).Want(status)
		if status == 202 {
			result.JSON(&target)
		}
	}
	request(target.Revision, 202)
	check := workbench.ParseDeployCheck(target.LastCheck)
	if check.Status != "queued" || check.OK {
		t.Fatal("queued check was presented as passed")
	}
	request(target.Revision-1, 409)
	var workers sync.WaitGroup
	claims := make(chan *protocol.DaemonDeployCheck, 2)
	errors := make(chan error, 2)
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			claimed, err := testHandler.claimRuntimeDeployCheck(context.Background(), runtime)
			claims <- claimed
			errors <- err
		}()
	}
	workers.Wait()
	close(claims)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for claimed := range claims {
		if claimed != nil {
			count++
			if claimed.CheckID != check.ID || claimed.Directory != "/srv/service" {
				t.Fatalf("wrong claim: %+v", claimed)
			}
		}
	}
	if count != 1 {
		t.Fatalf("claimed %d times", count)
	}
	report := func(runtimeID, checkID string, status int) {
		req := withURLParams(newOwnerDaemonRequest("POST", "/api/daemon/check", protocol.DaemonDeployCheckResult{DirectoryReady: true, ToolReady: true, MS: 10}), "runtimeId", runtimeID, "targetId", target.ID, "checkId", checkID)
		result := testutil.Call(t, testHandler.ReportDeployTargetCheck, req).Want(status)
		if status == 200 {
			result.JSON(&target)
		}
	}
	other := dbfx.Runtime(t, "Other deployment runtime")
	report(other, check.ID, 404)
	report(runtime, check.ID, 200)
	if !workbench.ParseDeployCheck(target.LastCheck).OK {
		t.Fatal("successful probe was not saved")
	}
	revision := target.Revision
	report(runtime, check.ID, 200)
	if target.Revision != revision {
		t.Fatal("replayed result changed revision")
	}
	request(target.Revision, 202)
	report(runtime, check.ID, 409)
	current := workbench.ParseDeployCheck(target.LastCheck)
	dbfx.Exec(t, "UPDATE deploy_targets SET last_check=NULL,revision=revision+1 WHERE id=$1", target.ID)
	report(runtime, current.ID, 409)
}
