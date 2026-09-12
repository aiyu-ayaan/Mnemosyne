package embed_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aiyu-ayaan/mnemosyne/internal/embed"
)

func TestNormaliseAndSimilarity(t *testing.T) {
	v := []float32{3, 4}
	embed.Normalise(v)
	expected := float32(0.6)
	if math.Abs(float64(v[0]-expected)) > 1e-6 {
		t.Errorf("expected %f, got %f", expected, v[0])
	}
	if math.Abs(float64(v[1]-0.8)) > 1e-6 {
		t.Errorf("expected 0.8, got %f", v[1])
	}

	simIdentical := embed.Similarity(v, v)
	if math.Abs(simIdentical-1.0) > 1e-6 {
		t.Errorf("expected 1.0 for identical vectors, got %f", simIdentical)
	}

	perpendicular := []float32{-v[1], v[0]}
	simPerp := embed.Similarity(v, perpendicular)
	if math.Abs(simPerp) > 1e-6 {
		t.Errorf("expected 0.0 for perpendicular vectors, got %f", simPerp)
	}

	// Mismatched lengths
	if s := embed.Similarity(v, []float32{1, 2, 3}); s != 0 {
		t.Errorf("expected 0 for mismatched lengths, got %f", s)
	}
}

func TestProviders(t *testing.T) {
	pNone, err := embed.New(embed.Settings{Provider: embed.ProviderNone})
	if err != nil || pNone != nil {
		t.Fatalf("expected nil provider, got %v, err: %v", pNone, err)
	}

	// Ollama provider
	tsOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		res := map[string]any{
			"embeddings": [][]float32{
				{0.1, 0.2, 0.3},
			},
		}
		json.NewEncoder(w).Encode(res)
	}))
	defer tsOllama.Close()

	pOllama, err := embed.New(embed.Settings{
		Provider: embed.ProviderOllama,
		Endpoint: tsOllama.URL,
		Model:    "test-model",
	})
	if err != nil {
		t.Fatalf("new ollama provider: %v", err)
	}
	if pOllama.Model() != "test-model" {
		t.Errorf("expected test-model, got %s", pOllama.Model())
	}
	if err := pOllama.Available(context.Background()); err != nil {
		t.Errorf("expected available, got %v", err)
	}
	vecs, err := pOllama.Embed(context.Background(), []string{"test"})
	if err != nil || len(vecs) != 1 || len(vecs[0]) != 3 {
		t.Fatalf("unexpected embed output: %v, err: %v", vecs, err)
	}

	// OpenAI provider
	tsOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		res := map[string]any{
			"data": []map[string]any{
				{"index": 1, "embedding": []float32{0.4, 0.5}},
				{"index": 0, "embedding": []float32{0.1, 0.2}},
			},
		}
		json.NewEncoder(w).Encode(res)
	}))
	defer tsOpenAI.Close()

	pOpenAI, err := embed.New(embed.Settings{
		Provider: embed.ProviderOpenAI,
		Endpoint: tsOpenAI.URL,
		Model:    "text-embedding-3-small",
	})
	if err != nil {
		t.Fatalf("new openai provider: %v", err)
	}
	vecsAI, err := pOpenAI.Embed(context.Background(), []string{"first", "second"})
	if err != nil || len(vecsAI) != 2 {
		t.Fatalf("expected 2 vectors, got %v, err: %v", vecsAI, err)
	}
	if vecsAI[0][0] != 0.1 || vecsAI[1][0] != 0.4 {
		t.Errorf("order not restored correctly from indices: %v", vecsAI)
	}
}
