package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/embed"
	"github.com/aiyu-ayaan/mnemosyne/internal/index"
	"github.com/aiyu-ayaan/mnemosyne/internal/markdown"
)

// SearchMode identifies the ranking strategy.
const (
	ModeHybrid   = "hybrid"
	ModeSemantic = "semantic"
	ModeText     = "text"
)

// memoryContentHash computes a deterministic digest of a memory's searchable content.
func memoryContentHash(title string, tags []string, body string) string {
	h := sha256.New()
	h.Write([]byte(title))
	h.Write([]byte{'\n'})
	for _, t := range tags {
		h.Write([]byte(t))
		h.Write([]byte{','})
	}
	h.Write([]byte{'\n'})
	h.Write([]byte(body))
	return hex.EncodeToString(h.Sum(nil))
}

func embeddingText(title string, tags []string, body string) string {
	var b strings.Builder
	b.WriteString(title)
	if len(tags) > 0 {
		b.WriteString("\nTags: ")
		b.WriteString(strings.Join(tags, ", "))
	}
	b.WriteString("\n\n")
	b.WriteString(body)
	return b.String()
}

// embedMemory embeds a single memory and saves its vector to the index.
func (s *Store) embedMemory(project, slug, title string, tags []string, body string) {
	if s.embed == nil || s.index == nil {
		return
	}
	hash := memoryContentHash(title, tags, body)
	text := embeddingText(title, tags, body)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	vecs, err := s.embed.Embed(ctx, []string{text})
	if err != nil {
		slog.Warn("could not embed memory", "project", project, "memory", slug, "err", err)
		return
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return
	}
	vec := embed.Normalise(vecs[0])
	if err := s.index.PutVector(project, slug, s.embed.Model(), hash, vec); err != nil {
		slog.Warn("could not store vector for memory", "project", project, "memory", slug, "err", err)
	}
}

