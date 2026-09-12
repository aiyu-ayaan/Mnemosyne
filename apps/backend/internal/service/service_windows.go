package service

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const mechanism = "Scheduled Task (logon trigger, current user)"

// taskXML is registered rather than the plain `schtasks /Create /SC ONLOGON`
// form, because the two reasons a Scheduled Task was chosen over the Run
// registry key — restart on failure and no console flash — are only reachable
// through the XML definition.
//
// StartWhenAvailable and the restart settings mean a daemon that dies, or a
// logon that happened while the machine was asleep, still ends up with a
// running daemon rather than silently nothing.
const taskXML = `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.3" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Mnemosyne memory daemon for the desktop app.</Description>
    <URI>\{{.Name}}</URI>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>{{.User}}</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>{{.User}}</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>true</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>{{.Exe}}</Command>
      <Arguments>daemon</Arguments>
      <WorkingDirectory>{{.WorkDir}}</WorkingDirectory>
    </Exec>
  </Actions>
</Task>
`

// Register creates the logon task for exe. Registering twice replaces the
// existing task rather than failing, so an upgrade is one call.
func Register(exe string) error {
	abs, err := filepath.Abs(exe)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", exe, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("no binary at %s to register: %w", abs, err)
	}

	user := os.Getenv("USERDOMAIN") + `\` + os.Getenv("USERNAME")
	if strings.HasPrefix(user, `\`) {
		user = os.Getenv("USERNAME")
	}

	body := taskXML
	for placeholder, value := range map[string]string{
		"{{.Name}}":    Name,
		"{{.User}}":    escape(user),
		"{{.Exe}}":     escape(abs),
		"{{.WorkDir}}": escape(filepath.Dir(abs)),
	} {
		body = strings.ReplaceAll(body, placeholder, value)
	}

	// schtasks insists the XML be UTF-16, exactly as the declaration above
	// says. A UTF-8 file with that declaration is rejected outright.
	path := filepath.Join(os.TempDir(), "mnemosyne-task.xml")
	if err := os.WriteFile(path, utf16LE(body), 0o600); err != nil {
		return fmt.Errorf("write the task definition: %w", err)
	}
	defer os.Remove(path)

	if _, err := run("schtasks", "/Create", "/TN", Name, "/XML", path, "/F"); err != nil {
		return err
	}
	return nil
}

// Unregister deletes the task. A task that is already gone is not an error:
// the caller's goal is that it be absent.
func Unregister() error {
	state, err := Query()
	if err != nil {
		return err
	}
	if !state.Registered {
		return nil
	}
	_, err = run("schtasks", "/Delete", "/TN", Name, "/F")
	return err
}

// Query reports whether the task exists and whether it is running.
func Query() (State, error) {
	state := State{Mechanism: mechanism, Path: `\` + Name}

	out, err := run("schtasks", "/Query", "/TN", Name, "/FO", "LIST")
	if err != nil {
		// schtasks exits non-zero for "the system cannot find the file
		// specified", which is the normal not-registered answer rather than a
		// failure to look.
		return state, nil
	}

	state.Registered = true
	state.Detail = out
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, "Status:") && strings.Contains(line, "Running") {
			state.Running = true
		}
	}
	return state, nil
}

// Start runs the task now, which is also how the daemon is started after an
// install without waiting for the next logon.
func Start() error {
	state, err := Query()
	if err != nil {
		return err
	}
	if !state.Registered {
		return ErrNotRegistered
	}
	_, err = run("schtasks", "/Run", "/TN", Name)
	return err
}

// Stop ends the running instance. The task stays registered, so the daemon
// comes back at the next logon.
func Stop() error {
	state, err := Query()
	if err != nil {
		return err
	}
	if !state.Registered {
		return ErrNotRegistered
	}
	if !state.Running {
		return nil
	}
	_, err = run("schtasks", "/End", "/TN", Name)
	return err
}

// escape makes a value safe inside the XML document.
func escape(v string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(v)); err != nil {
		return v
	}
	return b.String()
}

// utf16LE encodes with the byte-order mark schtasks expects.
func utf16LE(s string) []byte {
	out := []byte{0xFF, 0xFE}
	for _, r := range s {
		if r > 0xFFFF {
			r = '?' // no surrogate pairs needed for a path or a user name
		}
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}
