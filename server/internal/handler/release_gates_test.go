package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func verifyReleaseConfirmations(t *testing.T, id, project string) {
	t.Helper()
	read := func() []workbench.ReleaseGate {
		t.Helper()
		var response struct {
			Gates []workbench.ReleaseGate `json:"gates"`
		}
		testutil.Call(t, testHandler.GetReleaseOrder, testutil.WithURLParams(newRequest("GET", "/api/releases/"+id, nil), "releaseId", id)).Want(200).JSON(&response)
		return response.Gates
	}
	gates := read()
	if len(gates) != 4 || !gates[0].Passed || gates[1].Passed || gates[2].Passed || gates[3].Passed {
		t.Fatalf("unexpected initial gates: %+v", gates)
	}
	roles := workbench.DefaultProjectRoles()
	roles.Roles["sec"] = workbench.ProjectRole{Enabled: false}
	raw, _ := json.Marshal(workbench.ProjectRoleConfiguration{Config: roles, Revision: 1})
	resource := dbfx.Insert(t, "project_resource", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "resource_type": "workbench_roles", "resource_ref": raw})
	confirm := func(revision int, status int, agent bool) {
		t.Helper()
		request := testutil.WithURLParams(newRequest("POST", "/api/releases/"+id+"/confirm", map[string]any{"expected_revision": revision, "changes": true, "migration": true, "security_waiver": true}), "releaseId", id)
		if agent {
			request.Header.Set("X-Actor-Source", "task_token")
			request.Header.Set("X-Agent-ID", testUserID)
		}
		testutil.Call(t, testHandler.ConfirmReleaseOrder, request).Want(status)
	}
	confirm(1, http.StatusForbidden, true)
	confirm(1, http.StatusOK, false)
	confirm(1, http.StatusConflict, false)
	gates = read()
	if !gates[0].Passed || !gates[1].Passed || !gates[2].Passed || gates[3].Passed {
		t.Fatalf("confirmation did not persist: %+v", gates)
	}
	var audits int
	dbfx.QueryRow(t, `SELECT count(*) FROM activity_log WHERE workspace_id=$1 AND action='release_confirmed' AND details->>'release_id'=$2`, testWorkspaceID, id).Scan(&audits)
	if audits != 1 {
		t.Fatalf("expected one successful confirmation audit, got %d", audits)
	}
	dbfx.Exec(t, `UPDATE project_resource SET resource_ref=jsonb_set(resource_ref,'{revision}','2') WHERE id=$1`, resource)
	if read()[1].Passed {
		t.Fatal("role revision change retained security waiver")
	}
	confirm(2, http.StatusOK, false)
	if !read()[1].Passed {
		t.Fatal("fresh waiver did not pass")
	}
}

func TestReleaseSecurityGateBindsRepositoryVersionAndCommit(t *testing.T) {
	project, repo := deployConfigFixture(t)
	issue := dbfx.Issue(t, "Release scan", testutil.Cols{"project_id": project})
	commit := strings.Repeat("a", 40)
	raw, _ := json.Marshal(workbench.ReleaseManifest{Repositories: []workbench.ReleaseRepositorySnapshot{{RepoID: repo, Tag: "v1", Merges: []workbench.ReleaseMerge{{RepoID: repo, MergeCommit: commit}}}}})
	order := db.WorkbenchReleaseOrder{WorkspaceID: parseUUID(testWorkspaceID), ProjectID: parseUUID(project), Env: "prod", Manifest: raw, Confirmations: []byte("{}")}
	check := func() workbench.ReleaseGate {
		t.Helper()
		gates, err := readReleaseGates(context.Background(), testHandler.Queries, order)
		if err != nil {
			t.Fatal(err)
		}
		return gates[1]
	}
	if check().Passed {
		t.Fatal("missing scan passed")
	}
	scan := dbfx.Insert(t, "security_scans", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "repo_id": repo, "issue_id": issue, "run_id": uuidToString(dbid.NewV7()), "artifact_version": "v1", "source_commit": commit, "status": "completed", "scanned_at": time.Now(), "actor_type": "member", "actor_id": testUserID, "report": "{}", "digest": "test"})
	if !check().Passed {
		t.Fatalf("matching scan rejected: %+v", check())
	}
	for _, column := range []string{"artifact_version", "source_commit", "status"} {
		value := map[string]string{"artifact_version": "v2", "source_commit": strings.Repeat("b", 40), "status": "failed"}[column]
		dbfx.Exec(t, "UPDATE security_scans SET "+column+"=$1 WHERE id=$2", value, scan)
		if check().Passed {
			t.Fatalf("mismatching %s passed", column)
		}
		dbfx.Exec(t, `UPDATE security_scans SET artifact_version='v1',source_commit=$1,status='completed' WHERE id=$2`, commit, scan)
	}
	risk := dbfx.Insert(t, "security_checks", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "repo_id": repo, "issue_id": issue, "fingerprint": "gate-risk", "severity": "medium", "category": "deps", "title": "Vulnerability", "location": "lockfile", "recommendation": "Update dependency", "artifact_version": "v1", "first_scan_id": scan, "last_scan_id": scan, "last_seen_at": time.Now()})
	if check().Passed {
		t.Fatal("unresolved medium risk passed")
	}
	dbfx.Exec(t, `UPDATE security_checks SET status='fixed' WHERE id=$1`, risk)
	if !check().Passed {
		t.Fatal("fixed risk blocked matching scan")
	}
}

