package main

import (
	"encoding/json"
	"testing"
)

// otherTool is an entry that belongs to somebody else. Every assertion below is
// really about this: the user's config is not ours to rewrite, and an edit that
// loses another tool's hook is worse than no feature at all.
const otherTool = `{
  "hooks": {
    "SessionStart": [
      {"matcher": "startup", "hooks": [{"type": "command", "command": "other-tool init"}]}
    ],
    "PreToolUse": [
      {"hooks": [{"type": "command", "command": "other-tool guard"}]}
    ]
  },
  "model": "opus"
}`

func parse(t *testing.T, raw string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return doc
}

func TestHookAddIsIdempotentAndReversible(t *testing.T) {
	const cmd = `"C:\Program Files\mnemosyne.exe" hook session-start`
	doc := parse(t, otherTool)

	if hasHook(doc, cmd) {
		t.Fatal("reported installed before anything was written")
	}

	addHook(doc, cmd)
	addHook(doc, cmd) // installing twice must leave one entry
	if !hasHook(doc, cmd) {
		t.Fatal("not reported installed after addHook")
	}

	list := doc["hooks"].(map[string]any)["SessionStart"].([]any)
	if len(list) != 2 {
		t.Fatalf("SessionStart has %d entries, want 2 (theirs + ours)", len(list))
	}
	if !entryUses(list[0], "other-tool init") {
		t.Error("the other tool's SessionStart entry was lost")
	}

	if !removeHook(doc, cmd) {
		t.Fatal("removeHook reported nothing to remove")
	}
	if hasHook(doc, cmd) {
		t.Error("still installed after removeHook")
	}

	// Back to exactly what was there before, other keys included.
	want := parse(t, otherTool)
	got, _ := json.Marshal(doc)
	wantJSON, _ := json.Marshal(want)
	if string(got) != string(wantJSON) {
		t.Errorf("uninstall did not restore the document:\n got %s\nwant %s", got, wantJSON)
	}
}

func TestHookAddCreatesMissingStructure(t *testing.T) {
	doc := map[string]any{}
	addHook(doc, "mnemosyne hook session-start")
	if !hasHook(doc, "mnemosyne hook session-start") {
		t.Fatal("hook not found in a document built from nothing")
	}

	// And removing the only entry takes the empty list with it rather than
	// leaving "SessionStart": [] behind in someone's settings.
	removeHook(doc, "mnemosyne hook session-start")
	if _, still := doc["hooks"].(map[string]any)["SessionStart"]; still {
		t.Error("an empty SessionStart list was left behind")
	}
}

// TestHookReadsDoNotGrowTheDocument guards the case that would be invisible:
// a status check on a config with no hooks at all must not add a "hooks" key.
func TestHookReadsDoNotGrowTheDocument(t *testing.T) {
	doc := parse(t, `{"model": "opus"}`)
	if hasHook(doc, "mnemosyne hook session-start") {
		t.Fatal("found a hook in a document that has none")
	}
	if _, added := doc["hooks"]; added {
		t.Error("a read added a hooks key")
	}
}

// TestServerRegistrationIsIdempotentAndReversible is the mcpServers half of the
// same promise the hook half makes: other people's servers survive, ours
// appears once, and uninstall leaves the document as it was found.
const otherServers = `{
  "mcpServers": {
    "codegraph": {"type": "stdio", "command": "codegraph", "args": ["serve", "--mcp"]}
  },
  "numStartups": 41
}`

func TestServerRegistrationIsIdempotentAndReversible(t *testing.T) {
	const exe = `C:\Program Files\Mnemosyne\mnemosyne.exe`
	doc := parse(t, otherServers)

	if hasServer(doc, exe) {
		t.Fatal("reported registered before anything was written")
	}

	addServer(doc, exe)
	addServer(doc, exe)
	if !hasServer(doc, exe) {
		t.Fatal("not reported registered after addServer")
	}

	root := doc["mcpServers"].(map[string]any)
	if len(root) != 2 {
		t.Fatalf("mcpServers holds %d entries, want 2 (theirs + ours)", len(root))
	}
	if _, kept := root["codegraph"]; !kept {
		t.Error("the other server's registration was lost")
	}

	if !removeServer(doc) {
		t.Fatal("removeServer reported nothing to remove")
	}
	got, _ := json.Marshal(doc)
	want, _ := json.Marshal(parse(t, otherServers))
	if string(got) != string(want) {
		t.Errorf("uninstall did not restore the document:\n got %s\nwant %s", got, want)
	}
}

// TestServerRegistrationFollowsAMovedBinary matters on upgrade: a registration
// pointing at a path the binary no longer occupies is worse than none, because
// the client reports "program not found" and the user has nothing to act on.
func TestServerRegistrationFollowsAMovedBinary(t *testing.T) {
	doc := map[string]any{}
	addServer(doc, "/old/path/mnemosyne")

	if hasServer(doc, "/new/path/mnemosyne") {
		t.Fatal("a registration for the old path counted as the new one")
	}
	addServer(doc, "/new/path/mnemosyne")

	entry := doc["mcpServers"].(map[string]any)["mnemosyne"].(map[string]any)
	if entry["command"] != "/new/path/mnemosyne" {
		t.Errorf("command = %v, want the new path", entry["command"])
	}
}

// TestReadsDoNotAddAnMcpServersKey is the same guard the hook side has: a
// status check must not leave structure behind in someone's config.
func TestReadsDoNotAddAnMcpServersKey(t *testing.T) {
	doc := parse(t, `{"numStartups": 41}`)
	if hasServer(doc, "whatever") {
		t.Fatal("found a server in a document that has none")
	}
	if _, added := doc["mcpServers"]; added {
		t.Error("a read added an mcpServers key")
	}
}

// TestListsServerDoesNotMatchASuffixedName is the regression test for a real
// false positive: a "mnemosyne-dev" registration made install believe the real
// server was already there and skip it.
func TestListsServerDoesNotMatchASuffixedName(t *testing.T) {
	// codex: a padded table, name in the first column.
	const codexOnlyDev = `Name           Command                          Args   Env
codegraph      codegraph                        serve  -
mnemosyne-dev  C:\repo\bin\mnemosyne.exe        serve  MNEMOSYNE_DEV=*****`
	if listsServer(codexOnlyDev) {
		t.Error("mnemosyne-dev counted as mnemosyne")
	}

	const codexBoth = codexOnlyDev + "\n" + `mnemosyne      C:\Programs\mnemosyne.exe        serve  -`
	if !listsServer(codexBoth) {
		t.Error("a real registration was not found")
	}

	// claude: "name: command - status".
	if !listsServer(`mnemosyne: C:\Programs\mnemosyne.exe serve - Connected`) {
		t.Error("the name: command form was not recognised")
	}
	if listsServer(`mnemosyne-dev: C:\repo\bin\mnemosyne.exe serve - Connected`) {
		t.Error("mnemosyne-dev counted as mnemosyne in the name: command form")
	}
	if listsServer("") {
		t.Error("an empty list reported a registration")
	}
}
