// Package service registers the daemon to start at logon and starts or stops
// it from the CLI.
//
// The daemon is always per-user and always starts at logon, on every platform
// and in every install mode. A boot-time system service would run as
// SYSTEM/root before anyone logs in, with no access to a user profile and no
// way to know which of several users it serves — so admin rights change where
// the binary goes, never the process model.
//
// Each platform's native mechanism is driven through its own command-line tool
// rather than an API binding: schtasks, launchctl, systemctl. They are the
// documented interface, they are already installed, and shelling out to them is
// a fraction of the code of talking to the Task Scheduler COM API.
package service

import (
	"fmt"
	"os/exec"
	"strings"
)

// Name is the scheduled task name, the systemd unit basename, and the human
// label. Label is the reverse-DNS form launchd wants.
const (
	Name  = "Mnemosyne"
	Label = "dev.mnemosyne.daemon"
)

// State is what Query found.
type State struct {
	// Mechanism names the platform facility, for a message the user can act on.
	Mechanism string

	// Registered reports whether the logon entry exists.
	Registered bool

	// Running is the platform's own view of the process. It is best effort:
	// connecting to the channel is the authoritative check, and that is what
	// the CLI reports alongside this.
	Running bool

	// Path is the task, plist, or unit file.
	Path string

	// Detail carries whatever the platform tool said, for troubleshooting.
	Detail string
}

// ErrNotRegistered is returned by Start and Stop when there is nothing to act
// on, so the CLI can say "run service install" instead of echoing a tool error.
var ErrNotRegistered = fmt.Errorf("the logon entry is not registered")

// run executes a platform tool and folds its output into any error, since the
// output is the only useful diagnostic these tools produce.
func run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text != "" {
			return text, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, text)
		}
		return text, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return text, nil
}
