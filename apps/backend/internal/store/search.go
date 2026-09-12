package store

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/events"
	"github.com/aiyu-ayaan/mnemosyne/internal/index"
	"github.com/aiyu-ayaan/mnemosyne/internal/markdown"
)

// IndexFile is the index database, kept inside the root's .mnemosyne directory.
const IndexFile = "index.db"

// Search runs a ranked full-text query. An empty project searches every project.
func (s *Store) Search(query, project string, limit int) ([]index.Hit, error) {
	if s.index == nil {
		return nil, fmt.Errorf("search is unavailable: the index failed to open")
	}
	if project != "" {
		if _, err := s.GetProject(project); err != nil {
			return nil, err
		}
	}
	return s.index.Search(query, project, limit)
}

// Reconcile brings the index in line with what is on disk and returns the
// number of memories added, updated, or removed.
//
// Files are the source of truth, so this runs at startup and after any change
// Mnemosyne did not make itself: an agent editing a file directly, a git pull,
// a restored backup.
func (s *Store) Reconcile() (int, error) {
	if s.index == nil {
		return 0, nil
	}

	stamps, err := s.index.Stamps()
	if err != nil {
		return 0, err
	}

	projects, err := s.ListProjects()
	if err != nil {
		return 0, err
	}

	changed := 0
	seen := make(map[string]bool, len(stamps))

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
			key := p.Slug + "/" + slug
			seen[key] = true

			path := filepath.Join(dir, slug+memoryExt)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			// Unchanged files are skipped, so startup costs time proportional
			// to what moved rather than to how much is stored.
			if prev, ok := stamps[key]; ok &&
				prev.Size == info.Size() && prev.ModTime.Equal(info.ModTime()) {
				continue
			}
			if err := s.indexFile(p.Slug, slug, path); err != nil {
				slog.Warn("could not index memory", "project", p.Slug, "memory", slug, "err", err)
				continue
			}
			// Reconcile is how a change Mnemosyne did not make gets noticed, so
			// it is also where the event for that change comes from.
			s.publish(events.MemoryWritten, p.Slug, slug)
			changed++
		}
	}

	// Anything indexed whose file is gone was deleted behind our back.
	for key := range stamps {
		if seen[key] {
			continue
		}
		project, slug, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		if err := s.index.Delete(project, slug); err != nil {
			slog.Warn("could not drop stale index row", "project", project, "memory", slug, "err", err)
			continue
		}
		s.publish(events.MemoryDeleted, project, slug)
		changed++
	}

	if changed > 0 {
		s.bus.Publish(events.Event{Kind: events.IndexReconciled, Count: changed})
	}
	if err := s.reconcileVectors(); err != nil {
		slog.Warn("could not reconcile vectors", "err", err)
	}
	return changed, nil
}

// indexFile parses one memory file and writes it to the index.
func (s *Store) indexFile(project, slug, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	doc, err := markdown.Parse(data)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return s.index.Put(recordFrom(metaFrom(project, slug, doc), doc.Body, info.ModTime(), info.Size()))
}

// reindex updates the index after a write Mnemosyne performed itself.
//
// Index failures are logged, not returned: the memory is already safely on
// disk, and losing a search result until the next reconcile is a far smaller
// harm than failing a write that already succeeded.
func (s *Store) reindex(m *Memory) {
	if s.index == nil {
		return
	}
	dir, err := s.projectDir(m.Project)
	if err != nil {
		return
	}
	if err := s.indexFile(m.Project, m.Slug, filepath.Join(dir, m.Slug+memoryExt)); err != nil {
		slog.Warn("could not index memory", "project", m.Project, "memory", m.Slug, "err", err)
	}
	go s.embedMemory(m.Project, m.Slug, m.Meta.Title, m.Meta.Tags, m.Body)
}

// unindex drops a memory from the index after it was deleted from disk.
func (s *Store) unindex(project, slug string) {
	if s.index == nil {
		return
	}
	if err := s.index.Delete(project, slug); err != nil {
		slog.Warn("could not remove memory from index", "project", project, "memory", slug, "err", err)
	}
}

func recordFrom(meta Meta, body string, modTime time.Time, size int64) index.Record {
	return index.Record{
		Project: meta.Project,
		Slug:    meta.Slug,
		ID:      meta.ID,
		Title:   meta.Title,
		Tags:    meta.Tags,
		Body:    body,
		Created: meta.Created,
		Updated: meta.Updated,
		ModTime: modTime,
		Size:    size,
	}
}
