// Package daemon is the long-running, per-user process behind the desktop app:
// it serves the JSON API over the local channel and keeps the index in step
// with the files.
//
// It is not privileged over the MCP server. Both open the same memory root, and
// the storage design — WAL, atomic renames, reconcile — is what makes that safe,
// rather than one process owning the data and the other proxying through it.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/api"
	"github.com/aiyu-ayaan/mnemosyne/internal/channel"
	"github.com/aiyu-ayaan/mnemosyne/internal/config"
	"github.com/aiyu-ayaan/mnemosyne/internal/events"
	"github.com/aiyu-ayaan/mnemosyne/internal/index"
	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

// DefaultPoll is how often the memory root is reconciled to catch changes
// Mnemosyne did not make: an agent's write, an editor save, a git pull.
const DefaultPoll = 3 * time.Second

// shutdownGrace is how long in-flight requests get to finish. SSE clients are
// cancelled by the context and close immediately, so this only covers ordinary
// requests.
const shutdownGrace = 3 * time.Second

// Options configures a run.
type Options struct {
	// Store is the already-open store. The daemon takes ownership and closes it.
	Store *store.Store

	// Locations is the resolved layout, used for settings and the runtime dir.
	Locations config.Locations

	// Poll overrides DefaultPoll. Zero or negative disables the watcher, which
	// is only useful in tests.
	Poll time.Duration
}

// Run serves until ctx is cancelled.
func Run(ctx context.Context, opts Options) error {
	addr, err := channel.Resolve(opts.Locations.RuntimeDir, opts.Locations.Isolated())
	if err != nil {
		return err
	}

	// A fresh token per start means a token left behind by a previous run
	// cannot be replayed against this one.
	token, err := addr.NewToken()
	if err != nil {
		return err
	}
	defer func() {
		if err := addr.RemoveToken(); err != nil {
			slog.Warn("could not remove the daemon token", "err", err)
		}
	}()

	listener, err := addr.Listen()
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	srv := api.New(opts.Store, opts.Locations, token)
	srv.SetOnShutdown(cancel)
	defer srv.Close()

	httpSrv := &http.Server{
		Handler: srv,
		// No write timeout: /v1/events holds the response open for the life of
		// the client. Read timeouts still apply, so a client that opens a
		// connection and sends nothing does not tie up a goroutine forever.
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return runCtx },
	}

	poll := opts.Poll
	if poll == 0 {
		poll = DefaultPoll
	}
	if poll > 0 {
		go watch(runCtx, srv, poll)
	}

	slog.Info("mnemosyne daemon listening",
		"endpoint", addr.Endpoint, "root", opts.Store.Root(), "token", addr.TokenPath())

	errs := make(chan error, 1)
	go func() { errs <- httpSrv.Serve(listener) }()

	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve the local channel: %w", err)
	case <-runCtx.Done():
	}

	shutdown, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := httpSrv.Shutdown(shutdown); err != nil {
		slog.Warn("the daemon did not shut down cleanly", "err", err)
	}
	return nil
}

// watch reconciles the index on a timer, which is how a change made outside
// Mnemosyne becomes an event the desktop app sees.
//
// ponytail: polling, not filesystem notifications. Reconcile is stat-based and
// already the mechanism for "a change we did not make", so a ticker reuses it
// for zero new dependencies and no debounce logic. The ceiling is one stat per
// memory per tick — if that shows up in a profile on a large root, swap in
// fsnotify here and keep Reconcile as the fallback.
//
// Reconcile alone is not enough, though, and the gap is not obvious. Every
// process sharing a root shares its index, so an agent's `mnemosyne serve`
// indexes what it writes — and Reconcile, which decides by comparing the file
// against that same index, then sees nothing to do and says nothing. The desktop
// app only reloads on events, so an agent's write would never reach an open
// window. The daemon therefore keeps its own snapshot of the index and announces
// anything that moved under it.
func watch(ctx context.Context, srv *api.Server, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	// Seeded before the first tick, not on it: the window between the daemon
	// starting and the first tick is a window in which another process can
	// write, and an empty snapshot would announce the whole root as new.
	var seen map[string]index.Stamp
	root := ""
	if s := srv.Store(); s != nil {
		seen, _ = s.Stamps()
		root = s.Root()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s := srv.Store()
			if s == nil {
				return
			}
			// The root can change under us: settings reopens the store in
			// place. Stamps from the old root describe different files, so the
			// snapshot is restarted rather than diffed across the move.
			if s.Root() != root {
				seen, _ = s.Stamps()
				root = s.Root()
				continue
			}

			// What the index said before Reconcile ran, so the changes
			// Reconcile makes and announces itself can be told apart from the
			// ones another process had already applied.
			before, err := s.Stamps()
			if err != nil {
				slog.Warn("could not read the index stamps", "err", err)
				continue
			}
			if _, err := s.Reconcile(); err != nil {
				slog.Warn("could not reconcile the memory root", "err", err)
			}
			after, err := s.Stamps()
			if err != nil {
				slog.Warn("could not read the index stamps", "err", err)
				continue
			}

			announceOtherWrites(s, seen, before, after)
			seen = after
		}
	}
}

// announceOtherWrites publishes an event for every memory that changed since the
// daemon last looked and that Reconcile did not already announce.
//
// before and after bracket this tick's Reconcile, so anything differing between
// them is Reconcile's own work and has an event already. Everything else that
// differs from seen was written by another process.
func announceOtherWrites(s *store.Store, seen, before, after map[string]index.Stamp) {
	if seen == nil {
		return
	}
	for key, now := range after {
		if was, ok := before[key]; !ok || was != now {
			continue // Reconcile did this one, and published for it.
		}
		if had, ok := seen[key]; ok && had == now {
			continue // nothing moved.
		}
		if project, slug, ok := strings.Cut(key, "/"); ok {
			s.Publish(events.MemoryWritten, project, slug)
		}
	}
	for key := range seen {
		if _, still := after[key]; still {
			continue
		}
		// Reconcile drops a row during this tick, so a row it removed was still
		// there when the tick began. One the other process removed was already
		// gone by then — which is the one nobody has announced.
		if _, whenTickBegan := before[key]; whenTickBegan {
			continue
		}
		if project, slug, ok := strings.Cut(key, "/"); ok {
			s.Publish(events.MemoryDeleted, project, slug)
		}
	}
}