// reconcileVectors ensures all indexed memories have up-to-date embeddings.
func (s *Store) reconcileVectors() error {
	if s.embed == nil || s.index == nil {
		return nil
	}
	model := s.embed.Model()
	hashes, err := s.index.VectorHashes(model)
	if err != nil {
		return fmt.Errorf("read vector hashes: %w", err)
	}

	projects, err := s.ListProjects()
	if err != nil {
		return err
	}

	type item struct {
		project string
		slug    string
		hash    string
		text    string
	}
	var toEmbed []item

	for _, p := range projects {
		dir, err := s.projectDir(p.Slug)
		if err != nil {
			continue
		}
		slugs, err := s.memorySlugs(dir)
		if err != nil {
			continue
		}
		for _, slug := range slugs {
			path := filepath.Join(dir, slug+memoryExt)
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			doc, err := markdown.Parse(data)
			if err != nil {
				continue
			}
			h := memoryContentHash(doc.Title, doc.Tags, doc.Body)
			key := p.Slug + "/" + slug
			if hashes[key] == h {
				continue // already embedded and unchanged
			}
			toEmbed = append(toEmbed, item{
				project: p.Slug,
				slug:    slug,
				hash:    h,
				text:    embeddingText(doc.Title, doc.Tags, doc.Body),
			})
		}
	}

	if len(toEmbed) == 0 {
		return nil
	}

	// Process in batches
	for i := 0; i < len(toEmbed); i += embed.MaxBatch {
		end := i + embed.MaxBatch
		if end > len(toEmbed) {
			end = len(toEmbed)
		}
		batch := toEmbed[i:end]
		texts := make([]string, len(batch))
		for j, it := range batch {
			texts[j] = it.text
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		vecs, err := s.embed.Embed(ctx, texts)
		cancel()
		if err != nil {
			slog.Warn("batch embedding failed during reconcile", "err", err)
			break
		}

		for j, it := range batch {
			if j >= len(vecs) || len(vecs[j]) == 0 {
				continue
			}
			v := embed.Normalise(vecs[j])
			if err := s.index.PutVector(it.project, it.slug, model, it.hash, v); err != nil {
				slog.Warn("store vector failed", "project", it.project, "memory", it.slug, "err", err)
			}
		}
	}

	return nil
}

// Recall performs keyword (FTS5), semantic (vector), or hybrid search.
func (s *Store) Recall(ctx context.Context, query, project string, limit int, mode string) ([]index.Hit, error) {
	if s.index == nil {
		return nil, fmt.Errorf("search is unavailable: the index failed to open")
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query is empty")
	}
	if project != "" {
		if _, err := s.GetProject(project); err != nil {
			return nil, err
		}
	}
	if limit <= 0 {
		limit = 10
	}

	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ModeText:
		return s.Search(query, project, limit)

	case ModeSemantic:
		if s.embed == nil {
			return nil, fmt.Errorf("semantic search is unavailable: no embedding provider configured")
		}
		vecs, err := s.embed.Embed(ctx, []string{query})
		if err != nil {
			return nil, fmt.Errorf("embed query: %w", err)
		}
		if len(vecs) == 0 || len(vecs[0]) == 0 {
			return nil, fmt.Errorf("embedding provider returned an empty vector for query")
		}
		normQuery := embed.Normalise(vecs[0])
		neighbours, err := s.index.Nearest(normQuery, s.embed.Model(), project, limit)
		if err != nil {
			return nil, err
		}
		hits := make([]index.Hit, 0, len(neighbours))
		for _, n := range neighbours {
			h, err := s.index.GetHit(n.Project, n.Slug)
			if err != nil {
				continue
			}
			h.Score = n.Score
			hits = append(hits, h)
		}
		return hits, nil

	case ModeHybrid, "":
		// Hybrid search with Reciprocal Rank Fusion (RRF)
		if s.embed == nil {
			return s.Search(query, project, limit)
		}

		textHits, textErr := s.Search(query, project, limit*2)

		var vecHits []index.Neighbour
		vecs, err := s.embed.Embed(ctx, []string{query})
		if err == nil && len(vecs) > 0 && len(vecs[0]) > 0 {
			normQuery := embed.Normalise(vecs[0])
			vecHits, _ = s.index.Nearest(normQuery, s.embed.Model(), project, limit*2)
		}

		if textErr != nil && len(vecHits) == 0 {
			if textErr != nil {
				return nil, textErr
			}
			return nil, fmt.Errorf("search failed")
		}

		// RRF formula: score = sum(1.0 / (k + rank)), k = 60
		const k = 60.0
		rrfScores := make(map[string]float64)
		hitMap := make(map[string]index.Hit)

		for rank, h := range textHits {
			key := h.Project + "/" + h.Slug
			rrfScores[key] += 1.0 / (k + float64(rank+1))
			hitMap[key] = h
		}

		for rank, n := range vecHits {
			key := n.Project + "/" + n.Slug
			rrfScores[key] += 1.0 / (k + float64(rank+1))
			if _, exists := hitMap[key]; !exists {
				if h, err := s.index.GetHit(n.Project, n.Slug); err == nil {
					hitMap[key] = h
				}
			}
		}

		type scoredHit struct {
			key   string
			hit   index.Hit
			score float64
		}
		combined := make([]scoredHit, 0, len(rrfScores))
		for key, score := range rrfScores {
			h, ok := hitMap[key]
			if !ok {
				continue
			}
			h.Score = score
			combined = append(combined, scoredHit{key: key, hit: h, score: score})
		}

		sort.Slice(combined, func(i, j int) bool {
			if combined[i].score != combined[j].score {
				return combined[i].score > combined[j].score
			}
			return combined[i].key < combined[j].key
		})

		if len(combined) > limit {
			combined = combined[:limit]
		}

		results := make([]index.Hit, len(combined))
		for i, c := range combined {
			results[i] = c.hit
		}
		return results, nil

	default:
		return nil, fmt.Errorf("unknown search mode %q: use hybrid, semantic, or text", mode)
	}
}
