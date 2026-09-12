package index

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

// vectorSchema sits beside the FTS5 tables in the same database. Vectors are a
// second derived view of the same files, so they belong in the same disposable
// index rather than a second store to keep in step.
const vectorSchema = `
CREATE TABLE IF NOT EXISTS vectors (
    rowid INTEGER PRIMARY KEY REFERENCES memories(rowid) ON DELETE CASCADE,
    model TEXT NOT NULL,
    dims  INTEGER NOT NULL,
    hash  TEXT NOT NULL,
    vec   BLOB NOT NULL
);
`

// Vector is one memory's embedding, with what produced it.
//
// ponytail: this is a plain BLOB scanned linearly, not sqlite-vec. sqlite-vec
// is a loadable C extension, so adopting it means cgo or shipping a
// per-platform .so/.dll/.dylib — a build-system project — to replace a dot
// product over a few thousand rows that takes single-digit milliseconds. The
// ceiling is O(n) per query with every vector read into memory; at roughly
// 100k memories that stops being free, and the upgrade path is to swap the
// body of Nearest for an ANN index while leaving this interface alone.
type Vector struct {
	Project string
	Slug    string
	Model   string
	Hash    string
	Values  []float32
}

// Neighbour is one vector search result.
type Neighbour struct {
	Project string
	Slug    string
	Score   float64
}

// PutVector stores or replaces a memory's embedding. The memory must already be
// in the index — the vector hangs off its rowid, so it is removed with it.
func (ix *Index) PutVector(project, slug, model, hash string, values []float32) error {
	var rowid int64
	err := ix.db.QueryRow(`SELECT rowid FROM memories WHERE project = ? AND slug = ?`,
		project, slug).Scan(&rowid)
	if err == sql.ErrNoRows {
		return fmt.Errorf("%s/%s is not indexed, so it cannot be embedded", project, slug)
	}
	if err != nil {
		return fmt.Errorf("look up %s/%s: %w", project, slug, err)
	}

	if _, err := ix.db.Exec(
		`INSERT INTO vectors (rowid, model, dims, hash, vec) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(rowid) DO UPDATE SET model = excluded.model, dims = excluded.dims,
		     hash = excluded.hash, vec = excluded.vec`,
		rowid, model, len(values), hash, encodeVector(values)); err != nil {
		return fmt.Errorf("store the vector for %s/%s: %w", project, slug, err)
	}
	return nil
}

// VectorHashes returns the content hash of every stored vector for a model,
// keyed "project/slug". The sync pass uses it to embed only what changed, and
// passing the model in is what makes a model change re-embed everything.
func (ix *Index) VectorHashes(model string) (map[string]string, error) {
	rows, err := ix.db.Query(
		`SELECT m.project, m.slug, v.hash FROM vectors v
		 JOIN memories m ON m.rowid = v.rowid
		 WHERE v.model = ?`, model)
	if err != nil {
		return nil, fmt.Errorf("read vector hashes: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var project, slug, hash string
		if err := rows.Scan(&project, &slug, &hash); err != nil {
			return nil, fmt.Errorf("scan vector hash: %w", err)
		}
		out[project+"/"+slug] = hash
	}
	return out, rows.Err()
}

// Nearest returns the closest memories to query by cosine similarity. An empty
// project searches everything. Vectors from another model are skipped rather
// than compared: they are not in the same space.
func (ix *Index) Nearest(query []float32, model, project string, limit int) ([]Neighbour, error) {
	if len(query) == 0 {
		return nil, fmt.Errorf("the query vector is empty")
	}
	if limit <= 0 {
		limit = 10
	}

	q := `SELECT m.project, m.slug, v.dims, v.vec FROM vectors v
	      JOIN memories m ON m.rowid = v.rowid
	      WHERE v.model = ?`
	args := []any{model}
	if project != "" {
		q += ` AND m.project = ?`
		args = append(args, project)
	}

	rows, err := ix.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("read vectors: %w", err)
	}
	defer rows.Close()

	found := []Neighbour{}
	for rows.Next() {
		var (
			project, slug string
			dims          int
			blob          []byte
		)
		if err := rows.Scan(&project, &slug, &dims, &blob); err != nil {
			return nil, fmt.Errorf("scan vector: %w", err)
		}
		if dims != len(query) {
			continue
		}
		found = append(found, Neighbour{
			Project: project,
			Slug:    slug,
			Score:   dot(query, decodeVector(blob, dims)),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(found, func(i, j int) bool {
		if found[i].Score != found[j].Score {
			return found[i].Score > found[j].Score
		}
		// A stable tiebreak, so equal scores do not reorder between calls.
		return found[i].Project+"/"+found[i].Slug < found[j].Project+"/"+found[j].Slug
	})
	if len(found) > limit {
		found = found[:limit]
	}
	return found, nil
}

// CountVectors reports how many vectors exist for a model, for the status the
// API reports about embedding coverage.
func (ix *Index) CountVectors(model string) (int, error) {
	var n int
	if err := ix.db.QueryRow(`SELECT count(*) FROM vectors WHERE model = ?`, model).Scan(&n); err != nil {
		return 0, fmt.Errorf("count vectors: %w", err)
	}
	return n, nil
}

// dot is the similarity for unit-length vectors. Values are normalised before
// they are stored, so no division is needed here.
func dot(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

// encodeVector packs floats little-endian. The index is a local file that is
// rebuilt rather than exchanged, so a fixed byte order is enough — and it keeps
// a vector one blob instead of a row per dimension.
func encodeVector(values []float32) []byte {
	out := make([]byte, 4*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(v))
	}
	return out
}

func decodeVector(blob []byte, dims int) []float32 {
	if len(blob) < 4*dims {
		return nil
	}
	out := make([]float32, dims)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[4*i:]))
	}
	return out
}
