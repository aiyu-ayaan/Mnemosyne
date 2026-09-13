package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aiyu-ayaan/mnemosyne/internal/config"
	"github.com/aiyu-ayaan/mnemosyne/internal/install"
)

const agentsUsage = `Usage: mnemosyne agents <status|install|uninstall>

The single entry point: one command sets Mnemosyne up in every AI agent on this
machine, so a new session knows what is already remembered without being told.

Two things reach an agent, and they are not the same:

  the MCP server   the tools themselves, plus Mnemosyne's instructions, handed
                   to the client when it connects. Every MCP client can take
                   this; it is what makes recall and write_memory exist at all.
  the hook         a session-start command whose output goes straight into the
                   agent's context before the user's first message. Only some
                   clients have one, and it is the half an agent cannot skip.

"agents install" does both, for every client it finds:

  Claude Code   MCP + hook
  Codex         MCP + hook
  Cursor        MCP
  Windsurf      MCP

A client that is not installed is skipped rather than having its configuration
invented. Anything else speaking MCP takes the JSON that "agents status" prints.

These files belong to you: each is backed up before the first edit, only
Mnemosyne's own entry is added, and "agents uninstall" takes back exactly that.
`

// serverName is what Mnemosyne registers itself as in every client.
const serverName = "mnemosyne"

// hookMatcher is when Claude Code fires SessionStart. A resumed or compacted
// session has lost the block along with the rest of the context, so it is
// needed there as much as at startup.
const hookMatcher = "startup|resume|clear|compact"

// agentTarget is one AI client and the two places Mnemosyne reaches it.
type agentTarget struct {
	name string

	// home is the client's configuration directory. Its absence is how we know
	// the client is not on this machine — writing config for something that is
	// not installed leaves litter behind that nothing will ever read.
	home func() (string, error)

	// hookFile holds session-start hooks. Nil when the client has no hook
	// mechanism, which is most of them.
	hookFile func() (string, error)

	// mcpFile is a JSON config with an "mcpServers" object.
	mcpFile func() (string, error)

	// mcpCLI is the client's own command for registering a server, used when
	// its config is not JSON we can safely edit. Codex keeps TOML, and driving
	// its documented CLI beats taking on a TOML writer to reach one table —
	// the same reasoning the service package uses for schtasks and launchctl.
	mcpCLI []string
}

func agentTargets() []agentTarget {
	return []agentTarget{
		{
			name:     "Claude Code",
			home:     underHome(".claude"),
			hookFile: underHome(".claude", "settings.json"),
			mcpFile:  underHome(".claude.json"),
		},
		{
			name:     "Codex",
			home:     underHome(".codex"),
			hookFile: underHome(".codex", "hooks.json"),
			mcpCLI:   []string{"codex", "mcp", "add"},
		},
		{
			name:    "Cursor",
			home:    underHome(".cursor"),
			mcpFile: underHome(".cursor", "mcp.json"),
		},
		{
			name:    "Windsurf",
			home:    underHome(".codeium", "windsurf"),
			mcpFile: underHome(".codeium", "windsurf", "mcp_config.json"),
		},
	}
}

// underHome resolves a path inside the user's home directory.
func underHome(parts ...string) func() (string, error) {
	return func() (string, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(append([]string{home}, parts...)...), nil
	}
}

// present reports whether the client is on this machine.
func (t agentTarget) present() bool {
	dir, err := t.home()
	if err != nil {
		return false
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func agentsCmd(args []string) error {
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}

	switch action {
	case "status":
		return agentsStatus()
	case "install":
		return agentsInstall()
	case "uninstall":
		return agentsUninstall()
	case "help", "--help", "-h":
		fmt.Print(agentsUsage)
		return nil
	default:
		return fmt.Errorf("unknown agents subcommand %q\n\n%s", action, agentsUsage)
	}
}

// binaryPathForAgents is the absolute path written into every client's config.
//
// Absolute rather than the bare name, because an agent launched from a GUI does
// not always inherit the PATH a terminal has — and a client that cannot find
// the binary reports "program not found" with nothing to act on.
//
// It prefers the installed copy over the running one. Running this from a
// workspace build would otherwise point every agent on the machine at a binary
// that `pnpm dev` rebuilds and, on Windows, locks while it does.
func binaryPathForAgents() (string, error) {
	if dir, err := install.Dir(install.User); err == nil {
		candidate := filepath.Join(dir, binaryName())
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate the running binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}

// binaryName is the installed executable's filename.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "mnemosyne.exe"
	}
	return "mnemosyne"
}

