package channel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os/user"

	"github.com/Microsoft/go-winio"
)

// endpoint is a named pipe under the per-user namespace. A pipe has no
// directory permissions to inherit, so the owner goes in the name and the
// access control goes in the DACL below.
func endpoint(runtimeDir string, portable bool) (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("identify the current user: %w", err)
	}

	// Username, not SID: the pipe name is something a person reads in an error
	// message. The SID does the actual access control.
	name := "mnemosyne." + sanitise(u.Username)
	if portable {
		// Two portable copies in different folders are two independent
		// installations and must not share one pipe.
		sum := sha256.Sum256([]byte(runtimeDir))
		name += "." + hex.EncodeToString(sum[:4])
	}
	return `\\.\pipe\` + name, nil
}

func dial(ctx context.Context, endpoint string) (net.Conn, error) {
	conn, err := winio.DialPipeContext(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", endpoint, err)
	}
	return conn, nil
}

// listen creates the pipe with a DACL granting full access to this user and
// nobody else — not even Administrators, which would otherwise inherit it.
//
// D:P is a protected DACL, so no inherited entries are merged in; the single
// (A;;GA;;;<sid>) ace is the whole access list.
func listen(endpoint string) (net.Listener, error) {
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("identify the current user: %w", err)
	}

	l, err := winio.ListenPipe(endpoint, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + u.Uid + ")",
		MessageMode:        false,
		InputBufferSize:    64 * 1024,
		OutputBufferSize:   64 * 1024,
	})
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", endpoint, err)
	}
	return l, nil
}

// sanitise strips the domain prefix and anything a pipe name cannot carry.
func sanitise(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r == '\\' || r == '/':
			out = out[:0] // drop DOMAIN\, keep the account name
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		// The name must be stable across runs or the client cannot find it, so
		// an unusable username falls back to a constant rather than anything
		// derived from the moment.
		return "user"
	}
	return string(out)
}
