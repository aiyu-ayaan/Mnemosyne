package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const mechanism = "systemd --user unit (~/.config/systemd/user)"

// unitTemplate is a user unit, not a system one. default.target is the user
// session's equivalent of multi-user.target, so the daemon starts when the
// session does and stops when it ends.
//
// Lingering is deliberately not enabled: a daemon that keeps running after
// logout would hold the index open for a user who is not there.
const unitTemplate = `[Unit]
Description=Mnemosyne memory daemon
Documentation=https://github.com/aiyu-ayaan/mnemosyne

[Service]
Type=simple
ExecStart={{.Exe}} daemon
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

// Register writes the unit, enables it for the next login, and starts it now.
func Register(exe string) error {
	abs, err := filepath.Abs(exe)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", exe, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("no binary at %s to register: %w", abs, err)
	}

	path, err := unitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	body := strings.ReplaceAll(unitTemplate, "{{.Exe}}", abs)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	if _, err := run("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	if _, err := run("systemctl", "--user", "enable", "--now", unitName()); err != nil {
		return err
	}
	return nil
}

// Unregister disables the unit and removes it.
func Unregister() error {
	path, err := unitPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	_, _ = run("systemctl", "--user", "disable", "--now", unitName())
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	_, _ = run("systemctl", "--user", "daemon-reload")
	return nil
}

// Query reports whether the unit is enabled and active.
func Query() (State, error) {
	path, err := unitPath()
	if err != nil {
		return State{}, err
	}

	state := State{Mechanism: mechanism, Path: path}
	if _, err := os.Stat(path); err != nil {
		return state, nil
	}
	state.Registered = true

	// is-enabled and is-active exit non-zero for "no", which is an answer
	// rather than a failure, so the output is read and the error ignored.
	enabled, _ := run("systemctl", "--user", "is-enabled", unitName())
	active, _ := run("systemctl", "--user", "is-active", unitName())
	state.Registered = strings.TrimSpace(enabled) == "enabled"
	state.Running = strings.TrimSpace(active) == "active"
	state.Detail = strings.TrimSpace(enabled + " / " + active)
	return state, nil
}

func Start() error {
	state, err := Query()
	if err != nil {
		return err
	}
	if !state.Registered {
		return ErrNotRegistered
	}
	_, err = run("systemctl", "--user", "restart", unitName())
	return err
}

func Stop() error {
	state, err := Query()
	if err != nil {
		return err
	}
	if !state.Registered {
		return ErrNotRegistered
	}
	_, err = run("systemctl", "--user", "stop", unitName())
	return err
}

func unitName() string { return "mnemosyne.service" }

func unitPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the user config directory: %w", err)
	}
	return filepath.Join(dir, "systemd", "user", unitName()), nil
}
