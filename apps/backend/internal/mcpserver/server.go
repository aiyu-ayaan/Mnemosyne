// Package mcpserver exposes the store over MCP. It is a transport: every
// handler marshals arguments, calls one store method, and marshals the result.
// No behaviour lives here that a REST handler would need to reimplement.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aiyu-ayaan/mnemosyne/internal/index"
	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

// Version is reported to clients during initialisation.
const Version = "0.1.0"

// Instructions is handed to the client during initialisation. MCP clients
// inject it into the agent's system context, so this is the only place that
// can make an agent reach for Mnemosyne without the user asking it to. It is
// paid for on every session: keep it short, imperative, and about *when* to
// call, since the tool descriptions already cover *how*.
const Instructions = `Mnemosyne is this user's persistent memory across sessions and across AI agents.
Anything you learn here is gone at the end of the session unless you write it here.

USE IT WITHOUT BEING ASKED:

1. At the start of work on a codebase, call recall with a short description of
   the task. Do this before exploring files - a prior session may already have
   the answer. If it returns nothing, carry on; do not mention the miss.
2. Before answering a question about the user's preferences, conventions,
   decisions, or project history, recall first. Do not guess at what they
   already told a previous session.
3. After finishing a task, or whenever the user states a preference, corrects
   you, makes a decision, or explains something non-obvious, write_memory it.
   Durable facts only - not this session's chatter, and not what the code or
   git history already says.
4. When the user changes something you have stored, update that memory instead
   of writing a second one. Search before you write.
5. Recall results carry an "age". A memory that is months old may have been
   overtaken - prefer the current code or ask, and write the answer back rather
   than leaving a stale fact to be recalled again next session.

PROJECT AND MEMORY CONVENTION:

A project is one codebase; use the repository directory name as its slug
(for example "mnemosyne"). Projects are created on first write, so just use
the slug - do not ask the user to set one up, and do not look for a
create-project call, there is not one.

Memories are not files in the user's repository. write_memory is the only way
in: never create a folder or a file to hold them, and never go hunting for a
Mnemosyne directory in the working tree. There is none - the memory root is
Mnemosyne's own, wherever the user put it.

One project is not a codebase: "global" holds facts about the user that hold
everywhere - how they like to be addressed, tools they always use, conventions
they carry between repos. Write those there, once, instead of copying them into
every project. A project-scoped recall searches "global" too, so you get them
back without a second call.

Within a project, keep these four memories current rather than accumulating
loose notes. Create one lazily the first time you have something for it:

  todo         - the living checklist: what is done, in progress, and planned
  decisions    - choices made and, more importantly, why; one entry per choice
  conventions  - how this codebase does things: style, commits, testing, layout
  dev-log      - what actually shipped, newest first

Anything not covered by those four gets its own memory with a descriptive
slug. Tag freely, and link related memories with [[slug]] so read_backlinks
can walk between them.

PICKING A TOOL:

  recall           - the default. Semantic, so it finds the memory even when
                     the wording differs. Start here.
  search_memories  - exact words: an error string, a filename, an identifier.
  read_memory      - the full text, once recall or search points at a slug.
  list_memories    - browsing a project, or checking whether a slug exists.
  read_backlinks   - what else references this memory.
`

// Limits on how much a single call may return. An agent pays for every token
// of a tool result, so the defaults are small and the ceilings are firm.
const (
	defaultListLimit   = 50
	maxListLimit       = 500
	defaultSearchLimit = 10
	maxSearchLimit     = 50
)

// New builds an MCP server backed by s, advertising the memories that exist
// now. The advertised set only tracks later changes under Serve — see
// watchResources for why.
func New(s *store.Store) *mcp.Server {
	srv, _ := newWithResources(s)
	return srv
}

// newWithResources is New plus the URIs it advertised, which Serve needs in
// order to take them down again on a rebuild.
func newWithResources(s *store.Store) (*mcp.Server, []string) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "mnemosyne",
		Title:   "Mnemosyne",
		Version: Version,
	}, &mcp.ServerOptions{Instructions: Instructions})

	register(srv, s)
	registerPrompts(srv)
	return srv, registerResources(srv, s)
}

// --- tool arguments and results ---

type listProjectsIn struct{}

type listProjectsOut struct {
	Projects []store.Project `json:"projects"`
}

type listMemoriesIn struct {
	Project string `json:"project" jsonschema:"slug of the project to list"`
	Tag     string `json:"tag,omitempty" jsonschema:"only return memories carrying this tag"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum memories to return (default 50, max 500)"`
}

