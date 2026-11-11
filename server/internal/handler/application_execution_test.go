package handler

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
	"github.com/multica-ai/multica/server/internal/workbench"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"golang.org/x/crypto/ssh"
)

func TestApplicationExecutionPersistsUnknownAndRetriesOnlyUnfinishedSteps(t *testing.T) {
	for _, method := range []string{"key", "password"} {
		t.Run(method, func(t *testing.T) { testApplicationExecutionRecovery(t, method) })
	}
}

func testApplicationExecutionRecovery(t *testing.T, method string) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	_, hostKey, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, _ := ssh.NewSignerFromKey(hostKey)
	_, clientKey, _ := ed25519.GenerateKey(rand.Reader)
	clientSigner, _ := ssh.NewSignerFromKey(clientKey)
	clientDER, _ := x509.MarshalPKCS8PrivateKey(clientKey)
	clientPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientDER})
	credentialKind := "ssh-key"
	if method == "password" {
		clientPEM = []byte("fixture-password")
		credentialKind = "ssh-password"
	}
	config := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if !bytes.Equal(key.Marshal(), clientSigner.PublicKey().Marshal()) {
			return nil, io.EOF
		}
		return nil, nil
	}}
	config.PasswordCallback = func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if method != "password" || !bytes.Equal(password, clientPEM) {
			return nil, io.EOF
		}
		return nil, nil
	}
	config.AddHostKey(hostSigner)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var connections sync.WaitGroup
	var appCalls, sqlCalls atomic.Int32
	t.Cleanup(func() { listener.Close(); connections.Wait() })
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			go func() {
				defer connections.Done()
				defer connection.Close()
				_, channels, requests, err := ssh.NewServerConn(connection, config)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					channel, requests, err := incoming.Accept()
					if err != nil {
						return
					}
					for request := range requests {
						if request.Type != "exec" {
							_ = request.Reply(false, nil)
							continue
						}
						_ = request.Reply(true, nil)
						script, _ := io.ReadAll(channel)
						status := uint32(0)
						if strings.HasPrefix(string(script), "for tool in ") {
							// Prerequisite checks do not execute application or SQL changes.
						} else if strings.Contains(string(script), "psql ") {
							if sqlCalls.Add(1) == 1 {
								status = 1
							}
						} else {
							appCalls.Add(1)
						}
						_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
						_ = channel.Close()
						break
					}
				}
			}()
		}
	}()
	project := dbfx.Project(t, "Execution recovery")
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM workbench_release_orders WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, project)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM project_deployment WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, project)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM credentials WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, project)
	})
	h := *testHandler
	h.WorkbenchSecretBox, _ = secretbox.New(bytes.Repeat([]byte{5}, 32))
	var credential struct {
		ID       string `json:"id"`
		LastFour string `json:"last_four"`
	}
	testutil.Call(t, h.SaveWorkbenchCredential, newRequest("POST", "/api/workbench/credentials", map[string]any{"project_id": project, "label": "Test SSH", "kind": credentialKind, "usage": []string{"deploy"}, "secret": string(clientPEM)})).Want(200).JSON(&credential)
	if method == "password" && credential.LastFour != "" {
		t.Fatal("password suffix exposed in credential metadata")
	}
	appID, nodeID := uuidToString(dbid.NewV7()), uuidToString(dbid.NewV7())
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	deployment := workbench.ProjectDeploymentConfig{Mode: "orchestrated", Resources: []workbench.DeploymentResource{{ID: nodeID, Name: "Test server", Env: "staging", Host: host, Port: port, Username: "deploy", HostKey: ssh.FingerprintSHA256(hostSigner.PublicKey()), CredentialID: credential.ID}}, Applications: []workbench.DeploymentApplication{{ID: appID, Name: "Website", SourceKind: "directory", Source: "/srv/source", ArtifactPath: ".", Environments: map[string]workbench.ApplicationDeployment{"staging": {Kind: "nginx", Nodes: []string{nodeID}, Directory: "/srv/website", Domain: "example.test", HTTPS: "off", Strategy: "serial", HealthPath: "/"}}}}}
	deployment.Resources[0].AuthMethod = method
	var draftCheck struct {
		OK bool `json:"ok"`
	}
	testutil.Call(t, h.CheckDeploymentResource, testutil.WithURLParams(newRequest("POST", "/check-resource", deployment.Resources[0]), "id", project)).Want(200).JSON(&draftCheck)
	if !draftCheck.OK {
		t.Fatal("unsaved resource did not pass connection check")
	}
	stored, revision, err := readProjectDeployment(context.Background(), h.Queries, parseUUID(testWorkspaceID), parseUUID(project))
	if err != nil || revision != 0 || len(stored.Resources) != 0 {
		t.Fatal("connection check persisted the draft")
	}
	invalid := deployment.Resources[0]
	invalid.CredentialID = "invalid"
	testutil.Call(t, h.CheckDeploymentResource, testutil.WithURLParams(newRequest("POST", "/check-resource", invalid), "id", project)).Want(400)
	invalid = deployment.Resources[0]
	invalid.HostKey = "SHA256:" + strings.Repeat("A", 43)
	testutil.Call(t, h.CheckDeploymentResource, testutil.WithURLParams(newRequest("POST", "/check-resource", invalid), "id", project)).Want(200).JSON(&draftCheck)
	if draftCheck.OK {
		t.Fatal("incorrect fingerprint passed draft check")
	}
	appCalls.Store(0)
	testutil.Call(t, h.SaveProjectDeployment, testutil.WithURLParams(newRequest("PUT", "/api/projects/"+project+"/deployment", map[string]any{"config": deployment, "expected_revision": 0}), "id", project)).Want(200)
	var discovered struct {
		HostKey string `json:"host_key"`
	}
	testutil.Call(t, h.DiscoverDeploymentHost, testutil.WithURLParams(newRequest("POST", "/discover", map[string]string{"host": host, "port": port}), "id", project)).Want(200).JSON(&discovered)
	if discovered.HostKey != deployment.Resources[0].HostKey {
		t.Fatal("discovered incorrect server identity")
	}
	appChange := uuidToString(dbid.NewV7())
	delivery := workbench.ApplicationRelease{Title: "Recover SQL", Mode: "orchestrated", Changes: []workbench.ReleaseChange{{ID: appChange, Name: "Website", Type: "app", ApplicationID: appID, Version: "v1"}, {ID: uuidToString(dbid.NewV7()), Name: "Database", Type: "sql", ApplicationID: appID, After: appChange, Version: "migration-1", Database: "production", Body: "SELECT 1", Rehearsed: true}}}
	var order struct {
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
	}
	testutil.Call(t, h.CreateReleaseOrder, newRequest("POST", "/api/releases", workbench.ReleaseOrderInput{ProjectID: project, Env: "staging", RequestID: uuidToString(dbid.NewV7()), Delivery: &delivery})).Want(201).JSON(&order)
	testutil.Call(t, h.ConfirmReleaseOrder, testutil.WithURLParams(newRequest("POST", "/api/releases/"+order.ID+"/confirm", map[string]any{"expected_revision": order.Revision, "changes": true, "migration": true}), "releaseId", order.ID)).Want(200).JSON(&order)
	execute := func(acknowledged bool, status int) {
		t.Helper()
		testutil.Call(t, h.ExecuteApplicationRelease, testutil.WithURLParams(newRequest("POST", "/api/releases/"+order.ID+"/execute", map[string]any{"expected_revision": order.Revision, "acknowledged": acknowledged}), "releaseId", order.ID)).Want(status)
	}
	wait := func(want string) workbench.ReleaseManifest {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			var status string
			var raw []byte
			dbfx.QueryRow(t, `SELECT status,revision,manifest FROM workbench_release_orders WHERE workspace_id=$1 AND id=$2`, testWorkspaceID, order.ID).Scan(&status, &order.Revision, &raw)
			if status == want {
				var manifest workbench.ReleaseManifest
				if err := json.Unmarshal(raw, &manifest); err != nil {
					t.Fatal(err)
				}
				return manifest
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("execution did not reach %s", want)
		return workbench.ReleaseManifest{}
	}
	execute(false, 202)
	manifest := wait("partial")
	if manifest.Delivery.Results[1].Status != "unknown" || appCalls.Load() != 1 || sqlCalls.Load() != 1 {
		t.Fatalf("unexpected partial results: %+v", manifest.Delivery.Results)
	}
	execute(false, 400)
	execute(true, 202)
	wait("live")
	if appCalls.Load() != 1 || sqlCalls.Load() != 2 {
		t.Fatalf("successful application was replayed: app=%d SQL=%d", appCalls.Load(), sqlCalls.Load())
	}
	var overview struct {
		Deployments []struct {
			ApplicationID string `json:"application_id"`
			Version       string `json:"version"`
			Status        string `json:"status"`
		} `json:"deployments"`
	}
	testutil.Call(t, h.GetProjectDeployment, testutil.WithURLParams(newRequest("GET", "/deployment", nil), "id", project)).Want(200).JSON(&overview)
	if len(overview.Deployments) != 1 || overview.Deployments[0].ApplicationID != appID || overview.Deployments[0].Version != "v1" || overview.Deployments[0].Status != "success" {
		t.Fatalf("deployment overview lost execution evidence: %+v", overview)
	}
	var check struct {
		OK bool `json:"ok"`
	}
	testutil.Call(t, h.CheckDeploymentResource, testutil.WithURLParams(newRequest("POST", "/check", nil), "id", project, "resourceId", nodeID)).Want(200).JSON(&check)
	if !check.OK {
		t.Fatal("valid SSH resource did not pass connection check")
	}
	testutil.Call(t, h.CheckDeploymentResource, testutil.WithURLParams(newRequest("POST", "/check", nil), "id", project, "resourceId", uuidToString(dbid.NewV7()))).Want(404)
}
