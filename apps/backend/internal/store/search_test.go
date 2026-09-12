package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchFindsBodyTitleAndTags(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	write(t, s, "p", "Use FTS5, not vectors", "sqlite-vec is a loadable extension needing cgo.\n", "decision")
	write(t, s, "p", "Storage is file first", "Markdown on disk is the source of truth.\n", "architecture")

	cases := map[string]string{
		"cgo":          "use-fts5-not-vectors",  // body
		"vectors":      "use-fts5-not-vectors",  // title
		"architecture": "storage-is-file-first", // tag
		"markdown":     "storage-is-file-first",
	}
	for query, wantSlug := range cases {
		hits, err := s.Search(query, "", 10)
		if err != nil {
			t.Errorf("Search(%q): %v", query, err)
			continue
		}
		if len(hits) == 0 {
			t.Errorf("Search(%q) returned nothing", query)
			continue
		}
		if hits[0].Slug != wantSlug {
			t.Errorf("Search(%q) top hit = %q, want %q", query, hits[0].Slug, wantSlug)
		}
		if hits[0].Snippet == "" {
			t.Errorf("Search(%q) returned an empty snippet", query)
		}
	}
}

func TestSearchScopesToProject(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	write(t, s, "alpha", "Shared word", "pineapple lives here.\n")
	write(t, s, "beta", "Shared word", "pineapple lives here too.\n")

	all, err := s.Search("pineapple", "", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("cross-project search returned %d hits, want 2", len(all))
	}

	scoped, err := s.Search("pineapple", "alpha", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(scoped) != 1 || scoped[0].Project != "alpha" {
		t.Errorf("scoped search = %+v, want one hit in alpha", scoped)
	}
}

func TestSearchRespectsLimit(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	for _, title := range []string{"One", "Two", "Three", "Four"} {
		write(t, s, "p", title, "shared term everywhere\n")
	}

	hits, err := s.Search("shared", "", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("len(hits) = %d, want 2", len(hits))
	}
}

func TestSearchSurvivesInvalidFTSSyntax(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	write(t, s, "p", "Operators", "the C++ build uses a toolchain\n")

	// These are all syntax errors as raw FTS5 queries; the caller should still
	// get results rather than an error about a query language they never saw.
	for _, query := range []string{`C++`, `"unbalanced`, `toolchain)`, `AND`} {
		if _, err := s.Search(query, "", 10); err != nil {
			t.Errorf("Search(%q) errored: %v", query, err)
		}
	}

	// Punctuation alone has nothing to search for. That is an error, not zero
	// results, so the caller does not read it as "no memory matches".
	for _, query := range []string{`*`, `()`, `"""`} {
		if _, err := s.Search(query, "", 10); err == nil {
			t.Errorf("Search(%q) succeeded; a query with no terms must say so", query)
		}
	}
}

func TestSearchRejectsEmptyQuery(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	if _, err := s.Search("   ", "", 10); err == nil {
		t.Fatal("Search succeeded on an empty query")
	}
}

func TestUpdateReplacesOldTextInIndex(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	m := write(t, s, "p", "Changing", "the original word is aardvark\n")
	if hits, _ := s.Search("aardvark", "", 10); len(hits) != 1 {
		t.Fatalf("setup: aardvark hits = %d, want 1", len(hits))
	}

	if _, _, err := s.WriteMemory(WriteRequest{Project: "p", Memory: m.Slug, Body: "now it says buffalo\n"}); err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}

	if hits, _ := s.Search("aardvark", "", 10); len(hits) != 0 {
		t.Errorf("stale text still searchable after update: %+v", hits)
	}
	if hits, _ := s.Search("buffalo", "", 10); len(hits) != 1 {
		t.Errorf("new text not searchable after update: %+v", hits)
	}
}

func TestDeleteRemovesFromIndex(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	m := write(t, s, "p", "Doomed", "unrepeatable-token\n")
	if _, err := s.DeleteMemory("p", m.Slug); err != nil {
		t.Fatalf("DeleteMemory: %v", err)
	}
	if hits, _ := s.Search("unrepeatable-token", "", 10); len(hits) != 0 {
		t.Errorf("deleted memory still in index: %+v", hits)
	}
}

func TestDeleteProjectRemovesFromIndex(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	write(t, s, "doomed", "One", "unrepeatable-token\n")
	if err := s.DeleteProject("doomed"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if hits, _ := s.Search("unrepeatable-token", "", 10); len(hits) != 0 {
		t.Errorf("deleted project still in index: %+v", hits)
	}
}

func TestReconcilePicksUpExternalChanges(t *testing.T) {
	root := t.TempDir()

	s, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	write(t, s, "p", "Known", "original content\n")
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Simulate an agent, an editor, or a git pull changing the files while
	// Mnemosyne was not running: one added, one edited, one removed.
	dir := filepath.Join(root, "p")
	if err := os.WriteFile(filepath.Join(dir, "known.md"),
		[]byte("---\ntitle: Known\n---\n\nedited externally with zebra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "added.md"),
		[]byte("---\ntitle: Added\n---\n\nappeared with walrus\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	if hits, _ := reopened.Search("zebra", "", 10); len(hits) != 1 {
		t.Errorf("external edit not reindexed: %+v", hits)
	}
	if hits, _ := reopened.Search("walrus", "", 10); len(hits) != 1 {
		t.Errorf("externally added file not indexed: %+v", hits)
	}
	if hits, _ := reopened.Search("original", "", 10); len(hits) != 0 {
		t.Errorf("replaced content still searchable: %+v", hits)
	}
}

func TestReconcileDropsDeletedFiles(t *testing.T) {
	root := t.TempDir()

	s, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	write(t, s, "p", "Doomed", "unrepeatable-token\n")
	s.Close()

	if err := os.Remove(filepath.Join(root, "p", "doomed.md")); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	if hits, _ := reopened.Search("unrepeatable-token", "", 10); len(hits) != 0 {
		t.Errorf("externally deleted file still in index: %+v", hits)
	}
}

func TestReconcileSkipsUnchangedFiles(t *testing.T) {
	root := t.TempDir()

	s, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	write(t, s, "p", "One", "a\n")
	write(t, s, "p", "Two", "b\n")
	s.Close()

	reopened, err := Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	changed, err := reopened.Reconcile()
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if changed != 0 {
		t.Errorf("Reconcile touched %d memories on an unchanged root, want 0", changed)
	}
}

func TestIndexRebuildsFromScratch(t *testing.T) {
	root := t.TempDir()

	s, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	write(t, s, "p", "Survivor", "durable-token lives on\n")
	s.Close()

	// The index is derived state: deleting it must cost a rebuild and nothing more.
	if err := os.Remove(filepath.Join(root, InternalDir, IndexFile)); err != nil {
		t.Fatal(err)
	}

	rebuilt, err := Open(root)
	if err != nil {
		t.Fatalf("reopen after deleting the index: %v", err)
	}
	defer rebuilt.Close()

	hits, err := rebuilt.Search("durable-token", "", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("index did not rebuild from the files: %+v", hits)
	}
}
