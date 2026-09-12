package index_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/index"
)

func TestVectorIndexingAndNearest(t *testing.T) {
	dir := t.TempDir()
	ix, err := index.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer ix.Close()

	// Add two memories
	r1 := index.Record{
		Project: "proj",
		Slug:    "mem-1",
		ID:      "id-1",
		Title:   "Go programming",
		Tags:    []string{"golang"},
		Body:    "Writing fast concurrent code in Go.",
		Created: time.Now(),
		Updated: time.Now(),
		ModTime: time.Now(),
		Size:    100,
	}
	r2 := index.Record{
		Project: "proj",
		Slug:    "mem-2",
		ID:      "id-2",
		Title:   "Python scripting",
		Tags:    []string{"python"},
		Body:    "Data science and machine learning in Python.",
		Created: time.Now(),
		Updated: time.Now(),
		ModTime: time.Now(),
		Size:    120,
	}

	if err := ix.Put(r1); err != nil {
		t.Fatalf("put r1: %v", err)
	}
	if err := ix.Put(r2); err != nil {
		t.Fatalf("put r2: %v", err)
	}

	model := "test-model"
	// Vectors (2D for simple testing, normalised)
	v1 := []float32{1.0, 0.0}
	v2 := []float32{0.0, 1.0}

	if err := ix.PutVector("proj", "mem-1", model, "hash-1", v1); err != nil {
		t.Fatalf("put vector 1: %v", err)
	}
	if err := ix.PutVector("proj", "mem-2", model, "hash-2", v2); err != nil {
		t.Fatalf("put vector 2: %v", err)
	}

	count, err := ix.CountVectors(model)
	if err != nil || count != 2 {
		t.Fatalf("expected 2 vectors, got %d, err: %v", count, err)
	}

	hashes, err := ix.VectorHashes(model)
	if err != nil || len(hashes) != 2 {
		t.Fatalf("expected 2 hashes, got %v, err: %v", hashes, err)
	}
	if hashes["proj/mem-1"] != "hash-1" || hashes["proj/mem-2"] != "hash-2" {
		t.Errorf("unexpected hashes: %v", hashes)
	}

	// Query close to v1
	query := []float32{0.9, 0.1}
	nearest, err := ix.Nearest(query, model, "proj", 10)
	if err != nil {
		t.Fatalf("nearest: %v", err)
	}
	if len(nearest) != 2 {
		t.Fatalf("expected 2 neighbours, got %d", len(nearest))
	}
	if nearest[0].Slug != "mem-1" {
		t.Errorf("expected mem-1 closest to query, got %s", nearest[0].Slug)
	}

	// Delete mem-1 and verify cascade delete in vectors
	if err := ix.Delete("proj", "mem-1"); err != nil {
		t.Fatalf("delete mem-1: %v", err)
	}
	countAfter, err := ix.CountVectors(model)
	if err != nil || countAfter != 1 {
		t.Fatalf("expected 1 vector after delete, got %d, err: %v", countAfter, err)
	}
}
