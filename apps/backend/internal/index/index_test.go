package index_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/index"
)

// Agents query in whole sentences. Under FTS5's implicit AND a single word
// absent from every memory sinks the query, which made search and recall look
// broken for exactly the phrasing agents use.
func TestSearchMatchesNaturalLanguageQueries(t *testing.T) {
	ix, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer ix.Close()

	now := time.Now()
	if err := ix.Put(index.Record{
		Project: "p", Slug: "conventions", ID: "1", Title: "Conventions",
		Body:    "Commits follow development/Commit.md. Never push to remote.",
		Created: now, Updated: now, ModTime: now, Size: 100,
	}); err != nil {
		t.Fatalf("put: %v", err)
	}

	for _, query := range []string{
		"commit",
		"how should I write commit messages here",
		"what are the rules about pushing to remote",
	} {
		hits, err := ix.Search(query, "", 10)
		if err != nil {
			t.Fatalf("Search(%q): %v", query, err)
		}
		if len(hits) == 0 {
			t.Errorf("Search(%q) found nothing: a sentence must not require every term to match", query)
		}
	}

	// It must still discriminate, or OR would have turned search into "return
	// everything".
	hits, err := ix.Search("kubernetes helm", "", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("Search(kubernetes helm) = %d hits, want 0", len(hits))
	}
}
