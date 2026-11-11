package workbench

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestSSHDiscoveryAndPasswordVerifyHostBeforeAuthentication(t *testing.T) {
	_, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(privateKey)
	var authentications, executions atomic.Int32
	config := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		authentications.Add(1)
		if metadata.User() != "root" || string(password) != "test-password" {
			return nil, errors.New("denied")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			_, channels, requests, err := ssh.NewServerConn(connection, config)
			if err != nil {
				connection.Close()
				continue
			}
			go ssh.DiscardRequests(requests)
			for incoming := range channels {
				channel, requests, err := incoming.Accept()
				if err != nil {
					break
				}
				for request := range requests {
					if request.Type != "exec" {
						_ = request.Reply(false, nil)
						continue
					}
					_ = request.Reply(true, nil)
					_, _ = io.Copy(io.Discard, channel)
					executions.Add(1)
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
					channel.Close()
					break
				}
			}
			connection.Close()
		}
	}()
	t.Cleanup(func() { listener.Close(); <-done })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	fingerprint, err := DiscoverSSHHostKey(ctx, host, port)
	if err != nil || fingerprint != ssh.FingerprintSHA256(signer.PublicKey()) || authentications.Load() != 0 {
		t.Fatalf("discovery attempted authentication or returned the wrong identity: %v", err)
	}
	resource := DeploymentResource{Host: host, Port: port, Username: "root", AuthMethod: "password", HostKey: "SHA256:wrong"}
	if ExecuteDeploymentScript(ctx, resource, []byte("test-password"), "true\n") == nil || authentications.Load() != 0 {
		t.Fatal("sent password before verifying server identity")
	}
	resource.HostKey = fingerprint
	if err := ExecuteDeploymentScript(ctx, resource, []byte("test-password"), "true\n"); err != nil {
		t.Fatal(err)
	}
	if ExecuteDeploymentScript(ctx, resource, []byte("wrong-password"), "true\n") == nil {
		t.Fatal("accepted incorrect password")
	}
	if authentications.Load() != 2 || executions.Load() != 1 {
		t.Fatal("unexpected authentication or execution count")
	}
}
