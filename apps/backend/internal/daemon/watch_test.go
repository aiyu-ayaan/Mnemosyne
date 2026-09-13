package daemon_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/config"
	"github.com/aiyu-ayaan/mnemosyne/internal/daemon"
	"github.com/aiyu-ayaan/mnemosyne/internal/events"
	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

// TestWatchAnnouncesAnotherProcessesWrite is the regression test for a bug that
// was invisible from inside one process: an agent's `mnemosyne serve` writes a
// memory, and the desktop app never hears about it.
//
// Every process sharing a root shares its index, so the writer indexes what it
// wrote. Reconcile decides what changed by comparing the files against that same
// index, so it then sees nothing to do and publishes nothing — and the renderer
// only reloads on events. Two stores over one root is exactly that situation.
func TestWatchAnnouncesAnotherProcessesWrite(t *testing.T) {
	root := filepath.Join(t.TempDir(), "memories")

	// The daemon's store, and the event stream the desktop app would be on.
	served, err := store.Open(root)
	if err != nil {
		t.Fatalf("open the daemon's store: %v", err)
	}
	stream, unsubscribe := served.Events().Subscribe()
	defer unsubscribe()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx, daemon.Options{
			Store:     served,
			Locations: locationsFor(t, filepath.Dir(root), root),
			Poll:      50 * time.Millisecond,
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("daemon did not shut down within 10s")
		}
	})

	// A second process on the same root: its own store, its own bus, sharing
	// one index.db — an agent, or the CLI.
	other, err := store.Open(root)
	if err != nil {
		t.Fatalf("open the other process's store: %v", err)
	}
	defer other.Close()

	if _, _, err := other.WriteMemory(store.WriteRequest{
		Project: "domus", Title: "Written elsewhere", Body: "by another process",
	}); err != nil {
		t.Fatalf("write from the other process: %v", err)
	}

	waitFor(t, stream, events.MemoryWritten, "domus", "written-elsewhere")

	// And the same for a delete, which fails the same way and for the same
	// reason: the other process drops its own index row.
	if _, err := other.DeleteMemory("domus", "written-elsewhere"); err != nil {
		t.Fatalf("delete from the other process: %v", err)
	}
	waitFor(t, stream, events.MemoryDeleted, "domus", "written-elsewhere")
}

// TestWatchDoesNotAnnounceAQuietRoot guards the other half: a root nothing is
// touching must stay silent, or the desktop app refetches the library forever.
func TestWatchDoesNotAnnounceAQuietRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "memories")

	served, err := store.Open(root)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, _, err := served.WriteMemory(store.WriteRequest{
		Project: "domus", Title: "Settled", Body: "nothing will touch this",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stream, unsubscribe := served.Events().Subscribe()
	defer unsubscribe()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx, daemon.Options{
			Store:     served,
			Locations: locationsFor(t, filepath.Dir(root), root),
			Poll:      50 * time.Millisecond,
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("daemon did not shut down within 10s")
		}
	})

	// Several ticks' worth. Anything arriving here is the watcher announcing a
	// change that did not happen.
	select {
	case ev := <-stream:
		t.Fatalf("quiet root produced %s for %s/%s", ev.Kind, ev.Project, ev.Memory)
	case <-time.After(500 * time.Millisecond):
	}
}

// locationsFor is a portable layout in a temp directory, so each test gets its
// own channel name and cannot collide with a parallel run or a real daemon.
func locationsFor(t *testing.T, dir, root string) config.Locations {
	t.Helper()
	return config.Locations{
		Portable:    true,
		BinDir:      dir,
		ConfigPath:  filepath.Join(dir, "config.json"),
		DefaultRoot: root,
		RuntimeDir:  filepath.Join(dir, "run"),
	}
}

// waitFor reads the stream until the expected event arrives, ignoring the
// reconcile and vector events that legitimately accompany it.
func waitFor(t *testing.T, stream <-chan events.Event, kind events.Kind, project, memory string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-stream:
			if ev.Kind == kind && ev.Project == project && ev.Memory == memory {
				return
			}
		case <-deadline:
			t.Fatalf("no %s event for %s/%s within 5s", kind, project, memory)
		}
	}
}
