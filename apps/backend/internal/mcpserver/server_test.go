package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

// connect starts the real MCP server over an in-memory transport and returns a
// connected client session. Tests then exercise the tools the way an agent
// would, rather than calling the handlers directly.
func connect(t *testing.T, seed ...func(*store.Store)) (*mcp.ClientSession, *store.Store) {
	t.Helper()
	ctx := t.Context()

	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Seeding before the server is built matters for resources: New advertises
	// what exists at that moment.
	for _, fn := range seed {
		fn(s)
	}

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

// The discoverability contract: an agent only uses Mnemosyne unprompted if the
// server tells it to, and a client only auto-approves a read if the tool says
// it is one. Both are easy to drop when editing tool definitions, so pin them.
func TestDiscoverability(t *testing.T) {
	session, _ := connect(t)

	if got := session.InitializeResult().Instructions; got != Instructions {
		t.Fatalf("server instructions not sent to the client (got %d bytes, want %d)", len(got), len(Instructions))
	}

	want := map[string]bool{ // tool -> read-only
		"list_projects": true, "list_memories": true, "read_memory": true,
		"search_memories": true, "recall": true, "read_backlinks": true,
		"write_memory": false, "delete_memory": false,
	}

	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	seen := map[string]bool{}
	for _, tool := range tools.Tools {
		readOnly, known := want[tool.Name]
		if !known {
			t.Errorf("%s: undeclared tool - add it to this test with its read-only status", tool.Name)
			continue
		}
		seen[tool.Name] = true
		if tool.Annotations == nil {
			t.Errorf("%s: no annotations", tool.Name)
			continue
		}
		if tool.Annotations.ReadOnlyHint != readOnly {
			t.Errorf("%s: ReadOnlyHint = %v, want %v", tool.Name, tool.Annotations.ReadOnlyHint, readOnly)
		}
		if tool.Title == "" {
			t.Errorf("%s: no title", tool.Name)
		}
		if len(tool.Description) < 80 {
			t.Errorf("%s: description is %d chars - too short to tell an agent when to call it", tool.Name, len(tool.Description))
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("%s: not registered", name)
		}
	}

	if d := destroying; d.DestructiveHint == nil || !*d.DestructiveHint {
		t.Error("delete_memory must be annotated destructive")
	}
}

// seedMemory is a seed function for connect: it writes one memory before the
// server is built, so resources registered at startup have something to list.
func seedMemory(project, title, body string) func(*store.Store) {
	return func(s *store.Store) {
		if _, _, err := s.WriteMemory(store.WriteRequest{Project: project, Title: title, Body: body}); err != nil {
			panic(err)
		}
	}
}

// --- resources ---

func TestResourcesListMemories(t *testing.T) {
	session, _ := connect(t, seedMemory("mnemosyne", "Use FTS5", "sqlite-vec is a C extension\n"))

	res, err := session.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(res.Resources))
	}

	r := res.Resources[0]
	if want := "mnemosyne://mnemosyne/use-fts5"; r.URI != want {
		t.Errorf("URI = %q, want %q", r.URI, want)
	}
	if r.Title != "Use FTS5" {
		t.Errorf("Title = %q, want the memory title", r.Title)
	}
	if r.MIMEType != "text/markdown" {
		t.Errorf("MIMEType = %q, want text/markdown", r.MIMEType)
	}
}

func TestResourceReadReturnsBody(t *testing.T) {
	session, _ := connect(t, seedMemory("mnemosyne", "Use FTS5", "sqlite-vec is a C extension\n"))

	res, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{
		URI: "mnemosyne://mnemosyne/use-fts5",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("got %d contents, want 1", len(res.Contents))
	}
	if got := res.Contents[0].Text; got != "sqlite-vec is a C extension\n" {
		t.Errorf("Text = %q, want the memory body", got)
	}
}

func TestResourceReadRejectsUnknownURI(t *testing.T) {
	session, _ := connect(t)

	if _, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "file:///etc/passwd"}); err == nil {
		t.Error("ReadResource on a non-mnemosyne URI: err = nil, want an error")
	}
}

