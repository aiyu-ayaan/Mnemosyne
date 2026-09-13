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