// hookCommandFor is the command line a client runs at session start.
func hookCommandFor(exe string) string {
	// Plain quotes, not %q: a Windows path is full of backslashes, and the
	// escaping %q adds would reach the shell as literal double backslashes.
	if strings.ContainsAny(exe, " \t") {
		return `"` + exe + `" hook session-start`
	}
	return exe + " hook session-start"
}

// --- the mcpServers object, which every JSON-configured client shares ---

// serverEntry is Mnemosyne as one client's MCP server definition.
func serverEntry(exe string) map[string]any {
	return map[string]any{"type": "stdio", "command": exe, "args": []any{"serve"}}
}

// servers returns the mcpServers object, creating it only when asked, so a
// status check never grows the document.
func servers(doc map[string]any, create bool) map[string]any {
	inner, ok := doc["mcpServers"].(map[string]any)
	if ok {
		return inner
	}
	if !create {
		return nil
	}
	inner = map[string]any{}
	doc["mcpServers"] = inner
	return inner
}

// hasServer reports whether Mnemosyne is registered and pointing at exe. A
// registration for a binary that has since moved counts as absent, so install
// replaces it rather than leaving the client calling a path that is gone.
func hasServer(doc map[string]any, exe string) bool {
	root := servers(doc, false)
	if root == nil {
		return false
	}
	entry, ok := root[serverName].(map[string]any)
	if !ok {
		return false
	}
	command, _ := entry["command"].(string)
	return command == exe
}

func addServer(doc map[string]any, exe string) {
	servers(doc, true)[serverName] = serverEntry(exe)
}

func removeServer(doc map[string]any) bool {
	root := servers(doc, false)
	if root == nil {
		return false
	}
	if _, ok := root[serverName]; !ok {
		return false
	}
	delete(root, serverName)
	if len(root) == 0 {
		delete(doc, "mcpServers")
	}
	return true
}

func agentsStatus() error {
	exe, err := binaryPathForAgents()
	if err != nil {
		return err
	}
	cmd := hookCommandFor(exe)
	fmt.Printf("server name   %s\n", serverName)
	fmt.Printf("binary        %s\n", exe)
	fmt.Printf("hook command  %s\n\n", cmd)

	for _, t := range agentTargets() {
		if !t.present() {
			fmt.Printf("%-12s not installed on this machine\n", t.name)
			continue
		}
		fmt.Printf("%-12s MCP %s · hook %s\n", t.name, t.mcpState(exe), t.hookState(cmd))
	}

	fmt.Printf("\nAny other MCP client takes this:\n\n%s\n", genericConfig(exe))
	return nil
}

// mcpState describes whether the MCP server is registered, for status output.
func (t agentTarget) mcpState(exe string) string {
	switch {
	case t.mcpCLI != nil:
		registered, err := t.cliHasServer()
		if err != nil {
			return "unknown (" + err.Error() + ")"
		}
		if registered {
			return "registered"
		}
		return "not registered"
	case t.mcpFile != nil:
		path, err := t.mcpFile()
		if err != nil {
			return "unknown"
		}
		doc, err := readJSON(path)
		if err != nil {
			return "unreadable"
		}
		if doc != nil && hasServer(doc, exe) {
			return "registered"
		}
		return "not registered"
	}
	return "—"
}

// hookState describes whether the session-start hook is in place.
func (t agentTarget) hookState(cmd string) string {
	if t.hookFile == nil {
		return "unsupported"
	}
	path, err := t.hookFile()
	if err != nil {
		return "unknown"
	}
	doc, err := readJSON(path)
	if err != nil {
		return "unreadable"
	}
	if doc != nil && hasHook(doc, cmd) {
		return "installed"
	}
	return "not installed"
}

