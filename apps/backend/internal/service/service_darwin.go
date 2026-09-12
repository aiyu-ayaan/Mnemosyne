package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const mechanism = "launchd LaunchAgent (~/Library/LaunchAgents)"

// plistTemplate keeps the daemon alive and starts it at login. Standard output
// goes to a log beside the plist rather than being discarded, because the only
// thing worse than a daemon that will not start is one that will not say why.
const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Exe}}</string>
		<string>daemon</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardErrorPath</key>
	<string>{{.Log}}</string>
</dict>
</plist>
`

// Register writes the LaunchAgent and bootstraps it into the current GUI
// session, so the daemon starts now as well as at the next login.
func Register(exe string) error {
	abs, err := filepath.Abs(exe)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", exe, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("no binary at %s to register: %w", abs, err)
	}

	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	body := plistTemplate
	for placeholder, value := range map[string]string{
		"{{.Label}}": Label,
		"{{.Exe}}":   abs,
		"{{.Log}}":   filepath.Join(filepath.Dir(path), Label+".log"),
	} {
		body = strings.ReplaceAll(body, placeholder, value)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	// Replacing an existing agent means booting the old one out first;
	// bootstrap fails on a label that is already loaded.
	_, _ = run("launchctl", "bootout", domain()+"/"+Label)
	if _, err := run("launchctl", "bootstrap", domain(), path); err != nil {
		return err
	}
	return nil
}

// Unregister boots the agent out and removes the plist.
func Unregister() error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	_, _ = run("launchctl", "bootout", domain()+"/"+Label)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// Query reports whether the plist exists and whether launchd has it running.
func Query() (State, error) {
	path, err := plistPath()
	if err != nil {
		return State{}, err
	}

	state := State{Mechanism: mechanism, Path: path}
	if _, err := os.Stat(path); err != nil {
		return state, nil
	}
	state.Registered = true

	out, err := run("launchctl", "print", domain()+"/"+Label)
	if err != nil {
		// Not loaded. The plist is still on disk, so it will load at the next
		// login — registered but not running is a real, reportable state.
		return state, nil
	}
	state.Detail = out
	state.Running = strings.Contains(out, "state = running") || strings.Contains(out, "pid = ")
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
	// kickstart starts it, and -k restarts it if it is already up, which is
	// what "start" should mean after replacing the binary.
	_, err = run("launchctl", "kickstart", "-k", domain()+"/"+Label)
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
	// KeepAlive would restart it immediately after a plain kill, so stopping
	// means booting it out of the session. The plist stays, so the next login
	// brings it back.
	_, err = run("launchctl", "bootout", domain()+"/"+Label)
	return err
}

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate the home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

// domain is the per-user GUI domain. gui/<uid> is what a LaunchAgent belongs
// to; user/<uid> would load it without a session and it could not reach the
// window server or, more importantly here, the login keychain later.
func domain() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}
