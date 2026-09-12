// Package index is the derived search index over the memory files: SQLite with
// an FTS5 table. Nothing is stored here that does not also exist on disk, so
// deleting index.db costs one rebuild and nothing else.
//
// It deliberately does not import the store package. The store owns the index,
// converts its own types into Records, and would otherwise create an import
// cycle.
package index

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver: no cgo, so cross-compiling stays trivial
)

// Record is one memory as the index sees it.
type Record struct {
	Project string
	Slug    string
	ID      string
	Title   string
	Tags    []string
	Body    string
	Created time.Time
	Updated time.Time

	// ModTime and Size are the file's stat values. Reconcile compares them to
	// decide whether a file needs reparsing, which keeps startup proportional
	// to what changed rather than to how much is stored.
	ModTime time.Time
	Size    int64
}

// Hit is one search result. Snippet is a fragment of the matched text with the
// matching terms marked, not the whole body.
type Hit struct {
	Project string   `json:"project"`
	Slug    string   `json:"slug"`
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Tags    []string `json:"tags,omitempty"`
	Snippet string   `json:"snippet"`
	Score   float64  `json:"score"`
}

// Stamp is the indexed file state used to detect changes during reconcile.
type Stamp struct {
	ModTime time.Time
	Size    int64
}

// Index is an open handle on index.db.
type Index struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS memories (
    rowid   INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL,
    slug    TEXT NOT NULL,
    id      TEXT NOT NULL DEFAULT '',
    title   TEXT NOT NULL DEFAULT '',
    tags    TEXT NOT NULL DEFAULT '',
    created TEXT NOT NULL DEFAULT '',
    updated TEXT NOT NULL DEFAULT '',
    mtime   INTEGER NOT NULL DEFAULT 0,
    size    INTEGER NOT NULL DEFAULT 0,
    UNIQUE (project, slug)
);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    title,
    tags,
    body,
    tokenize = 'porter unicode61'
);
`

// Open opens or creates the index at path.
func Open(path string) (*Index, error) {
	// WAL keeps reads from blocking the writer; busy_timeout absorbs the brief
	// contention when a write and a search overlap.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
		url.PathEscape(path))

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open index at %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create index schema: %w", err)
	}
	return &Index{db: db}, nil
}

// Close releases the database handle.
func (ix *Index) Close() error {
	if ix == nil || ix.db == nil {
		return nil
	}
	return ix.db.Close()
}

// Put inserts or replaces a memory. The FTS row is rewritten rather than
// updated in place, which is the supported way to change an fts5 row.
func (ix *Index) Put(r Record) error {
	tx, err := ix.db.Begin()
	if err != nil {
		return fmt.Errorf("begin index write: %w", err)
	}
	defer tx.Rollback()

	var rowid int64
	err = tx.QueryRow(`SELECT rowid FROM memories WHERE project = ? AND slug = ?`,
		r.Project, r.Slug).Scan(&rowid)
	switch {
	case err == sql.ErrNoRows:
		rowid = 0
	case err != nil:
		return fmt.Errorf("look up %s/%s: %w", r.Project, r.Slug, err)
	}

	tags := strings.Join(r.Tags, " ")
	if rowid == 0 {
		res, err := tx.Exec(
			`INSERT INTO memories (project, slug, id, title, tags, created, updated, mtime, size)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.Project, r.Slug, r.ID, r.Title, tags,
			format(r.Created), format(r.Updated), r.ModTime.UnixNano(), r.Size)
		if err != nil {
			return fmt.Errorf("insert %s/%s: %w", r.Project, r.Slug, err)
		}
		if rowid, err = res.LastInsertId(); err != nil {
			return fmt.Errorf("insert %s/%s: %w", r.Project, r.Slug, err)
		}
	} else {
		if _, err := tx.Exec(
			`UPDATE memories SET id = ?, title = ?, tags = ?, created = ?, updated = ?, mtime = ?, size = ?
			 WHERE rowid = ?`,
			r.ID, r.Title, tags, format(r.Created), format(r.Updated),
			r.ModTime.UnixNano(), r.Size, rowid); err != nil {
			return fmt.Errorf("update %s/%s: %w", r.Project, r.Slug, err)
		}
		if _, err := tx.Exec(`DELETE FROM memories_fts WHERE rowid = ?`, rowid); err != nil {
			return fmt.Errorf("clear search row for %s/%s: %w", r.Project, r.Slug, err)
		}
	}

	if _, err := tx.Exec(
		`INSERT INTO memories_fts (rowid, title, tags, body) VALUES (?, ?, ?, ?)`,
		rowid, r.Title, tags, r.Body); err != nil {
		return fmt.Errorf("index text for %s/%s: %w", r.Project, r.Slug, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit index write: %w", err)
	}
	return nil
}

