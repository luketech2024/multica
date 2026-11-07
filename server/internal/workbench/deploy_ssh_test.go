package workbench

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestDeploySSHVerifiesHostBeforeCredentialsAndProbesWithoutStarting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("probe server fixture requires a POSIX shell")
	}
	for _, wrongHost := range []bool{false, true} {
		t.Run(strconv.FormatBool(wrongHost), func(t *testing.T) {
			_, hostPrivate, _ := ed25519.GenerateKey(rand.Reader)
			hostSigner, _ := ssh.NewSignerFromKey(hostPrivate)
			_, clientPrivate, _ := ed25519.GenerateKey(rand.Reader)
			clientSigner, _ := ssh.NewSignerFromKey(clientPrivate)
			encoded, _ := x509.MarshalPKCS8PrivateKey(clientPrivate)
			secret := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
			var authenticated atomic.Bool
			serverConfig := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
				if !bytes.Equal(key.Marshal(), clientSigner.PublicKey().Marshal()) {
					return nil, errors.New("wrong client")
				}
				authenticated.Store(true)
				return nil, nil
			}}
			serverConfig.AddHostKey(hostSigner)
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				connection, err := listener.Accept()
				if err != nil {
					return
				}
				defer connection.Close()
				_, channels, requests, err := ssh.NewServerConn(connection, serverConfig)
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
						var payload struct{ Command string }
						if ssh.Unmarshal(request.Payload, &payload) != nil {
							_ = request.Reply(false, nil)
							break
						}
						_ = request.Reply(true, nil)
						command := exec.Command("sh", "-c", payload.Command)
						command.Stdout, command.Stderr = channel, channel.Stderr()
						status := uint32(0)
						if command.Run() != nil {
							status = 1
						}
						_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
						_ = channel.Close()
						break
					}
				}
			}()
			directory := filepath.Join(t.TempDir(), "dir'$(never-run)")
			if err := os.Mkdir(directory, 0700); err != nil {
				t.Fatal(err)
			}
			program, _ := os.Executable()
			host, port, _ := net.SplitHostPort(listener.Addr().String())
			target := DeployTargetInput{Kind: "service-process", Location: map[string]string{
				"host": host, "port": port, "username": "probe", "dir": directory,
				"host_key": ssh.FingerprintSHA256(hostSigner.PublicKey()), "start_command": strconv.Quote(program) + " --must-not-run",
			}}
			if wrongHost {
				target.Location["host_key"] = "SHA256:wrong-host"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result := ProbeSSH(ctx, target, secret)
			if result.OK == wrongHost {
				t.Fatalf("unexpected result: %+v", result)
			}
			if authenticated.Load() == wrongHost {
				t.Fatal("host verification must precede authentication")
			}
			if !wrongHost {
				if result.FreeBytes == nil || *result.FreeBytes == 0 {
					t.Fatal("disk capacity missing")
				}
				entries, err := os.ReadDir(directory)
				if err != nil || len(entries) != 0 {
					t.Fatal("SSH probe did not clean up its temporary file")
				}
			}
			if !bytes.Equal(secret, make([]byte, len(secret))) {
				t.Fatal("private key buffer was retained")
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("SSH connection was not closed")
			}
		})
	}
}
