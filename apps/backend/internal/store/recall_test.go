package store_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

type mockEmbedder struct {
	model string
}

func (m *mockEmbedder) Model() string { return m.model }
func (m *mockEmbedder) Available(ctx context.Context) error { return nil }
func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		lower := strings.ToLower(t)
		// 3-dimensional mock vector: [ai/machine learning, database/storage, frontend/ui]
		v := []float32{0.01, 0.01, 0.01}
		if strings.Contains(lower, "ai") || strings.Contains(lower, "machine learning") || strings.Contains(lower, "neural") {
			v[0] = 1.0
		}
		if strings.Contains(lower, "sql") || strings.Contains(lower, "database") || strings.Contains(lower, "storage") {
			v[1] = 1.0
		}
		if strings.Contains(lower, "react") || strings.Contains(lower, "frontend") || strings.Contains(lower, "ui") {
			v[2] = 1.0
		}
		out[i] = v
	}
	return out, nil
}

func TestStoreRecallAndReconcileVectors(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "memories")
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	embedder := &mockEmbedder{model: "mock-model"}
	s.SetEmbedder(embedder)

	// Write three distinct memories
	_, _, err = s.WriteMemory(store.WriteRequest{
		Project: "tech",
		Title:   "Neural Networks",
		Body:    "Deep learning architectures and artificial intelligence models.",
	})
	if err != nil {
		t.Fatalf("write mem 1: %v", err)
	}

	_, _, err = s.WriteMemory(store.WriteRequest{
		Project: "tech",
		Title:   "SQLite Database",
		Body:    "Lightweight embedded SQL storage engine.",
	})
	if err != nil {
		t.Fatalf("write mem 2: %v", err)
	}

	_, _, err = s.WriteMemory(store.WriteRequest{
		Project: "tech",
		Title:   "React Framework",
		Body:    "Building interactive frontend user interfaces.",
	})
	if err != nil {
		t.Fatalf("write mem 3: %v", err)
	}

	ctx := context.Background()

	// Semantic search for AI
	aiHits, err := s.Recall(ctx, "machine learning algorithms", "tech", 5, store.ModeSemantic)
	if err != nil {
		t.Fatalf("semantic recall: %v", err)
	}
	if len(aiHits) == 0 {
		t.Fatalf("expected semantic hits, got none")
	}
	if aiHits[0].Title != "Neural Networks" {
		t.Errorf("expected Neural Networks top hit, got %s", aiHits[0].Title)
	}

	// Hybrid search
	hybridHits, err := s.Recall(ctx, "SQL storage", "tech", 5, store.ModeHybrid)
	if err != nil {
		t.Fatalf("hybrid recall: %v", err)
	}
	if len(hybridHits) == 0 {
		t.Fatalf("expected hybrid hits, got none")
	}
	if hybridHits[0].Title != "SQLite Database" {
		t.Errorf("expected SQLite Database top hit, got %s", hybridHits[0].Title)
	}

	// Reconcile vectors test
	changed, err := s.Reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	_ = changed
}
