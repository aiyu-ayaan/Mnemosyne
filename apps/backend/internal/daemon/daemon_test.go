package daemon_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/channel"
	"github.com/aiyu-ayaan/mnemosyne/internal/config"
	"github.com/aiyu-ayaan/mnemosyne/internal/daemon"
	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

// The channel is the one piece with a different implementation per platform, so
// it is exercised for real — a named pipe on Windows, a unix socket elsewhere —
// rather than through an in-memory listener that would test neither.
func TestDaemonServesOverTheLocalChannel(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "memories"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	loc := config.Locations{
		Portable:    true, // a per-test pipe name, so parallel runs cannot collide
		BinDir:      dir,
		ConfigPath:  filepath.Join(dir, "config.json"),
		DefaultRoot: filepath.Join(dir, "memories"),
		RuntimeDir:  filepath.Join(dir, "run"),
	}
	addr, err := channel.Resolve(loc.RuntimeDir, loc.Portable)
	if err != nil {
		t.Fatalf("resolve channel: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- daemon.Run(ctx, daemon.Options{Store: s, Locations: loc, Poll: -1}) }()

	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("daemon exited with %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("daemon did not shut down within 10s")
		}
	})

	token := waitForToken(t, addr)
	client := addr.HTTPClient()

	req, err := http.NewRequest("GET", channel.BaseURL+"/v1/health", nil)
	if err != nil {
		t.Fatalf("build health request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("health over the channel: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", res.StatusCode)
	}

	var health struct {
		Root    string `json:"root"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if health.Root != s.Root() {
		t.Fatalf("root = %q, want %q", health.Root, s.Root())
	}
	if health.Version == "" {
		t.Fatal("health reported no version")
	}
}

func TestDaemonShutdownOverLocalChannel(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "memories"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	loc := config.Locations{
		Portable:    true,
		BinDir:      dir,
		ConfigPath:  filepath.Join(dir, "config.json"),
		DefaultRoot: filepath.Join(dir, "memories"),
		RuntimeDir:  filepath.Join(dir, "run"),
	}
	addr, err := channel.Resolve(loc.RuntimeDir, loc.Portable)
	if err != nil {
		t.Fatalf("resolve channel: %v", err)
	}

	ctx := context.Background()
	done := make(chan error, 1)
	go func() { done <- daemon.Run(ctx, daemon.Options{Store: s, Locations: loc, Poll: -1}) }()

	token := waitForToken(t, addr)
	client := addr.HTTPClient()

	req, err := http.NewRequest("POST", channel.BaseURL+"/v1/shutdown", nil)
	if err != nil {
		t.Fatalf("build shutdown request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("shutdown over the channel: %v", err)
	}
	res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("shutdown status = %d, want 200", res.StatusCode)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("daemon exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not exit within 5s after /v1/shutdown")
	}
}

// waitForToken polls for the token file, which is what tells a client the
// daemon has finished starting. There is no readiness signal on the channel
// itself, and a fixed sleep would be either slow or flaky.
func waitForToken(t *testing.T, addr channel.Address) string {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if token, err := addr.ReadToken(); err == nil && token != "" {
			return token
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("no token at %s within 10s — the daemon did not start", addr.TokenPath())
	return ""
}