// Delete removes a memory from the index. Removing something that is not there
// is not an error: the caller's goal is that it be absent.
func (ix *Index) Delete(project, slug string) error {
	tx, err := ix.db.Begin()
	if err != nil {
		return fmt.Errorf("begin index delete: %w", err)
	}
	defer tx.Rollback()

	var rowid int64
	err = tx.QueryRow(`SELECT rowid FROM memories WHERE project = ? AND slug = ?`,
		project, slug).Scan(&rowid)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("look up %s/%s: %w", project, slug, err)
	}

	if _, err := tx.Exec(`DELETE FROM memories_fts WHERE rowid = ?`, rowid); err != nil {
		return fmt.Errorf("delete search row for %s/%s: %w", project, slug, err)
	}
	if _, err := tx.Exec(`DELETE FROM memories WHERE rowid = ?`, rowid); err != nil {
		return fmt.Errorf("delete %s/%s: %w", project, slug, err)
	}
	return tx.Commit()
}

// DeleteProject removes every memory belonging to a project.
func (ix *Index) DeleteProject(project string) error {
	slugs, err := ix.Slugs(project)
	if err != nil {
		return err
	}
	for _, slug := range slugs {
		if err := ix.Delete(project, slug); err != nil {
			return err
		}
	}
	return nil
}

// Slugs lists the memory slugs the index holds for a project.
func (ix *Index) Slugs(project string) ([]string, error) {
	rows, err := ix.db.Query(`SELECT slug FROM memories WHERE project = ?`, project)
	if err != nil {
		return nil, fmt.Errorf("list indexed slugs for %s: %w", project, err)
	}
	defer rows.Close()

	slugs := []string{}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("scan indexed slug: %w", err)
		}
		slugs = append(slugs, slug)
	}
	return slugs, rows.Err()
}

// Stamps returns the recorded file state for every indexed memory, keyed
// "project/slug". Reconcile uses it to touch only what changed.
func (ix *Index) Stamps() (map[string]Stamp, error) {
	rows, err := ix.db.Query(`SELECT project, slug, mtime, size FROM memories`)
	if err != nil {
		return nil, fmt.Errorf("read index stamps: %w", err)
	}
	defer rows.Close()

	out := map[string]Stamp{}
	for rows.Next() {
		var (
			project, slug string
			mtime, size   int64
		)
		if err := rows.Scan(&project, &slug, &mtime, &size); err != nil {
			return nil, fmt.Errorf("scan index stamp: %w", err)
		}
		out[project+"/"+slug] = Stamp{ModTime: time.Unix(0, mtime), Size: size}
	}
	return out, rows.Err()
}

// Search runs a ranked full-text query. An empty project searches everything.
func (ix *Index) Search(query, project string, limit int) ([]Hit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query is empty")
	}
	if limit <= 0 {
		limit = 10
	}

	// Punctuation only, such as "*" or "()". Saying so beats returning zero
	// hits, which would read as "nothing matches" and send the caller looking
	// for content that was never searched for.
	literal := quoteTerms(query)
	if literal == "" {
		return nil, fmt.Errorf("query %q has no searchable terms", query)
	}

	hits, err := ix.search(query, project, limit)
	if err == nil {
		return hits, nil
	}

	// The query was not valid FTS5 syntax — a bare "C++" or an unbalanced
	// quote, say. Re-run it as literal phrases rather than making the caller
	// learn the query language.
	hits, retryErr := ix.search(literal, project, limit)
	if retryErr != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	return hits, nil
}

func (ix *Index) search(match, project string, limit int) ([]Hit, error) {
	q := `
		SELECT m.project, m.slug, m.id, m.title, m.tags,
		       snippet(memories_fts, 2, '[', ']', ' … ', 16),
		       bm25(memories_fts)
		FROM memories_fts
		JOIN memories m ON m.rowid = memories_fts.rowid
		WHERE memories_fts MATCH ?`
	args := []any{match}
	if project != "" {
		q += ` AND m.project = ?`
		args = append(args, project)
	}
	q += ` ORDER BY bm25(memories_fts) LIMIT ?`
	args = append(args, limit)

	rows, err := ix.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := []Hit{}
	for rows.Next() {
		var (
			h       Hit
			tags    string
			snippet string
			score   float64
		)
		if err := rows.Scan(&h.Project, &h.Slug, &h.ID, &h.Title, &tags, &snippet, &score); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		if tags != "" {
			h.Tags = strings.Fields(tags)
		}
		h.Snippet = strings.TrimSpace(snippet)
		// bm25 returns a negative score where more negative is a better match.
		// Flipping the sign means callers can sort descending like everywhere else.
		h.Score = -score
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// quoteTerms turns arbitrary user text into a valid FTS5 query by wrapping each
// word in double quotes, which makes every term a literal phrase.
func quoteTerms(query string) string {
	terms := []string{}
	for field := range strings.FieldsSeq(query) {
		cleaned := strings.Trim(field, `"'()*^-:`)
		if cleaned == "" {
			continue
		}
		terms = append(terms, `"`+strings.ReplaceAll(cleaned, `"`, `""`)+`"`)
	}
	return strings.Join(terms, " ")
}

func format(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
