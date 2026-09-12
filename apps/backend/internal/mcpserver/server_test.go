package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

// connect starts the real MCP server over an in-memory transport and returns a
// connected client session. Tests then exercise the tools the way an agent
// would, rather than calling the handlers directly.
func connect(t *testing.T) (*mcp.ClientSession, *store.Store) {
	t.Helper()
	ctx := t.Context()

	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := New(s).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session, s
}

// call invokes a tool and decodes its structured output into out.
func call(t *testing.T, session *mcp.ClientSession, name string, args map[string]any, out any) *mcp.CallToolResult {
	t.Helper()

	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s) returned a tool error: %s", name, textOf(res))
	}
	if out != nil {
		data, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("marshal %s output: %v", name, err)
		}
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatalf("decode %s output: %v", name, err)
		}
	}
	return res
}

// callExpectingError invokes a tool that should fail and returns its message.
func callExpectingError(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()

	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		// A protocol-level rejection (schema validation) is also a failure the
		// caller can act on, which is what this asserts.
		return err.Error()
	}
	if !res.IsError {
		t.Fatalf("CallTool(%s) succeeded; expected an error", name)
	}
	return textOf(res)
}

func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestToolsAreAdvertised(t *testing.T) {
	session, _ := connect(t)

	res, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("tool %q has no description; an agent chooses tools by reading these", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %q has no input schema", tool.Name)
		}
	}

	for _, want := range []string{
		"list_projects", "list_memories", "read_memory",
		"write_memory", "delete_memory", "search_memories",
		"recall", "read_backlinks",
	} {
		if !got[want] {
			t.Errorf("tool %q is not advertised", want)
		}
	}
	if len(res.Tools) != 8 {
		t.Errorf("advertised %d tools, want exactly 8: %v", len(res.Tools), got)
	}
}

func TestWriteThenReadRoundTrip(t *testing.T) {
	session, _ := connect(t)

	var written writeMemoryOut
	call(t, session, "write_memory", map[string]any{
		"project": "mnemosyne",
		"title":   "Use FTS5, not vectors",
		"content": "sqlite-vec needs cgo.\n",
		"tags":    []string{"decision", "search"},
	}, &written)

	if !written.Created {
		t.Error("created = false on first write")
	}
	if written.Memory != "use-fts5-not-vectors" {
		t.Errorf("memory slug = %q", written.Memory)
	}

	var got store.Memory
	call(t, session, "read_memory", map[string]any{
		"project": "mnemosyne",
		"memory":  written.Memory,
	}, &got)

	if got.Title != "Use FTS5, not vectors" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Body != "sqlite-vec needs cgo.\n" {
		t.Errorf("Body = %q", got.Body)
	}
	if strings.Join(got.Tags, ",") != "decision,search" {
		t.Errorf("Tags = %v", got.Tags)
	}
}

func TestWriteUpdatesExistingMemory(t *testing.T) {
	session, _ := connect(t)

	var first writeMemoryOut
	call(t, session, "write_memory", map[string]any{
		"project": "p", "title": "Original", "content": "v1\n", "tags": []string{"keep"},
	}, &first)

	var second writeMemoryOut
	call(t, session, "write_memory", map[string]any{
		"project": "p", "memory": first.Memory, "title": "Original", "content": "v2\n",
	}, &second)

	if second.Created {
		t.Error("created = true when updating")
	}

	var got store.Memory
	call(t, session, "read_memory", map[string]any{"project": "p", "memory": first.Memory}, &got)

	if got.Body != "v2\n" {
		t.Errorf("Body = %q, want the updated content", got.Body)
	}
	if strings.Join(got.Tags, ",") != "keep" {
		t.Errorf("Tags = %v; omitted tags must be left alone", got.Tags)
	}
}

func TestListProjectsAndMemories(t *testing.T) {
	session, _ := connect(t)

	call(t, session, "write_memory", map[string]any{
		"project": "alpha", "title": "One", "content": "a\n", "tags": []string{"x"},
	}, nil)
	call(t, session, "write_memory", map[string]any{
		"project": "alpha", "title": "Two", "content": "b\n", "tags": []string{"y"},
	}, nil)
	call(t, session, "write_memory", map[string]any{
		"project": "beta", "title": "Three", "content": "c\n",
	}, nil)

	var projects listProjectsOut
	call(t, session, "list_projects", map[string]any{}, &projects)
	if len(projects.Projects) != 2 {
		t.Fatalf("projects = %+v, want 2", projects.Projects)
	}
	if projects.Projects[0].Slug != "alpha" || projects.Projects[0].MemoryCount != 2 {
		t.Errorf("projects[0] = %+v", projects.Projects[0])
	}

	var listed listMemoriesOut
	call(t, session, "list_memories", map[string]any{"project": "alpha"}, &listed)
	if len(listed.Memories) != 2 {
		t.Errorf("memories = %+v, want 2", listed.Memories)
	}

	var tagged listMemoriesOut
	call(t, session, "list_memories", map[string]any{"project": "alpha", "tag": "x"}, &tagged)
	if len(tagged.Memories) != 1 || tagged.Memories[0].Title != "One" {
		t.Errorf("tag-filtered memories = %+v", tagged.Memories)
	}
}

