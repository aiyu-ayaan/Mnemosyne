// Package store is the core of Mnemosyne: projects and memories on disk.
// It knows nothing about MCP, HTTP, or SQLite. Every transport calls into here,
// so behaviour cannot drift between surfaces.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/markdown"
)

// InternalDir holds Mnemosyne's own files inside the memory root. It is skipped
// when enumerating projects.
const InternalDir = ".mnemosyne"

const projectFile = "project.json"

// ErrNotFound is returned for a missing project or memory. Callers match on it
// with errors.Is to turn it into their own transport's not-found response.
var ErrNotFound = errors.New("not found")

// Store owns a memory root directory.
type Store struct {
	root string
}

// Project is a directory of memories.
type Project struct {
	Slug        string    `json:"slug"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Created     time.Time `json:"created"`
	MemoryCount int       `json:"memoryCount"`
}

// Open prepares root for use, creating it if it does not exist.
func Open(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve memory root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(abs, InternalDir), 0o755); err != nil {
		return nil, fmt.Errorf("create memory root: %w", err)
	}
	return &Store{root: abs}, nil
}

// Root is the absolute path of the memory root.
func (s *Store) Root() string { return s.root }

// InternalPath returns a path inside the root's .mnemosyne directory.
func (s *Store) InternalPath(name string) string {
	return filepath.Join(s.root, InternalDir, name)
}

// projectDir validates a project slug and returns its directory. Slug validation
// and path containment are checked independently: a traversal here writes
// arbitrary files on the user's machine, so one guard is not enough.
func (s *Store) projectDir(project string) (string, error) {
	if !markdown.ValidSlug(project) {
		return "", fmt.Errorf("invalid project name %q: use lowercase letters, digits, and hyphens", project)
	}
	dir := filepath.Join(s.root, project)
	if err := s.contained(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// contained rejects any path that resolves outside the memory root.
func (s *Store) contained(path string) error {
	rel, err := filepath.Rel(s.root, filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes the memory root", path)
	}
	return nil
}

// ListProjects returns every project in the root, sorted by slug.
func (s *Store) ListProjects() ([]Project, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("read memory root: %w", err)
	}

	projects := []Project{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p, err := s.GetProject(e.Name())
		if err != nil {
			// A directory that is not a usable project should not hide the ones
			// that are.
			continue
		}
		projects = append(projects, *p)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Slug < projects[j].Slug })
	return projects, nil
}

// GetProject reads one project. A directory without project.json is still a
// project — dropping a folder of Markdown into the root should just work — so
// the metadata is synthesised rather than demanded.
func (s *Store) GetProject(project string) (*Project, error) {
	dir, err := s.projectDir(project)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("project %q: %w", project, ErrNotFound)
	}

	p := Project{Slug: project, Name: project, Created: info.ModTime().UTC()}
	if data, err := os.ReadFile(filepath.Join(dir, projectFile)); err == nil {
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parse %s for project %q: %w", projectFile, project, err)
		}
		p.Slug = project // the directory name is authoritative
	}
	if p.Name == "" {
		p.Name = project
	}

	slugs, err := s.memorySlugs(dir)
	if err != nil {
		return nil, err
	}
	p.MemoryCount = len(slugs)
	return &p, nil
}

// EnsureProject returns the project, creating it if it does not exist. Creation
// is implicit because an agent writing its first memory should not have to make
// a separate call to declare the project first.
func (s *Store) EnsureProject(project string) (*Project, error) {
	if p, err := s.GetProject(project); err == nil {
		return p, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	dir, err := s.projectDir(project)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create project %q: %w", project, err)
	}

	p := Project{Slug: project, ID: markdown.NewID(), Name: project, Created: time.Now().UTC().Truncate(time.Second)}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode project %q: %w", project, err)
	}
	if err := writeFileAtomic(filepath.Join(dir, projectFile), append(data, '\n')); err != nil {
		return nil, err
	}
	return &p, nil
}

// DeleteProject removes a project and every memory in it.
func (s *Store) DeleteProject(project string) error {
	dir, err := s.projectDir(project)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("project %q: %w", project, ErrNotFound)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("delete project %q: %w", project, err)
	}
	return nil
}

// writeFileAtomic writes to a temp file in the same directory and renames it
// over the target. A same-directory rename is atomic on every supported
// platform, so a crash mid-write leaves the old file or the new one, never a
// half-written one.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("sync %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close %s: %w", path, err)
	}

	// Windows will not rename over an existing file, so clear the target first.
	// The window this opens is why the temp file is fsynced before the rename.
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			cleanup()
			return fmt.Errorf("replace %s: %w", path, err)
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename into %s: %w", path, err)
	}
	return nil
}
