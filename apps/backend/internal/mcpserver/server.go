// Package mcpserver exposes the store over MCP. It is a transport: every
// handler marshals arguments, calls one store method, and marshals the result.
// No behaviour lives here that a REST handler would need to reimplement.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aiyu-ayaan/mnemosyne/internal/index"
	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

// Version is reported to clients during initialisation.
const Version = "0.1.0"

// Limits on how much a single call may return. An agent pays for every token
// of a tool result, so the defaults are small and the ceilings are firm.
const (
	defaultListLimit   = 50
	maxListLimit       = 500
	defaultSearchLimit = 10
	maxSearchLimit     = 50
)

// New builds an MCP server backed by s.
func New(s *store.Store) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "mnemosyne",
		Title:   "Mnemosyne",
		Version: Version,
	}, nil)

	register(srv, s)
	return srv
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
	Title   string   `json:"title" jsonschema:"human-readable title; on create it also seeds the memory slug"`
	Content string   `json:"content" jsonschema:"the memory itself, as Markdown"`
	Memory  string   `json:"memory,omitempty" jsonschema:"slug or id of an existing memory to update; omit to create a new one"`
	Tags    []string `json:"tags,omitempty" jsonschema:"replaces the memory's tags when supplied"`
	Links   []string `json:"links,omitempty" jsonschema:"slugs of related memories; replaces existing links when supplied"`
}

type writeMemoryOut struct {
	Project string `json:"project"`
	Memory  string `json:"memory"`
	Title   string `json:"title"`
	Created bool   `json:"created" jsonschema:"true if a new memory was created, false if an existing one was updated"`
}

type deleteMemoryIn struct {
	Project string `json:"project" jsonschema:"slug of the project the memory belongs to"`
	Memory  string `json:"memory" jsonschema:"slug or id of the memory to delete"`
}

type deleteMemoryOut struct {
	Project string `json:"project"`
	Memory  string `json:"memory"`
	Deleted bool   `json:"deleted"`
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


// --- registration ---

func register(srv *mcp.Server, s *store.Store) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_projects",
		Description: "List every project Mnemosyne holds memories for, with its memory count.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ listProjectsIn) (*mcp.CallToolResult, listProjectsOut, error) {
		projects, err := s.ListProjects()
		if err != nil {
			return nil, listProjectsOut{}, err
		}
		return nil, listProjectsOut{Projects: projects}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_memories",
		Description: "List a project's memories. Returns titles, tags, and timestamps only — " +
			"use read_memory to fetch the content of the ones that look relevant.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listMemoriesIn) (*mcp.CallToolResult, listMemoriesOut, error) {
		memories, err := s.ListMemories(in.Project, in.Tag, clamp(in.Limit, defaultListLimit, maxListLimit))
		if err != nil {
			return nil, listMemoriesOut{}, err
		}
		return nil, listMemoriesOut{Memories: memories}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_memory",
		Description: "Read one memory in full, including its metadata and Markdown body.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in readMemoryIn) (*mcp.CallToolResult, *store.Memory, error) {
		m, err := s.ReadMemory(in.Project, in.Memory)
		if err != nil {
			return nil, nil, err
		}
		return nil, m, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "write_memory",
		Description: "Create a memory, or update an existing one by passing its slug or id as 'memory'. " +
			"The project is created automatically if it does not exist. " +
			"Tags and links are only changed when supplied.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in writeMemoryIn) (*mcp.CallToolResult, writeMemoryOut, error) {
		req := store.WriteRequest{
			Project: in.Project,
			Memory:  in.Memory,
			Title:   in.Title,
			Body:    in.Content,
		}
		// A nil slice means the caller omitted the field; an empty one means
		// they asked for it to be cleared. Only the latter should overwrite.
		if in.Tags != nil {
			req.Tags = &in.Tags
		}
		if in.Links != nil {
			req.Links = &in.Links
		}

		m, created, err := s.WriteMemory(req)
		if err != nil {
			return nil, writeMemoryOut{}, err
		}
		return nil, writeMemoryOut{
			Project: m.Project,
			Memory:  m.Slug,
			Title:   m.Title,
			Created: created,
		}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "delete_memory",
		Description: "Delete a memory permanently. There is no undo and no backup yet, " +
			"so confirm with the user before calling this.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in deleteMemoryIn) (*mcp.CallToolResult, deleteMemoryOut, error) {
		if err := s.DeleteMemory(in.Project, in.Memory); err != nil {
			return nil, deleteMemoryOut{}, err
		}
		return nil, deleteMemoryOut{Project: in.Project, Memory: in.Memory, Deleted: true}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "search_memories",
		Description: "Full-text search across memories, ranked by relevance. " +
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
		Name: "recall",
		Description: "Recall memories using semantic understanding and hybrid ranking. " +
			"Supports hybrid (default), semantic, and text modes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in recallIn) (*mcp.CallToolResult, recallOut, error) {
		hits, err := s.Recall(ctx, in.Query, in.Project, clamp(in.Limit, defaultSearchLimit, maxSearchLimit), in.Mode)
		if err != nil {
			return nil, recallOut{}, err
		}
		return nil, recallOut{Results: hits}, nil
	})
}

// Serve runs the server over stdio until the client disconnects or ctx is done.
func Serve(ctx context.Context, s *store.Store) error {
	if err := New(s).Run(ctx, &mcp.StdioTransport{}); err != nil {
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