func TestSearchMemories(t *testing.T) {
	session, _ := connect(t)

	call(t, session, "write_memory", map[string]any{
		"project": "p", "title": "Storage", "content": "markdown files are the source of truth\n",
	}, nil)
	call(t, session, "write_memory", map[string]any{
		"project": "p", "title": "Indexing", "content": "sqlite is a derived index\n",
	}, nil)

	var found searchOut
	call(t, session, "search_memories", map[string]any{"query": "derived"}, &found)

	if len(found.Results) != 1 {
		t.Fatalf("results = %+v, want 1", found.Results)
	}
	if found.Results[0].Slug != "indexing" {
		t.Errorf("top hit = %q, want indexing", found.Results[0].Slug)
	}
	if found.Results[0].Snippet == "" {
		t.Error("hit has no snippet")
	}
}

func TestRecallMemories(t *testing.T) {
	session, _ := connect(t)

	call(t, session, "write_memory", map[string]any{
		"project": "p", "title": "Memory Architecture", "content": "long term recall for agents\n",
	}, nil)

	var found recallOut
	call(t, session, "recall", map[string]any{"query": "recall", "project": "p"}, &found)

	if len(found.Results) != 1 {
		t.Fatalf("results = %+v, want 1", found.Results)
	}
	if found.Results[0].Slug != "memory-architecture" {
		t.Errorf("top hit = %q, want memory-architecture", found.Results[0].Slug)
	}
}

func TestReadBacklinks(t *testing.T) {
	session, _ := connect(t)

	// Target memory
	call(t, session, "write_memory", map[string]any{
		"project": "proj", "title": "Target Document", "content": "base document\n",
	}, nil)

	// Source memory linking via wikilink
	call(t, session, "write_memory", map[string]any{
		"project": "proj", "title": "Source Document", "content": "referencing [[target-document]] here\n",
	}, nil)

	var found readBacklinksOut
	call(t, session, "read_backlinks", map[string]any{"project": "proj", "memory": "target-document"}, &found)

	if len(found.Backlinks) != 1 {
		t.Fatalf("backlinks = %+v, want 1", found.Backlinks)
	}
	if found.Backlinks[0].Slug != "source-document" {
		t.Errorf("expected source-document backlink, got %s", found.Backlinks[0].Slug)
	}
}

func TestSearchLimitIsClamped(t *testing.T) {
	session, _ := connect(t)

	for _, title := range []string{"A", "B", "C"} {
		call(t, session, "write_memory", map[string]any{
			"project": "p", "title": title, "content": "shared token\n",
		}, nil)
	}

	// An over-large limit is clamped rather than rejected: the agent guessed a
	// number, and failing the call over it helps nobody.
	var found searchOut
	call(t, session, "search_memories", map[string]any{"query": "shared", "limit": 9999}, &found)
	if len(found.Results) != 3 {
		t.Errorf("results = %d, want all 3", len(found.Results))
	}

	var limited searchOut
	call(t, session, "search_memories", map[string]any{"query": "shared", "limit": 1}, &limited)
	if len(limited.Results) != 1 {
		t.Errorf("results = %d, want 1", len(limited.Results))
	}
}

func TestDeleteMemory(t *testing.T) {
	session, _ := connect(t)

	var written writeMemoryOut
	call(t, session, "write_memory", map[string]any{
		"project": "p", "title": "Doomed", "content": "x\n",
	}, &written)

	var deleted deleteMemoryOut
	call(t, session, "delete_memory", map[string]any{"project": "p", "memory": written.Memory}, &deleted)
	if !deleted.Deleted {
		t.Error("deleted = false")
	}

	msg := callExpectingError(t, session, "read_memory", map[string]any{"project": "p", "memory": written.Memory})
	if !strings.Contains(msg, "not found") {
		t.Errorf("error = %q, want it to say the memory is not found", msg)
	}
}

func TestErrorsAreReadableSentences(t *testing.T) {
	session, _ := connect(t)
	call(t, session, "write_memory", map[string]any{"project": "real", "title": "T", "content": "c\n"}, nil)

	cases := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"missing project", "list_memories", map[string]any{"project": "ghost"}, "not found"},
		{"missing memory", "read_memory", map[string]any{"project": "real", "memory": "ghost"}, "not found"},
		{"traversal", "read_memory", map[string]any{"project": "real", "memory": "../../etc/passwd"}, "invalid"},
		{"bad project name", "list_memories", map[string]any{"project": "../escape"}, "invalid"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := strings.ToLower(callExpectingError(t, session, tc.tool, tc.args))
			if !strings.Contains(msg, tc.want) {
				t.Errorf("error = %q, want it to mention %q", msg, tc.want)
			}
		})
	}
}

func TestWriteRejectsTraversal(t *testing.T) {
	session, _ := connect(t)

	msg := callExpectingError(t, session, "write_memory", map[string]any{
		"project": "../escape", "title": "Evil", "content": "x\n",
	})
	if !strings.Contains(strings.ToLower(msg), "invalid") {
		t.Errorf("error = %q, want a rejection of the project name", msg)
	}
}

func TestWriteRequiresTitleOnCreate(t *testing.T) {
	session, _ := connect(t)

	// The schema marks title required, so this is rejected before it reaches
	// the store. Either layer catching it is fine; silently creating an
	// untitled memory is not.
	if msg := callExpectingError(t, session, "write_memory", map[string]any{
		"project": "p", "content": "orphan\n",
	}); msg == "" {
		t.Error("write_memory without a title produced no error message")
	}
}

func TestContextCancellationIsHonoured(t *testing.T) {
	session, _ := connect(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_projects",
		Arguments: map[string]any{},
	}); err == nil {
		t.Error("CallTool with a cancelled context succeeded")
	}
}
