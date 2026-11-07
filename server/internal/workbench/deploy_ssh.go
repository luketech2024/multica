package workbench

import (
	"context"
	"errors"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-shellwords"
	"golang.org/x/crypto/ssh"
)

// ProbeSSH validates the configured host identity before authentication and only runs a fixed probe.
func ProbeSSH(ctx context.Context, target DeployTargetInput, privateKey []byte) DeployCheck {
	started := time.Now()
	result := DeployCheck{Status: "failed"}
	defer func() { clear(privateKey) }()
	finish := func(detail string) DeployCheck {
		result.At, result.MS, result.Detail = time.Now().UTC(), time.Since(started).Milliseconds(), detail
		return result
	}
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return finish("SSH private key is invalid or requires a passphrase")
	}
	address := net.JoinHostPort(target.Location["host"], target.Location["port"])
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return finish("SSH host could not be reached")
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	config := &ssh.ClientConfig{
		User: target.Location["username"], Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			if ssh.FingerprintSHA256(key) != target.Location["host_key"] {
				return errors.New("SSH host key does not match")
			}
			return nil
		},
	}
	clientConnection, channels, requests, err := ssh.NewClientConn(connection, address, config)
	if err != nil {
		return finish("SSH authentication or host key verification failed")
	}
	client := ssh.NewClient(clientConnection, channels, requests)
	defer client.Close()
	command, err := sshDeploymentProbeCommand(target)
	if err != nil {
		return finish("SSH deployment directory or start command is invalid")
	}
	session, err := client.NewSession()
	if err != nil {
		return finish("SSH probe session could not be opened")
	}
	defer session.Close()
	output, err := session.Output(command)
	if err != nil {
		return finish("SSH directory, capacity or required tool probe failed")
	}
	kilobytes, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 53)
	if err != nil || kilobytes == 0 || kilobytes > (1<<53-1)/1024 {
		return finish("SSH disk capacity is unavailable or exhausted")
	}
	available := kilobytes * 1024
	result.FreeBytes, result.OK, result.Status = &available, true, "passed"
	return finish("SSH host identity, deployment directory, available disk capacity and required tools are ready")
}

func sshDeploymentProbeCommand(target DeployTargetInput) (string, error) {
	directory := target.Location["dir"]
	if !path.IsAbs(directory) || strings.ContainsAny(directory, "\x00\r\n") {
		return "", errors.New("absolute deployment directory required")
	}
	tool := "docker info --format '{{.ServerVersion}}' >/dev/null"
	if target.Kind == "service-process" {
		parser := shellwords.NewParser()
		parser.ParseEnv, parser.ParseBacktick = false, false
		words, err := parser.Parse(target.Location["start_command"])
		if err != nil || len(words) == 0 {
			return "", errors.New("start command required")
		}
		tool = "command -v " + sshShellQuote(words[0]) + " >/dev/null"
	}
	return "set -eu; cd " + sshShellQuote(directory) + "; " + tool +
		"; probe=$(mktemp .multica-deploy-check.XXXXXXXX); trap 'rm -f \"$probe\"' EXIT; " +
		"printf check >\"$probe\"; LC_ALL=C df -Pk . | awk 'NR==2 {print $4}'", nil
}

func sshShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
