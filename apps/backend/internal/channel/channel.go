// Package channel is the local transport between the daemon and its clients:
// a named pipe on Windows, a unix socket everywhere else.
//
// There is no TCP port, so nothing on the network can reach it, no firewall
// prompt appears on first run, and there is no port to collide with. The OS is
// the security boundary — a pipe DACL or unix file permissions already enforce
// "this user, this machine". The token in tokenFile is defence in depth against
// another process running as the same user, not the boundary itself.
package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// TokenFile holds the bearer token every request must present. It lives in the
// runtime directory with the same owner-only permissions as the socket.
const TokenFile = "daemon.token"

// BaseURL is the URL prefix clients use. The host is ignored — the connection
// is made to the pipe or socket, not resolved — but net/http requires one.
const BaseURL = "http://mnemosyne"

// Address is where the daemon listens.
type Address struct {
	// Runtime is the directory holding the token and, on Unix, the socket.
	Runtime string

	// Endpoint is the pipe name or socket path, as Dial and Listen want it.
	Endpoint string
}

// Resolve returns the address for a runtime directory. portable copies get
// their own endpoint so that an installed daemon and a portable one on the same
// machine do not fight over one name.
func Resolve(runtimeDir string, portable bool) (Address, error) {
	endpoint, err := endpoint(runtimeDir, portable)
	if err != nil {
		return Address{}, err
	}
	return Address{Runtime: runtimeDir, Endpoint: endpoint}, nil
}

// TokenPath is the token file for this address.
func (a Address) TokenPath() string { return filepath.Join(a.Runtime, TokenFile) }

// Dial opens a connection to the daemon.
func (a Address) Dial(ctx context.Context) (net.Conn, error) { return dial(ctx, a.Endpoint) }

// Listen starts listening, creating the runtime directory if needed. The
// listener is owner-only; see the platform files for how that is enforced.
func (a Address) Listen() (net.Listener, error) {
	if err := os.MkdirAll(a.Runtime, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", a.Runtime, err)
	}
	return listen(a.Endpoint)
}

// HTTPClient speaks HTTP over the channel. The payload is the same JSON the
// api package serves, so a client needs no protocol code of its own.
func (a Address) HTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return a.Dial(ctx)
		},
		// One daemon, one connection pool. SSE holds a connection for the life
		// of the window, so the cap has to leave room for ordinary requests.
		MaxIdleConns:    4,
		MaxConnsPerHost: 16,
	}}
}

// NewToken writes a fresh token, replacing any existing one. Every daemon start
// mints a new one, so a stale token from a previous run cannot be replayed.
func (a Address) NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate a daemon token: %w", err)
	}
	token := hex.EncodeToString(buf)

	if err := os.MkdirAll(a.Runtime, 0o700); err != nil {
		return "", fmt.Errorf("create %s: %w", a.Runtime, err)
	}
	// 0600 is the point of the file: on Unix it is what stops another account
	// reading it. On Windows the mode is advisory, and the profile directory's
	// inherited ACL is what applies.
	if err := os.WriteFile(a.TokenPath(), []byte(token), 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", a.TokenPath(), err)
	}
	return token, nil
}

// ReadToken reads the token a running daemon wrote. A missing file means no
// daemon has run, which is not an error: the caller learns that by failing to
// connect, with a better message than "token missing".
func (a Address) ReadToken() (string, error) {
	data, err := os.ReadFile(a.TokenPath())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", a.TokenPath(), err)
	}
	return strings.TrimSpace(string(data)), nil
}

// RemoveToken deletes the token file on shutdown so that a dead daemon does not
// leave a credential behind.
func (a Address) RemoveToken() error {
	if err := os.Remove(a.TokenPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