func agentsInstall() error {
	loc, err := config.Detect(false)
	if err != nil {
		return err
	}
	// This writes into user-level configuration that every session of every
	// agent reads. A development build has no business there.
	if err := refuseInDev(loc); err != nil {
		return err
	}

	exe, err := binaryPathForAgents()
	if err != nil {
		return err
	}
	cmd := hookCommandFor(exe)

	touched := 0
	for _, t := range agentTargets() {
		if !t.present() {
			fmt.Printf("%-12s not installed on this machine — skipped\n", t.name)
			continue
		}

		mcp := t.installMCP(exe)
		hook := t.installHook(cmd)
		fmt.Printf("%-12s MCP %s · hook %s\n", t.name, mcp, hook)
		touched++
	}

	if touched == 0 {
		return fmt.Errorf("no agent was found on this machine")
	}
	fmt.Printf("\nrestart any running agent session to pick it up\n")
	fmt.Printf("anything else speaking MCP takes this:\n\n%s\n", genericConfig(exe))
	return nil
}

// installMCP registers the server with one client and reports what happened.
func (t agentTarget) installMCP(exe string) string {
	switch {
	case t.mcpCLI != nil:
		registered, err := t.cliHasServer()
		if err != nil {
			return "failed: " + err.Error()
		}
		if registered {
			return "already registered"
		}
		args := append(append([]string{}, t.mcpCLI[1:]...), serverName, "--", exe, "serve")
		out, err := exec.Command(t.mcpCLI[0], args...).CombinedOutput()
		if err != nil {
			return fmt.Sprintf("failed: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return "registered"

	case t.mcpFile != nil:
		path, err := t.mcpFile()
		if err != nil {
			return "failed: " + err.Error()
		}
		doc, err := readJSON(path)
		if err != nil {
			return "left alone: not readable JSON"
		}
		if doc == nil {
			doc = map[string]any{}
		}
		if hasServer(doc, exe) {
			return "already registered"
		}
		addServer(doc, exe)
		if err := writeJSONBackedUp(path, doc); err != nil {
			return "failed: " + err.Error()
		}
		return "registered"
	}
	return "unsupported"
}

// installHook writes the session-start hook for one client.
func (t agentTarget) installHook(cmd string) string {
	if t.hookFile == nil {
		return "unsupported"
	}
	path, err := t.hookFile()
	if err != nil {
		return "failed: " + err.Error()
	}
	doc, err := readJSON(path)
	if err != nil {
		return "left alone: not readable JSON"
	}
	if doc == nil {
		doc = map[string]any{}
	}
	if hasHook(doc, cmd) {
		return "already installed"
	}
	addHook(doc, cmd)
	if err := writeJSONBackedUp(path, doc); err != nil {
		return "failed: " + err.Error()
	}
	return "installed"
}

func agentsUninstall() error {
	exe, err := binaryPathForAgents()
	if err != nil {
		return err
	}
	cmd := hookCommandFor(exe)

	for _, t := range agentTargets() {
		if !t.present() {
			continue
		}
		fmt.Printf("%-12s MCP %s · hook %s\n", t.name, t.removeMCP(), t.removeHook(cmd))
	}
	return nil
}

func (t agentTarget) removeMCP() string {
	switch {
	case t.mcpCLI != nil:
		// The same CLI that added it removes it, and "remove" is the verb every
		// one of them uses.
		args := append(append([]string{}, t.mcpCLI[1:len(t.mcpCLI)-1]...), "remove", serverName)
		out, err := exec.Command(t.mcpCLI[0], args...).CombinedOutput()
		if err != nil {
			return "nothing to remove"
		}
		_ = out
		return "removed"

	case t.mcpFile != nil:
		path, err := t.mcpFile()
		if err != nil {
			return "nothing to remove"
		}
		doc, err := readJSON(path)
		if err != nil || doc == nil || !removeServer(doc) {
			return "nothing to remove"
		}
		if err := writeJSONBackedUp(path, doc); err != nil {
			return "failed: " + err.Error()
		}
		return "removed"
	}
	return "—"
}

func (t agentTarget) removeHook(cmd string) string {
	if t.hookFile == nil {
		return "unsupported"
	}
	path, err := t.hookFile()
	if err != nil {
		return "nothing to remove"
	}
	doc, err := readJSON(path)
	if err != nil || doc == nil || !removeHook(doc, cmd) {
		return "nothing to remove"
	}
	if err := writeJSONBackedUp(path, doc); err != nil {
		return "failed: " + err.Error()
	}
	return "removed"
}

// cliHasServer asks a client's own CLI whether the server is already there.
// A client that is not on PATH is reported as an error rather than as "no", so
// install does not follow up by trying to add through a command that is missing.
func (t agentTarget) cliHasServer() (bool, error) {
	bin := t.mcpCLI[0]
	if _, err := exec.LookPath(bin); err != nil {
		return false, fmt.Errorf("%s is not on PATH", bin)
	}
	args := append(append([]string{}, t.mcpCLI[1:len(t.mcpCLI)-1]...), "list")
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		// An empty list exits non-zero in some versions. Absence is the safe
		// reading: adding an entry that exists is refused, which install reports.
		return false, nil
	}
	return listsServer(string(out)), nil
}

// listsServer finds Mnemosyne in a client's `mcp list` output.
//
// The name is matched as a whole first field, not as a substring: every one of
// these lists puts the server name in the first column, and a plain
// strings.Contains counts a "mnemosyne-dev" registration as this one — which
// silently skips the install the user asked for.
func listsServer(out string) bool {
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		// Some clients print "name: command", others pad a table column.
		if strings.TrimSuffix(fields[0], ":") == serverName {
			return true
		}
	}
	return false
}

