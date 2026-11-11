package workbench

import (
	"context"
	"errors"
	"net"

	"golang.org/x/crypto/ssh"
)

// DiscoverSSHHostKey stops the handshake before user authentication. The caller
// must confirm this candidate through a trusted channel before using credentials.
func DiscoverSSHHostKey(ctx context.Context, host, port string) (string, error) {
	address := net.JoinHostPort(host, port)
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return "", errors.New("SSH server is unreachable")
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	fingerprint := ""
	config := &ssh.ClientConfig{User: "multica-host-discovery", HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fingerprint = ssh.FingerprintSHA256(key)
		return errors.New("host discovery complete")
	}}
	_, _, _, _ = ssh.NewClientConn(connection, address, config)
	if fingerprint == "" {
		return "", errors.New("could not read the SSH server identity")
	}
	return fingerprint, nil
}