type listMemoriesOut struct {
	Memories []store.Meta `json:"memories"`
}

type readMemoryIn struct {
	Project string `json:"project" jsonschema:"slug of the project the memory belongs to"`
	Memory  string `json:"memory" jsonschema:"slug or id of the memory to read"`
}

type writeMemoryIn struct {
	Project string   `json:"project" jsonschema:"slug of the project; it is created if it does not exist"`
	Title   string   `json:"title,omitempty" jsonschema:"human-readable title; required unless 'memory' names the slug to write, in which case it defaults from the slug"`
	Content string   `json:"content" jsonschema:"the memory itself, as Markdown"`
	Memory  string   `json:"memory,omitempty" jsonschema:"slug of the memory to write; an existing one is updated, a new slug is created at that slug. Omit to let the title pick the slug."`
	Tags    []string `json:"tags,omitempty" jsonschema:"replaces the memory's tags when supplied"`
	Links   []string `json:"links,omitempty" jsonschema:"slugs of related memories; replaces existing links when supplied"`
	Mode    string   `json:"mode,omitempty" jsonschema:"how content meets what is already there: replace (default), append, or prepend. Use append to add to a running list and prepend for newest-first logs - neither needs a read_memory first"`
}

type writeMemoryOut struct {
	Project string      `json:"project"`
	Memory  string      `json:"memory"`
	Title   string      `json:"title"`
	Created bool        `json:"created" jsonschema:"true if a new memory was created, false if an existing one was updated"`
	Similar []index.Hit `json:"similar,omitempty" jsonschema:"only on create: existing memories that may already cover this ground. Read one before letting it become a duplicate - merge into it with mode 'append' and delete this new one, or leave both if they are genuinely different"`
}

type deleteMemoryIn struct {
	Project string `json:"project" jsonschema:"slug of the project the memory belongs to"`
	Memory  string `json:"memory" jsonschema:"slug or id of the memory to delete"`
}

type deleteMemoryOut struct {
	Project string `json:"project"`
	Memory  string `json:"memory"`
	Deleted bool   `json:"deleted"`
	Trashed string `json:"trashed,omitempty" jsonschema:"where the file was moved; it can be restored from here"`
}

type searchIn struct {
	Query   string `json:"query" jsonschema:"words to search for across titles, tags, and bodies"`
	Project string `json:"project,omitempty" jsonschema:"restrict the search to one project; omit to search all of them"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum results to return (default 10, max 50)"`
}

type searchOut struct {
	Results []index.Hit `json:"results"`
}

type recallIn struct {
	Query   string `json:"query" jsonschema:"semantic query or concept to recall"`
	Project string `json:"project,omitempty" jsonschema:"restrict recall to one project; omit to search across all projects"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum results to return (default 10, max 50)"`
	Mode    string `json:"mode,omitempty" jsonschema:"search mode: hybrid (default), semantic, or text"`
}

type recallOut struct {
	Results []index.Hit `json:"results"`
}

type readBacklinksIn struct {
	Project string `json:"project" jsonschema:"slug of the project the target memory belongs to"`
	Memory  string `json:"memory" jsonschema:"slug or id of the memory to find backlinks for"`
}

type readBacklinksOut struct {
	Backlinks []store.Meta `json:"backlinks"`
}

// --- registration ---

// Annotation hints. The spec's hints are advisory, but clients surface them:
// a read-only tool can be auto-approved, a destructive one prompts. Spelling
// them out is what stops delete_memory from being treated like a read.
var (
	readOnly   = &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: ptr(false)}
	mutating   = &mcp.ToolAnnotations{DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}
	destroying = &mcp.ToolAnnotations{DestructiveHint: ptr(true), IdempotentHint: true, OpenWorldHint: ptr(false)}
)

func ptr[T any](v T) *T { return &v }