func TestReleaseTargetGateRequiresEveryDeliveryAndFreshCheck(t *testing.T) {
	project, repo := deployConfigFixture(t)
	profile, err := workbench.BuildRepositoryProfile(workbench.RepositoryProfileInput{Template: "custom", Delivery: []string{"docker-image", "service-process"}}, workbench.RepositoryProfile{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"url": "https://example.com/repo.git", "workbench_profile_revision": 1, "stack": profile.Stack, "delivery": profile.Delivery})
	dbfx.Exec(t, `UPDATE project_resource SET resource_ref=$1 WHERE id=$2`, raw, repo)
	order := db.WorkbenchReleaseOrder{WorkspaceID: parseUUID(testWorkspaceID), ProjectID: parseUUID(project), Env: "staging"}
	manifest := workbench.ReleaseManifest{Repositories: []workbench.ReleaseRepositorySnapshot{{RepoID: repo}}}
	check := func() workbench.ReleaseGate {
		t.Helper()
		gate, err := readReleaseTargetGate(context.Background(), testHandler.Queries, order, manifest)
		if err != nil {
			t.Fatal(err)
		}
		return gate
	}
	if len(check().Blockers) != 2 {
		t.Fatal("missing delivery targets were not blocked")
	}
	runtime := dbfx.Runtime(t, "Release gate runtime")
	passed, _ := json.Marshal(map[string]any{"status": "passed", "ok": true, "free_bytes": 10000, "at": time.Now().UTC()})
	for _, kind := range []string{"docker-host", "service-process"} {
		location := map[string]string{"host": "localhost", "dir": "/tmp/app", "health_url": "http://localhost/health"}
		if kind == "docker-host" {
			location["image"] = "example:1"
		} else {
			location["start_command"] = "./app"
		}
		raw, _ := json.Marshal(location)
		dbfx.Insert(t, "deploy_targets", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "repo_id": repo, "label": kind, "env": "staging", "kind": kind, "channel": "runtime", "runtime_id": runtime, "location": raw, "last_check": passed})
	}
	if !check().Passed {
		t.Fatalf("complete targets rejected: %+v", check())
	}
	order.Env = "prod"
	if check().Passed {
		t.Fatal("staging targets satisfied production")
	}
	order.Env = "staging"
	dbfx.Exec(t, `UPDATE deploy_targets SET last_check=jsonb_set(last_check,'{at}',to_jsonb(now()-interval '25 hours')) WHERE project_id=$1 AND kind='service-process'`, project)
	gate := check()
	if gate.Passed || len(gate.Blockers) != 1 || gate.Blockers[0].Code != "target_check" {
		t.Fatalf("stale target passed: %+v", gate)
	}
}

func TestReleaseStagingRequiresAcceptanceAndTargetsWithoutProductionConfirmations(t *testing.T) {
	project, repo := deployConfigFixture(t)
	raw, _ := json.Marshal(workbench.ReleaseManifest{Repositories: []workbench.ReleaseRepositorySnapshot{{RepoID: repo, Tag: "v1"}}})
	order := db.WorkbenchReleaseOrder{WorkspaceID: parseUUID(testWorkspaceID), ProjectID: parseUUID(project), Env: "staging", Manifest: raw, Confirmations: []byte("{}")}
	gates, err := readReleaseGates(context.Background(), testHandler.Queries, order)
	if err != nil {
		t.Fatal(err)
	}
	for _, gate := range gates {
		if gate.ID == "security" || gate.ID == "changes" {
			if gate.Required || !gate.Passed || len(gate.Blockers) != 0 {
				t.Fatalf("production-only gate applied to staging: %+v", gate)
			}
		} else if !gate.Required || gate.Passed {
			t.Fatalf("missing acceptance or target was allowed: %+v", gate)
		}
	}
}
