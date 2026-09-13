package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aiyu-ayaan/mnemosyne/internal/config"
)

const agentsUsage = `Usage: mnemosyne agents <status|install|uninstall>

Wires Mnemosyne into the agents on this machine so that every new session starts
knowing what is already remembered, without the user asking for it.

Two things reach an agent, and they are not the same:

  the MCP server   every client gets Mnemosyne's instructions at connect time.
                   "claude mcp add" / "codex mcp add" is what turns this on.
  the hook         a session-start command whose output goes straight into the
                   agent's context. This is what "agents install" writes, and it
                   is the only one an agent cannot quietly skip.

Clients without a hook mechanism (Cursor, Windsurf, Zed) get the first only.

"agents install" edits files that belong to you — it backs each one up first,
adds only its own entry, and "agents uninstall" takes back exactly that.
`

// hookMatcher is when Claude Code fires SessionStart. A resumed or compacted
// session has lost the block along with the rest of the context, so it is
// needed there as much as at startup.
const hookMatcher = "startup|resume|clear|compact"

// agentTarget is one client's hook configuration file.
type agentTarget struct {
	name string
	// path is the config file, resolved per user.
	path func() (string, error)
}

func agentTargets() []agentTarget {
	return []agentTarget{
		{name: "Claude Code", path: claudeSettingsPath},
		{name: "Codex", path: codexHooksPath},
	}
}

func claudeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func codexHooksPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "hooks.json"), nil
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

// hookCommand is the command line written into every agent's config. It is the
// binary's absolute path rather than the bare name, because an agent launched
// from a GUI does not always inherit the PATH a terminal has.
func hookCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate the running binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	// Plain quotes, not %q: a Windows path is full of backslashes, and the
	// escaping %q adds would reach the shell as literal double backslashes.
	if strings.ContainsAny(exe, " \t") {
		return `"` + exe + `" hook session-start`, nil
	}
	return exe + " hook session-start", nil
}

func agentsStatus() error {
	cmd, err := hookCommand()
	if err != nil {
		return err
	}
	fmt.Printf("hook command  %s\n\n", cmd)

	for _, t := range agentTargets() {
		path, err := t.path()
		if err != nil {
			fmt.Printf("%-12s could not resolve its config: %v\n", t.name, err)
			continue
		}
		doc, err := readJSON(path)
		switch {
		case err != nil:
			fmt.Printf("%-12s unreadable (%v)\n             %s\n", t.name, err, path)
			continue
		case doc == nil:
			fmt.Printf("%-12s not configured — no %s\n", t.name, path)
			continue
		}
		state := "not installed"
		if hasHook(doc, cmd) {
			state = "installed"
		}
		fmt.Printf("%-12s %s\n             %s\n", t.name, state, path)
	}
	return nil
}

func agentsInstall() error {
	loc, err := config.Detect(false)
	if err != nil {
		return err
	}
	// The hook is written into user-level configuration that every session of
	// every agent reads. A development build has no business there.
	if err := refuseInDev(loc); err != nil {
		return err
	}

	cmd, err := hookCommand()
	if err != nil {
		return err
	}

	installed := 0
	for _, t := range agentTargets() {
		path, err := t.path()
		if err != nil {
			fmt.Printf("%s: %v\n", t.name, err)
			continue
		}
		doc, err := readJSON(path)
		if err != nil {
			fmt.Printf("%s: %s is not readable JSON (%v) — left alone\n", t.name, path, err)
			continue
		}
		// A client that keeps no hook file yet gets one created; Codex has none
		// until something writes it.
		if doc == nil {
			doc = map[string]any{}
		}
		if hasHook(doc, cmd) {
			fmt.Printf("%-12s already installed\n", t.name)
			installed++
			continue
		}

		addHook(doc, cmd)
		if err := writeJSONBackedUp(path, doc); err != nil {
			fmt.Printf("%s: %v\n", t.name, err)
			continue
		}
		fmt.Printf("%-12s installed into %s\n", t.name, path)
		installed++
	}

	if installed == 0 {
		return fmt.Errorf("no agent configuration was changed")
	}
	fmt.Println("\nrestart any running agent session to pick it up")
	return nil
}

func agentsUninstall() error {
	cmd, err := hookCommand()
	if err != nil {
		return err
	}
	for _, t := range agentTargets() {
		path, err := t.path()
		if err != nil {
			continue
		}
		doc, err := readJSON(path)
		if err != nil || doc == nil {
			continue
		}
		if !removeHook(doc, cmd) {
			fmt.Printf("%-12s nothing to remove\n", t.name)
			continue
		}
		if err := writeJSONBackedUp(path, doc); err != nil {
			fmt.Printf("%s: %v\n", t.name, err)
			continue
		}
		fmt.Printf("%-12s removed from %s\n", t.name, path)
	}
	return nil
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
