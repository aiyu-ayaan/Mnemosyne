//go:build !windows

package channel

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// SocketFile is the unix socket inside the runtime directory.
const SocketFile = "daemon.sock"

// endpoint is a path inside the runtime directory, which is already per-user
// and 0700, so the socket inherits the isolation rather than declaring it.
// The flag is irrelevant here: an isolated copy has its own runtime directory.
func endpoint(runtimeDir string, _ bool) (string, error) {
	return filepath.Join(runtimeDir, SocketFile), nil
}

func dial(ctx context.Context, endpoint string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", endpoint, err)
	}
	return conn, nil
}

// listen binds the socket. A leftover socket from a killed daemon is removed
// first: bind fails on an existing path, and refusing to start because the last
// run was killed would be worse than replacing a file nothing is listening on.
func listen(endpoint string) (net.Listener, error) {
	if conn, err := net.Dial("unix", endpoint); err == nil {
		conn.Close()
		return nil, fmt.Errorf("another mnemosyne daemon is already listening on %s", endpoint)
	}
	if err := os.Remove(endpoint); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("clear %s: %w", endpoint, err)
	}

	l, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", endpoint, err)
	}
	// Belt and braces: the directory is 0700, and so is the socket.
	if err := os.Chmod(endpoint, 0o600); err != nil {
		l.Close()
		return nil, fmt.Errorf("restrict %s: %w", endpoint, err)
	}
	return l, nil
}