func register(srv *mcp.Server, s *store.Store) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_projects",
		Title:       "List projects",
		Annotations: readOnly,
		Description: "List every project Mnemosyne holds memories for, with its memory count. " +
			"Use this to find the right project slug when you are not sure the repository " +
			"directory name is the one that was used.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ listProjectsIn) (*mcp.CallToolResult, listProjectsOut, error) {
		projects, err := s.ListProjects()
		if err != nil {
			return nil, listProjectsOut{}, err
		}
		return nil, listProjectsOut{Projects: projects}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_memories",
		Title:       "List memories",
		Annotations: readOnly,
		Description: "List a project's memories. Returns titles, tags, and timestamps only - " +
			"use read_memory to fetch the content of the ones that look relevant. " +
			"Good for browsing a project or checking whether a slug such as 'todo' " +
			"already exists before writing to it; use recall when you are looking " +
			"for something by meaning.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listMemoriesIn) (*mcp.CallToolResult, listMemoriesOut, error) {
		memories, err := s.ListMemories(in.Project, in.Tag, clamp(in.Limit, defaultListLimit, maxListLimit))
		if err != nil {
			return nil, listMemoriesOut{}, explain(s, in.Project, err)
		}
		return nil, listMemoriesOut{Memories: memories}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_memory",
		Title:       "Read memory",
		Annotations: readOnly,
		Description: "Read one memory in full, including its metadata and Markdown body. " +
			"Call this once recall or search_memories has pointed you at a slug, and " +
			"always before updating a memory so you extend it instead of overwriting it.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in readMemoryIn) (*mcp.CallToolResult, *store.Memory, error) {
		m, err := s.ReadMemory(in.Project, in.Memory)
		if err != nil {
			return nil, nil, explain(s, in.Project, err)
		}
		return nil, m, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "write_memory",
		Title:       "Write memory",
		Annotations: mutating,
		Description: "Store something worth remembering after this session ends: a decision and " +
			"its reasoning, a user preference, a convention, a correction, a gotcha. " +
			"Pass a slug as 'memory' to write at a known slug - 'todo', 'decisions' - which " +
			"updates it if it exists and creates it if it does not. Omit 'memory' and the " +
			"title picks the slug. Search first and update rather than writing a near-duplicate. " +
			"Content replaces the whole body by default; pass mode 'append' or 'prepend' to add " +
			"to an existing memory without reading and resending the rest of it. " +
			"The project is created automatically if it does not exist, so pass the repository " +
			"directory name as the slug. Tags and links are only changed when supplied. " +
			"Do not store what the code or git history already says.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in writeMemoryIn) (*mcp.CallToolResult, writeMemoryOut, error) {
		req := store.WriteRequest{
			Project: in.Project,
			Memory:  in.Memory,
			Title:   in.Title,
			Body:    in.Content,
			Mode:    in.Mode,
		}
		// A nil slice means the caller omitted the field; an empty one means
		// they asked for it to be cleared. Only the latter should overwrite.
		if in.Tags != nil {
			req.Tags = &in.Tags
		}
		if in.Links != nil {
			req.Links = &in.Links
		}

		// Looked up before the write, not after: a memory that exists matches
		// its own title, and one exact hit is enough to stop the search falling
		// back to OR — so afterwards the only thing this finds is itself.
		//
		// Only when the caller let the title pick the slug. Writing to a slug
		// they named is a deliberate choice about where something goes, not a
		// guess worth second-guessing.
		var similar []index.Hit
		if in.Memory == "" {
			similar = similarTo(ctx, s, in.Project, in.Title)
		}

		m, created, err := s.WriteMemory(req)
		if err != nil {
			return nil, writeMemoryOut{}, err
		}
		out := writeMemoryOut{
			Project: m.Project,
			Memory:  m.Slug,
			Title:   m.Title,
			Created: created,
		}
		if created {
			out.Similar = similar
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_memory",
		Title:       "Delete memory",
		Annotations: destroying,
		Description: "Remove a memory from its project. The file is moved to the trash rather " +
			"than erased, and the result says where, so the user can restore it - but it " +
			"stops being recalled immediately, so confirm with the user first. To correct a " +
			"memory that has gone stale, overwrite it with write_memory instead of deleting it.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in deleteMemoryIn) (*mcp.CallToolResult, deleteMemoryOut, error) {
		trashed, err := s.DeleteMemory(in.Project, in.Memory)
		if err != nil {
			return nil, deleteMemoryOut{}, explain(s, in.Project, err)
		}
		return nil, deleteMemoryOut{Project: in.Project, Memory: in.Memory, Deleted: true, Trashed: trashed}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_memories",
		Title:       "Search memories (exact words)",
		Annotations: readOnly,
		Description: "Full-text keyword search across memories, ranked by relevance. Use this " +
			"when you know the exact string - an error message, a filename, an " +
			"identifier - and recall when you only know the meaning. " +
			"Returns a snippet of each match rather than the whole memory; " +
			"follow up with read_memory for the ones worth reading in full.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in searchIn) (*mcp.CallToolResult, searchOut, error) {
		hits, err := s.Search(in.Query, in.Project, clamp(in.Limit, defaultSearchLimit, maxSearchLimit))
		if err != nil {
			return nil, searchOut{}, err
		}
		return nil, searchOut{Results: hits}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "recall",
		Title:       "Recall memories",
		Annotations: readOnly,
		Description: "The default way to look something up: semantic plus keyword ranking, so it " +
			"finds the memory even when the wording differs. Call this at the start of a " +
			"task, and before answering anything about the user's preferences, past " +
			"decisions, or project history - do not guess at what a previous session was " +
			"told. Omit 'project' to search every project. Modes: hybrid (default), " +
			"semantic, text. Returning nothing is normal; carry on without comment.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in recallIn) (*mcp.CallToolResult, recallOut, error) {
		hits, err := s.Recall(ctx, in.Query, in.Project, clamp(in.Limit, defaultSearchLimit, maxSearchLimit), in.Mode)
		if err != nil {
			return nil, recallOut{}, err
		}
		return nil, recallOut{Results: hits}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_backlinks",
		Title:       "Read backlinks",
		Annotations: readOnly,
		Description: "Find all memories that link to a specified memory via [[wikilinks]] or " +
			"frontmatter links. Use it to walk outward from a memory you have already " +
			"read into the surrounding context.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in readBacklinksIn) (*mcp.CallToolResult, readBacklinksOut, error) {
		metas, err := s.Backlinks(in.Project, in.Memory)
		if err != nil {
			return nil, readBacklinksOut{}, explain(s, in.Project, err)
		}
		return nil, readBacklinksOut{Backlinks: metas}, nil
	})
}