// genericConfig is the snippet for a client Mnemosyne does not know about,
// which is every MCP client not in agentTargets.
func genericConfig(exe string) string {
	doc := map[string]any{"mcpServers": map[string]any{serverName: serverEntry(exe)}}
	data, err := json.MarshalIndent(doc, "  ", "  ")
	if err != nil {
		return ""
	}
	return "  " + string(data)
}

// --- JSON surgery ---
//
// These files belong to the user and hold other tools' entries, so every edit is
// additive and keyed by our own command string: install twice and one entry
// appears, uninstall takes back exactly what install added, and anything that
// cannot be parsed is left untouched rather than rewritten from scratch.

func readJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return map[string]any{}, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// writeJSONBackedUp keeps a copy of what was there before the first edit. These
// files can carry a lot of someone's setup, and a tool that rewrites one should
// leave them something to go back to.
func writeJSONBackedUp(path string, doc map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if existing, err := os.ReadFile(path); err == nil {
		backup := path + ".mnemosyne.bak"
		if _, err := os.Stat(backup); os.IsNotExist(err) {
			if err := os.WriteFile(backup, existing, 0o600); err != nil {
				return fmt.Errorf("back up %s: %w", path, err)
			}
		}
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// hooksRoot returns the map holding the SessionStart list. Both Claude Code and
// Codex nest their hooks under a "hooks" key; create says whether to add one
// when it is missing, so a read never grows the document.
func hooksRoot(doc map[string]any, create bool) map[string]any {
	inner, ok := doc["hooks"].(map[string]any)
	if ok {
		return inner
	}
	if !create {
		return nil
	}
	inner = map[string]any{}
	doc["hooks"] = inner
	return inner
}

func hasHook(doc map[string]any, cmd string) bool {
	root := hooksRoot(doc, false)
	if root == nil {
		return false
	}
	list, _ := root["SessionStart"].([]any)
	for _, entry := range list {
		if entryUses(entry, cmd) {
			return true
		}
	}
	return false
}

// entryUses reports whether one SessionStart entry runs cmd.
func entryUses(entry any, cmd string) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	inner, _ := m["hooks"].([]any)
	for _, h := range inner {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if s, _ := hm["command"].(string); s == cmd {
			return true
		}
	}
	return false
}

// addHook is idempotent in itself, not only because install checks first: a
// config that grows a duplicate entry every run is the classic way this kind of
// tool ends up running its hook four times per session.
func addHook(doc map[string]any, cmd string) {
	if hasHook(doc, cmd) {
		return
	}
	root := hooksRoot(doc, true)
	list, _ := root["SessionStart"].([]any)
	root["SessionStart"] = append(list, map[string]any{
		"matcher": hookMatcher,
		"hooks": []any{map[string]any{
			"type":          "command",
			"command":       cmd,
			"timeout":       10,
			"statusMessage": "Loading memories…",
		}},
	})
}

func removeHook(doc map[string]any, cmd string) bool {
	root := hooksRoot(doc, false)
	if root == nil {
		return false
	}
	list, _ := root["SessionStart"].([]any)
	kept := make([]any, 0, len(list))
	for _, entry := range list {
		if entryUses(entry, cmd) {
			continue
		}
		kept = append(kept, entry)
	}
	if len(kept) == len(list) {
		return false
	}
	if len(kept) == 0 {
		delete(root, "SessionStart")
	} else {
		root["SessionStart"] = kept
	}
	return true
}