func TestMemoryURIRoundTrip(t *testing.T) {
	project, slug, err := parseMemoryURI(memoryURI("my-project", "some-memory"))
	if err != nil {
		t.Fatalf("parseMemoryURI: %v", err)
	}
	if project != "my-project" || slug != "some-memory" {
		t.Errorf("round trip gave %q/%q, want my-project/some-memory", project, slug)
	}
}

// --- prompts ---

func TestPromptsAreOffered(t *testing.T) {
	session, _ := connect(t)

	res, err := session.ListPrompts(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}

	got := map[string]*mcp.Prompt{}
	for _, p := range res.Prompts {
		got[p.Name] = p
	}
	for _, want := range []string{"checkpoint", "onboard", "review-stale"} {
		p, ok := got[want]
		if !ok {
			t.Errorf("prompt %q is not offered", want)
			continue
		}
		// A prompt with no description is one the user cannot choose between.
		if len(p.Description) < 40 {
			t.Errorf("prompt %q description is too short to say when to use it: %q", want, p.Description)
		}
		if p.Title == "" {
			t.Errorf("prompt %q has no title", want)
		}
	}
}

func TestGetPromptRendersProject(t *testing.T) {
	session, _ := connect(t)

	res, err := session.GetPrompt(t.Context(), &mcp.GetPromptParams{
		Name:      "checkpoint",
		Arguments: map[string]string{"project": "mnemosyne"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(res.Messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(res.Messages))
	}

	text, ok := res.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("content is %T, want *mcp.TextContent", res.Messages[0].Content)
	}
	if !strings.Contains(text.Text, `"mnemosyne"`) {
		t.Errorf("prompt body does not name the project it was given:\n%s", text.Text)
	}
}

// Omitting the optional project must still read as an instruction rather than
// as a template with a hole in it.
func TestGetPromptWithoutProject(t *testing.T) {
	session, _ := connect(t)

	res, err := session.GetPrompt(t.Context(), &mcp.GetPromptParams{Name: "onboard"})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	text := res.Messages[0].Content.(*mcp.TextContent).Text
	if strings.Contains(text, "%!") || strings.Contains(text, `""`) {
		t.Errorf("prompt body has an unfilled hole:\n%s", text)
	}
}

// --- write modes, duplicates, trash, age ---

// Appending is the write an agent makes most often. It has to work without a
// read_memory first, or the round trip it saves is the whole point lost.
func TestWriteAppendsWithoutReading(t *testing.T) {
	session, _ := connect(t)

	var first writeMemoryOut
	call(t, session, "write_memory", map[string]any{
		"project": "p", "memory": "dev-log", "content": "shipped A\n",
	}, &first)
	if !first.Created {
		t.Error("first write to a new slug: created = false")
	}

	call(t, session, "write_memory", map[string]any{
		"project": "p", "memory": "dev-log", "content": "shipped B\n", "mode": "prepend",
	}, nil)

	var got store.Memory
	call(t, session, "read_memory", map[string]any{"project": "p", "memory": "dev-log"}, &got)
	if want := "shipped B\n\nshipped A\n"; got.Body != want {
		t.Errorf("Body = %q, want %q", got.Body, want)
	}
}

func TestWriteRejectsUnknownMode(t *testing.T) {
	session, _ := connect(t)

	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "write_memory",
		Arguments: map[string]any{"project": "p", "memory": "todo", "content": "x", "mode": "supersede"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("an unknown mode was accepted")
	}
	if msg := textOf(res); !strings.Contains(msg, "append") {
		t.Errorf("error does not say what the valid modes are: %q", msg)
	}
}

// Creating a memory next to one that already covers the same ground should tell
// the agent so, rather than quietly growing a second copy.
func TestCreateReportsSimilarMemories(t *testing.T) {
	session, _ := connect(t, seedMemory("p", "Commit message style", "use conventional commits\n"))

	var out writeMemoryOut
	call(t, session, "write_memory", map[string]any{
		"project": "p", "title": "Commit message conventions", "content": "prefix with a type\n",
	}, &out)

	if !out.Created {
		t.Fatal("created = false for a new title")
	}
	if len(out.Similar) == 0 {
		t.Fatal("Similar is empty; the near-duplicate was not reported")
	}
	for _, h := range out.Similar {
		if h.Slug == out.Memory {
			t.Errorf("Similar contains the memory just written (%q)", h.Slug)
		}
	}
}

// The search has to run before the write. Afterwards the new memory matches its
// own title exactly, and one exact hit is enough to stop the query falling back
// to OR — so a title that only loosely resembles the existing one finds nothing
// but itself, which is filtered out, leaving an empty and falsely reassuring
// answer. This is the shape that regressed.
func TestCreateReportsLooselySimilarMemories(t *testing.T) {
	session, _ := connect(t, seedMemory("p", "Dev log", "what shipped, newest first\n"))

	var out writeMemoryOut
	call(t, session, "write_memory", map[string]any{
		"project": "p", "title": "Notes about the development log", "content": "another log\n",
	}, &out)

	if len(out.Similar) == 0 {
		t.Fatal("Similar is empty; a loosely-worded near-duplicate went unreported")
	}
	if got := out.Similar[0].Slug; got != "dev-log" {
		t.Errorf("Similar[0] = %q, want dev-log", got)
	}
}

// Writing to a slug the caller named is a deliberate placement, not a guess, so
// it is not second-guessed with a duplicate report.
func TestWriteToNamedSlugSkipsSimilar(t *testing.T) {
	session, _ := connect(t, seedMemory("p", "Commit message style", "use conventional commits\n"))

	var out writeMemoryOut
	call(t, session, "write_memory", map[string]any{
		"project": "p", "memory": "conventions", "content": "prefix commits with a type\n",
	}, &out)

	if len(out.Similar) != 0 {
		t.Errorf("Similar = %v, want nothing for an explicitly named slug", out.Similar)
	}
}

func TestDeleteReportsWhereItWent(t *testing.T) {
	session, _ := connect(t, seedMemory("p", "Doomed", "body\n"))

	var out deleteMemoryOut
	call(t, session, "delete_memory", map[string]any{"project": "p", "memory": "doomed"}, &out)

	if !out.Deleted {
		t.Error("deleted = false")
	}
	if out.Trashed == "" {
		t.Error("Trashed is empty; the agent cannot tell the user where the file went")
	}
}

// Recall hits carry an age, which is what lets an agent discount a fact that
// may have been overtaken instead of repeating it confidently.
func TestRecallHitsCarryAge(t *testing.T) {
	session, _ := connect(t, seedMemory("p", "Build command", "run pnpm build\n"))

	var out recallOut
	call(t, session, "recall", map[string]any{"query": "pnpm build", "mode": "text"}, &out)

	if len(out.Results) == 0 {
		t.Fatal("no results")
	}
	if out.Results[0].Age == "" {
		t.Errorf("hit has no age: %+v", out.Results[0])
	}
	if out.Results[0].Updated == "" {
		t.Errorf("hit has no updated timestamp: %+v", out.Results[0])
	}
}

// A memory an agent writes mid-session has to show up in the user's resource
// picker without a reconnect, which is what watchResources is for.
func TestWatchResourcesTracksWrites(t *testing.T) {
	ctx := t.Context()

	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	srv, advertised := newWithResources(s)
	go watchResources(ctx, srv, s, advertised)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
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

	if _, _, err := s.WriteMemory(store.WriteRequest{
		Project: "p", Title: "Written later", Body: "after the server started\n",
	}); err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}

	// The watcher is a goroutine on the event bus, so the resource appears
	// shortly after the write rather than during it.
	want := "mnemosyne://p/written-later"
	for i := 0; ; i++ {
		res, err := session.ListResources(ctx, nil)
		if err != nil {
			t.Fatalf("ListResources: %v", err)
		}
		for _, r := range res.Resources {
			if r.URI == want {
				return
			}
		}
		if i == 100 {
			t.Fatalf("resource %q never appeared; have %+v", want, res.Resources)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
