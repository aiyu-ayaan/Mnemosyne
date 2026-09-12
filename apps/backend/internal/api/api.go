// Package api exposes the store as JSON over HTTP. It is a transport in the
// same sense as mcpserver: every handler unmarshals, calls one store method,
// and marshals the result.
//
// It is served over a named pipe or a unix socket rather than a TCP port — see
// the channel package — but it is ordinary HTTP, so the client side is
// whatever HTTP library the caller already has.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/config"
	"github.com/aiyu-ayaan/mnemosyne/internal/embed"
	"github.com/aiyu-ayaan/mnemosyne/internal/events"
	"github.com/aiyu-ayaan/mnemosyne/internal/mcpserver"
	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

// Limits mirror the MCP tool limits, for the same reason: a client that guesses
// a number gets a sane answer instead of an error.
const (
	defaultListLimit   = 50
	maxListLimit       = 500
	defaultSearchLimit = 10
	maxSearchLimit     = 50
)

// heartbeat keeps an idle SSE connection from being closed by an intermediary
// and tells the client the daemon is still alive.
const heartbeat = 25 * time.Second

// Server exposes the store over HTTP. It is safe for concurrent use.
type Server struct {
	mu    sync.RWMutex
	store *store.Store
	loc   config.Locations
	token string
	bus   *events.Bus
	mux   *http.ServeMux
}

// New wraps s. loc is where settings are read and written; token, when not
// empty, is required on every request as a bearer token.
func New(s *store.Store, loc config.Locations, token string) *Server {
	if s.Embedder() == nil {
		if cfg, err := loc.Load(); err == nil {
			if p, err := embed.New(cfg.Embed); err == nil && p != nil {
				s.SetEmbedder(p)
			}
		}
	}
	srv := &Server{store: s, loc: loc, token: token, bus: s.Events()}
	srv.routes()
	return srv
}

// Store is the current store. It may be replaced by a settings change, so
// callers should not hold the pointer across requests.
func (srv *Server) Store() *store.Store {
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	return srv.store
}

// Close releases the current store.
func (srv *Server) Close() error {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.store == nil {
		return nil
	}
	err := srv.store.Close()
	srv.store = nil
	return err
}

// ServeHTTP authenticates and dispatches.
//
// The token is defence in depth, not the security boundary: the OS permissions
// on the pipe or socket already limit access to this user. It guards against
// another process running as the same user that happens to find the path.
func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if srv.token != "" && r.Header.Get("Authorization") != "Bearer "+srv.token {
		writeError(w, http.StatusUnauthorized, "missing or incorrect token")
		return
	}
	srv.mux.ServeHTTP(w, r)
}

func (srv *Server) routes() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/health", srv.health)
	mux.HandleFunc("GET /v1/events", srv.streamEvents)

	mux.HandleFunc("GET /v1/projects", srv.listProjects)
	mux.HandleFunc("POST /v1/projects", srv.createProject)
	mux.HandleFunc("GET /v1/projects/{project}", srv.getProject)
	mux.HandleFunc("DELETE /v1/projects/{project}", srv.deleteProject)

	mux.HandleFunc("GET /v1/projects/{project}/memories", srv.listMemories)
	mux.HandleFunc("POST /v1/projects/{project}/memories", srv.writeMemory)
	mux.HandleFunc("GET /v1/projects/{project}/memories/{memory}", srv.getMemory)
	mux.HandleFunc("PUT /v1/projects/{project}/memories/{memory}", srv.writeMemory)
	mux.HandleFunc("DELETE /v1/projects/{project}/memories/{memory}", srv.deleteMemory)

	mux.HandleFunc("GET /v1/search", srv.search)
	mux.HandleFunc("GET /v1/embeddings", srv.getEmbeddings)

	mux.HandleFunc("GET /v1/settings", srv.getSettings)
	mux.HandleFunc("PUT /v1/settings", srv.putSettings)

	srv.mux = mux
}

// --- health and settings ---

type healthOut struct {
	Version        string `json:"version"`
	Root           string `json:"root"`
	Portable       bool   `json:"portable"`
	ConfigPath     string `json:"configPath"`
	Projects       int    `json:"projects"`
	Memories       int    `json:"memories"`
	IndexBytes     int64  `json:"indexBytes"`
	Embeddings     bool   `json:"embeddings"`
	EmbeddingModel string `json:"embeddingModel,omitempty"`
}

func (srv *Server) health(w http.ResponseWriter, r *http.Request) {
	s := srv.Store()
	out := healthOut{
		Version:    mcpserver.Version,
		Root:       s.Root(),
		Portable:   srv.loc.Portable,
		ConfigPath: srv.loc.ConfigPath,
	}

	if embedder := s.Embedder(); embedder != nil {
		out.Embeddings = (embedder.Available(r.Context()) == nil)
		out.EmbeddingModel = embedder.Model()
	}

	projects, err := s.ListProjects()
	if err != nil {
		fail(w, err)
		return
	}
	out.Projects = len(projects)
	for _, p := range projects {
		out.Memories += p.MemoryCount
	}
	if info, err := os.Stat(s.InternalPath(store.IndexFile)); err == nil {
		out.IndexBytes = info.Size()
	}
	writeJSON(w, http.StatusOK, out)
}

type settings struct {
	Root        string `json:"root"`
	DefaultRoot string `json:"defaultRoot,omitempty"`
	Portable    bool   `json:"portable,omitempty"`
}

func (srv *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, settings{
		Root:        srv.Store().Root(),
		DefaultRoot: srv.loc.DefaultRoot,
		Portable:    srv.loc.Portable,
	})
}

