package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/api"
	"github.com/aiyu-ayaan/mnemosyne/internal/config"
	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

const token = "test-token"

func newServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()

	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "memories"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	loc := config.Locations{
		ConfigPath:  filepath.Join(dir, "config.json"),
		DefaultRoot: filepath.Join(dir, "memories"),
		RuntimeDir:  filepath.Join(dir, "run"),
	}
	// The server, not the store, is closed on cleanup: a settings change
	// replaces the store, and only the server knows which one is current.
	srv := api.New(s, loc, token)
	t.Cleanup(func() { srv.Close() })

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts, s
}

func do(t *testing.T, ts *httptest.Server, method, path, body string) *http.Response {
	t.Helper()

	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func decode[T any](t *testing.T, res *http.Response) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func TestTokenIsRequired(t *testing.T) {
	ts, _ := newServer(t)

	res, err := ts.Client().Get(ts.URL + "/v1/health")
	if err != nil {
		t.Fatalf("get health: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status without a token = %d, want 401", res.StatusCode)
	}
}

func TestMemoryLifecycle(t *testing.T) {
	ts, _ := newServer(t)

	res := do(t, ts, "POST", "/v1/projects/notes/memories",
		`{"title":"Use FTS5","body":"sqlite-vec needs cgo.","tags":["decision"],"links":null}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", res.StatusCode)
	}
	created := decode[store.Memory](t, res)
	if created.Slug != "use-fts5" {
		t.Fatalf("slug = %q, want use-fts5", created.Slug)
	}

	res = do(t, ts, "GET", "/v1/projects/notes/memories/use-fts5", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d, want 200", res.StatusCode)
	}
	if got := decode[store.Memory](t, res); !strings.Contains(got.Body, "cgo") {
		t.Fatalf("body = %q, want it to mention cgo", got.Body)
	}

	res = do(t, ts, "PUT", "/v1/projects/notes/memories/use-fts5",
		`{"title":"Use FTS5","body":"Revisit in phase 3.","tags":["decision","search"],"links":null}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want 200", res.StatusCode)
	}

	res = do(t, ts, "GET", "/v1/search?q=phase", "")
	hits := decode[struct {
		Results []struct{ Slug string } `json:"results"`
	}](t, res)
	if len(hits.Results) != 1 || hits.Results[0].Slug != "use-fts5" {
		t.Fatalf("search results = %+v, want one hit on use-fts5", hits.Results)
	}

	res = do(t, ts, "DELETE", "/v1/projects/notes/memories/use-fts5", "")
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", res.StatusCode)
	}

	res = do(t, ts, "GET", "/v1/projects/notes/memories/use-fts5", "")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("read after delete = %d, want 404", res.StatusCode)
	}
}

// A bad slug is the caller's mistake, not a server failure. The status code is
// what the GUI uses to decide between "fix your input" and "something broke".
func TestInvalidNameIsABadRequest(t *testing.T) {
	ts, _ := newServer(t)

	res := do(t, ts, "GET", "/v1/projects/Not%20A%20Slug/memories", "")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}

func TestSettingsMoveTheRoot(t *testing.T) {
	ts, _ := newServer(t)

	moved := filepath.Join(t.TempDir(), "elsewhere")
	body, err := json.Marshal(map[string]string{"root": moved})
	if err != nil {
		t.Fatalf("encode settings: %v", err)
	}

	res := do(t, ts, "PUT", "/v1/settings", string(body))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("put settings = %d, want 200", res.StatusCode)
	}

	res = do(t, ts, "GET", "/v1/health", "")
	health := decode[struct{ Root string }](t, res)
	if health.Root != moved {
		t.Fatalf("root = %q, want %q", health.Root, moved)
	}
}

// The SSE stream is the whole reason the daemon exists rather than the GUI
// polling, so it gets a test that a write actually arrives on it.
func TestEventStreamReportsAWrite(t *testing.T) {
	ts, _ := newServer(t)

	req, err := http.NewRequest("GET", ts.URL+"/v1/events", nil)
	if err != nil {
		t.Fatalf("build events request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("open event stream: %v", err)
	}
	defer res.Body.Close()

	lines := make(chan string, 8)
	go func() {
		buf := make([]byte, 512)
		for {
			n, err := res.Body.Read(buf)
			if n > 0 {
				lines <- string(buf[:n])
			}
			if err != nil {
				close(lines)
				return
			}
		}
	}()

	do(t, ts, "POST", "/v1/projects/notes/memories", `{"title":"Hello","body":"world","tags":null,"links":null}`)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case chunk, open := <-lines:
			if !open {
				t.Fatal("event stream closed before the write arrived")
			}
			if strings.Contains(chunk, "memory.written") {
				return
			}
		case <-deadline:
			t.Fatal("no memory.written event within 5s")
		}
	}
}