// Serve runs the server over stdio until the client disconnects or ctx is done.
func Serve(ctx context.Context, s *store.Store) error {
	srv, advertised := newWithResources(s)

	// Scoped to this call, so the watcher cannot outlive the server it updates.
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	go watchResources(ctx, srv, s, advertised)

	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}
	return nil
}

// clamp applies the default for an unset limit and the ceiling for an
// over-large one, rather than rejecting the call over a number the agent
// guessed at.
func clamp(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

// howManySimilar is how many near-neighbours a create reports. Enough to spot
// the duplicate that already exists, few enough that the agent reads them.
const howManySimilar = 3

// similarTo finds memories in project that may already cover title. It is
// advisory and never blocks the write: a false positive costs the agent one
// read, while a false negative costs the user a library of near-duplicates
// nobody notices until recall starts returning three of them.
//
// Errors are swallowed on purpose. A project that does not exist yet, or a
// search index that failed to open, means there is nothing to duplicate — and
// neither is a reason to fail the write the caller actually asked for.
func similarTo(ctx context.Context, s *store.Store, project, title string) []index.Hit {
	if strings.TrimSpace(title) == "" {
		return nil
	}
	hits, err := s.Recall(ctx, title, project, howManySimilar, store.ModeHybrid)
	if err != nil || len(hits) == 0 {
		return nil
	}
	return hits
}

// explain turns a bare "not found" into the call the agent should make next.
//
// An agent that asks for a project which does not exist has usually guessed the
// slug or assumed projects must be created before they can be used. Both are
// recoverable in one more call, but only if the error says so — otherwise the
// agent falls back to the filesystem and starts inventing folders, which is
// exactly what Mnemosyne is there to avoid.
func explain(s *store.Store, project string, err error) error {
	if err == nil || project == "" || !errors.Is(err, store.ErrNotFound) {
		return err
	}
	// The project exists and it was the memory that was missing. Naming a slug
	// the agent can see in list_memories would only mislead.
	if _, gerr := s.GetProject(project); gerr == nil {
		return err
	}

	known, lerr := s.ListProjects()
	if lerr != nil || len(known) == 0 {
		return fmt.Errorf("%w — no projects exist yet. A project is created by its first write: "+
			"call write_memory with project %q and it will exist", err, project)
	}
	slugs := make([]string, 0, len(known))
	for _, p := range known {
		slugs = append(slugs, p.Slug)
	}
	return fmt.Errorf("%w — a project is created by its first write, not by a separate call: "+
		"call write_memory with project %q and it will exist. Existing projects: %s",
		err, project, strings.Join(slugs, ", "))
}

// ToolNames is every tool this server advertises.
//
// It exists for the CLI, which has to recognise a tool name typed as though it
// were a command and say where it actually lives. A test asserts this matches
// what the server really registers, so the list cannot drift.
func ToolNames() []string {
	return []string{
		"list_projects",
		"list_memories",
		"read_memory",
		"write_memory",
		"delete_memory",
		"search_memories",
		"recall",
		"read_backlinks",
	}
}