// putSettings changes the memory root: it is written to the config file and the
// store is reopened in place, so the window does not have to be restarted.
//
// The event bus is shared across the swap, which is what keeps the clients
// currently streaming /v1/events subscribed.
func (srv *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var in settings
	if err := decode(r, &in); err != nil {
		fail(w, err)
		return
	}

	root := strings.TrimSpace(in.Root)
	if root == "" {
		writeError(w, http.StatusBadRequest, "root is required")
		return
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("resolve %q: %v", root, err))
		return
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()

	if srv.store != nil && srv.store.Root() == abs {
		writeJSON(w, http.StatusOK, settings{Root: abs, DefaultRoot: srv.loc.DefaultRoot, Portable: srv.loc.Portable})
		return
	}

	// The new root is opened before anything is committed, so a bad path leaves
	// the daemon serving the root it already had.
	next, err := store.OpenWith(abs, srv.bus)
	if err != nil {
		fail(w, err)
		return
	}

	cfg, err := srv.loc.Load()
	if err != nil {
		next.Close()
		fail(w, err)
		return
	}
	cfg.Root = abs
	if err := srv.loc.Save(cfg); err != nil {
		next.Close()
		fail(w, err)
		return
	}

	previous := srv.store
	srv.store = next
	if previous != nil {
		if err := previous.Close(); err != nil {
			slog.Warn("could not close the previous store", "err", err)
		}
	}

	srv.bus.Publish(events.Event{Kind: events.SettingsChanged})
	writeJSON(w, http.StatusOK, settings{Root: abs, DefaultRoot: srv.loc.DefaultRoot, Portable: srv.loc.Portable})
}

// --- projects ---

func (srv *Server) listProjects(w http.ResponseWriter, _ *http.Request) {
	projects, err := srv.Store().ListProjects()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (srv *Server) getProject(w http.ResponseWriter, r *http.Request) {
	p, err := srv.Store().GetProject(r.PathValue("project"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (srv *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Project string `json:"project"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, err)
		return
	}
	p, err := srv.Store().EnsureProject(strings.TrimSpace(in.Project))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (srv *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	if err := srv.Store().DeleteProject(r.PathValue("project")); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- memories ---

func (srv *Server) listMemories(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	memories, err := srv.Store().ListMemories(
		r.PathValue("project"),
		q.Get("tag"),
		clamp(intParam(q.Get("limit")), defaultListLimit, maxListLimit),
	)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": memories})
}

func (srv *Server) getMemory(w http.ResponseWriter, r *http.Request) {
	m, err := srv.Store().ReadMemory(r.PathValue("project"), r.PathValue("memory"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// memoryIn is a create or update. Tags and Links are pointers so that omitting
// them means "leave them alone" and sending [] means "clear them" — the same
// distinction store.WriteRequest makes.
type memoryIn struct {
	Title string    `json:"title"`
	Body  string    `json:"body"`
	Tags  *[]string `json:"tags"`
	Links *[]string `json:"links"`
}

func (srv *Server) writeMemory(w http.ResponseWriter, r *http.Request) {
	var in memoryIn
	if err := decode(r, &in); err != nil {
		fail(w, err)
		return
	}

	m, created, err := srv.Store().WriteMemory(store.WriteRequest{
		Project: r.PathValue("project"),
		Memory:  r.PathValue("memory"), // empty on POST, which creates
		Title:   in.Title,
		Body:    in.Body,
		Tags:    in.Tags,
		Links:   in.Links,
	})
	if err != nil {
		fail(w, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, m)
}

func (srv *Server) deleteMemory(w http.ResponseWriter, r *http.Request) {
	if err := srv.Store().DeleteMemory(r.PathValue("project"), r.PathValue("memory")); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- search ---

func (srv *Server) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mode := q.Get("mode")
	hits, err := srv.Store().Recall(
		r.Context(),
		q.Get("q"),
		q.Get("project"),
		clamp(intParam(q.Get("limit")), defaultSearchLimit, maxSearchLimit),
		mode,
	)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": hits})
}

// --- embeddings ---

type embeddingsOut struct {
	Available bool   `json:"available"`
	Provider  string `json:"provider"`
	Model     string `json:"model,omitempty"`
}

func (srv *Server) getEmbeddings(w http.ResponseWriter, r *http.Request) {
	s := srv.Store()
	embedder := s.Embedder()
	if embedder == nil {
		writeJSON(w, http.StatusOK, embeddingsOut{Available: false, Provider: "none"})
		return
	}
	available := embedder.Available(r.Context()) == nil
	writeJSON(w, http.StatusOK, embeddingsOut{
		Available: available,
		Provider:  embedder.Model(),
		Model:     embedder.Model(),
	})
}

// --- events ---

// streamEvents holds the connection open and writes each change as an SSE
// message, so an edit made by an agent or an editor reaches the window without
// it polling.
func (srv *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "this server cannot stream")
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, cancel := srv.bus.Subscribe()
	defer cancel()

	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// A comment line is a no-op to the client but proves the socket.
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case e, open := <-ch:
			if !open {
				return
			}
			data, err := json.Marshal(e)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Kind, data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// --- helpers ---

func decode(r *http.Request, dst any) error {
	if r.Body == nil {
		return fmt.Errorf("a request body is required: %w", store.ErrInvalid)
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("parse request body: %v: %w", err, store.ErrInvalid)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("could not write response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// fail maps a store error onto a status code. The store's sentinels are the
// only thing inspected, so the mapping cannot drift with error wording.
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.Warn("request failed", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func intParam(v string) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0
	}
	return n
}

func clamp(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}
