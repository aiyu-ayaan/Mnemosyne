package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aiyu-ayaan/mnemosyne/internal/events"
	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

// Memories are exposed as MCP resources as well as through the tools, because
// the two answer different questions. A tool is the agent deciding to go
// looking; a resource is the *user* pointing at something — "@decisions" in the
// client's own picker — before the agent has decided anything.
//
// The URI is mnemosyne://<project>/<slug>: stable across edits, since a rename
// writes a new slug, and readable enough to recognise in a picker.
const resourceScheme = "mnemosyne"

// maxResources caps how many memories are advertised.
//
// ponytail: a flat cap, oldest-listed wins, because resources/list is sent
// whole and a large store would otherwise spend a client's context on a
// directory listing. If this ever bites, the fix is a resource template plus
// completion rather than a bigger number.
const maxResources = 500

// memoryURI builds the resource URI for one memory. Both segments are escaped,
// though valid slugs never need it — a slug is validated on write, and a URI
// built from unescaped input is a habit worth not having.
func memoryURI(project, slug string) string {
	return fmt.Sprintf("%s://%s/%s", resourceScheme, url.PathEscape(project), url.PathEscape(slug))
}

// parseMemoryURI is memoryURI's inverse.
func parseMemoryURI(uri string) (project, slug string, err error) {
	rest, ok := strings.CutPrefix(uri, resourceScheme+"://")
	if !ok {
		return "", "", fmt.Errorf("not a mnemosyne resource: %q", uri)
	}
	rawProject, rawSlug, ok := strings.Cut(rest, "/")
	if !ok {
		return "", "", fmt.Errorf("resource URI is missing a memory: %q", uri)
	}
	if project, err = url.PathUnescape(rawProject); err != nil {
		return "", "", fmt.Errorf("bad project in %q: %w", uri, err)
	}
	if slug, err = url.PathUnescape(rawSlug); err != nil {
		return "", "", fmt.Errorf("bad memory in %q: %w", uri, err)
	}
	return project, slug, nil
}

// resourceFor describes one memory to a client. Tags go in the description
// because that is the only line a picker shows.
func resourceFor(m store.Meta) *mcp.Resource {
	description := m.Title
	if len(m.Tags) > 0 {
		description += " — " + strings.Join(m.Tags, ", ")
	}
	return &mcp.Resource{
		URI:         memoryURI(m.Project, m.Slug),
		Name:        m.Project + "/" + m.Slug,
		Title:       m.Title,
		Description: description,
		MIMEType:    "text/markdown",
	}
}

// readMemoryResource serves any mnemosyne:// URI from the store. One handler
// for every resource: the URI already says which memory, so a closure per
// memory would only be a way to serve a stale body after an edit.
func readMemoryResource(s *store.Store) mcp.ResourceHandler {
	return func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		project, slug, err := parseMemoryURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		m, err := s.ReadMemory(project, slug)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "text/markdown",
				Text:     m.Body,
			}},
		}, nil
	}
}

// registerResources advertises every memory currently on disk and returns the
// URIs it added. The SDK has no way to ask a server what it is advertising, so
// the caller keeps the list in order to be able to take it back down.
func registerResources(srv *mcp.Server, s *store.Store) []string {
	handler := readMemoryResource(s)

	projects, err := s.ListProjects()
	if err != nil {
		slog.Warn("could not list projects for resources", "err", err)
		return nil
	}

	added := make([]string, 0, len(projects))
	for _, p := range projects {
		metas, err := s.ListMemories(p.Slug, "", maxResources)
		if err != nil {
			slog.Warn("could not list memories for resources", "project", p.Slug, "err", err)
			continue
		}
		for _, m := range metas {
			if len(added) == maxResources {
				slog.Warn("resource list truncated", "max", maxResources)
				return added
			}
			r := resourceFor(m)
			srv.AddResource(r, handler)
			added = append(added, r.URI)
		}
	}
	return added
}

// watchResources keeps the advertised set in step with the store until ctx is
// done. The SDK sends notifications/resources/list_changed on each add and
// remove, so a memory an agent writes shows up in the user's picker without a
// reconnect.
//
// It runs only under Serve: New gives a correct snapshot, and a goroutine that
// outlives its server is worse than a list that does not move.
func watchResources(ctx context.Context, srv *mcp.Server, s *store.Store, known []string) {
	ch, cancel := s.Events().Subscribe()
	defer cancel()

	handler := readMemoryResource(s)
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			switch e.Kind {
			case events.MemoryWritten:
				// A write may be a create or an update. Adding an existing URI
				// again is how the SDK replaces it, so both cases are one call.
				m, err := s.ReadMemory(e.Project, e.Memory)
				if err != nil {
					continue
				}
				r := resourceFor(m.Meta)
				srv.AddResource(r, handler)
				known = append(known, r.URI)
			case events.MemoryDeleted:
				srv.RemoveResources(memoryURI(e.Project, e.Memory))
			case events.ProjectDeleted, events.IndexReconciled, events.SettingsChanged:
				// These move more than one memory at a time — a reconcile after
				// an external edit, a root change that swaps the whole store —
				// so the cheap thing is to rebuild rather than to diff.
				srv.RemoveResources(known...)
				known = registerResources(srv, s)
			}
		}
	}
}
