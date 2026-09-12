package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Windows will not remove an open database file, so the index has to be
	// closed before TempDir cleanup runs.
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func write(t *testing.T, s *Store, project, title, body string, tags ...string) *Memory {
	t.Helper()
	req := WriteRequest{Project: project, Title: title, Body: body}
	if len(tags) > 0 {
		req.Tags = &tags
	}
	m, _, err := s.WriteMemory(req)
	if err != nil {
		t.Fatalf("WriteMemory(%q): %v", title, err)
	}
	return m
}

func TestWriteCreatesProjectAndFile(t *testing.T) {
	s := newStore(t)

	m, created, err := s.WriteMemory(WriteRequest{
		Project: "mnemosyne",
		Title:   "Use FTS5, not vectors",
		Body:    "sqlite-vec needs cgo.\n",
	})
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}
	if !created {
		t.Error("created = false on first write")
	}
	if m.Slug != "use-fts5-not-vectors" {
		t.Errorf("Slug = %q", m.Slug)
	}
	if m.ID == "" || m.Created.IsZero() || m.Updated.IsZero() {
		t.Errorf("metadata not populated: %+v", m.Meta)
	}

	path := filepath.Join(s.Root(), "mnemosyne", "use-fts5-not-vectors.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("memory file not on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Root(), "mnemosyne", projectFile)); err != nil {
		t.Errorf("project.json not written: %v", err)
	}
}

func TestWriteUpdatePreservesIdentity(t *testing.T) {
	s := newStore(t)
	first := write(t, s, "p", "Original title", "v1\n", "alpha")

	updated, created, err := s.WriteMemory(WriteRequest{
		Project: "p",
		Memory:  first.Slug,
		Title:   "Renamed title",
		Body:    "v2\n",
	})
	if err != nil {
		t.Fatalf("WriteMemory update: %v", err)
	}
	if created {
		t.Error("created = true when updating an existing memory")
	}
	if updated.ID != first.ID {
		t.Errorf("ID changed on update: %q -> %q", first.ID, updated.ID)
	}
	if updated.Slug != first.Slug {
		t.Errorf("slug changed on update: %q -> %q", first.Slug, updated.Slug)
	}
	if !updated.Created.Equal(first.Created) {
		t.Errorf("Created changed on update: %v -> %v", first.Created, updated.Created)
	}
	if updated.Title != "Renamed title" || updated.Body != "v2\n" {
		t.Errorf("update did not apply: %+v", updated)
	}
	// Tags were not supplied, so they must survive untouched.
	if len(updated.Tags) != 1 || updated.Tags[0] != "alpha" {
		t.Errorf("Tags = %v; omitted fields must be left alone", updated.Tags)
	}
}

func TestWriteReplacesTagsWhenSupplied(t *testing.T) {
	s := newStore(t)
	m := write(t, s, "p", "Tagged", "body\n", "alpha", "beta")

	empty := []string{}
	cleared, _, err := s.WriteMemory(WriteRequest{Project: "p", Memory: m.Slug, Body: "body\n", Tags: &empty})
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}
	if len(cleared.Tags) != 0 {
		t.Errorf("Tags = %v, want empty when explicitly set to empty", cleared.Tags)
	}
}

func TestTagsAreNormalised(t *testing.T) {
	s := newStore(t)
	m := write(t, s, "p", "Messy tags", "body\n", "  Decision ", "decision", "SEARCH")
	if strings.Join(m.Tags, ",") != "decision,search" {
		t.Errorf("Tags = %v, want normalised and de-duplicated", m.Tags)
	}
}

func TestSlugCollisionGetsSuffix(t *testing.T) {
	s := newStore(t)
	a := write(t, s, "p", "Same title", "one\n")
	b := write(t, s, "p", "Same title", "two\n")

	if a.Slug != "same-title" || b.Slug != "same-title-2" {
		t.Fatalf("slugs = %q, %q; want same-title, same-title-2", a.Slug, b.Slug)
	}
	first, err := s.ReadMemory("p", a.Slug)
	if err != nil {
		t.Fatalf("ReadMemory: %v", err)
	}
	if first.Body != "one\n" {
		t.Errorf("first memory overwritten: body = %q", first.Body)
	}
}

func TestReadByULID(t *testing.T) {
	s := newStore(t)
	m := write(t, s, "p", "By id", "body\n")

	got, err := s.ReadMemory("p", m.ID)
	if err != nil {
		t.Fatalf("ReadMemory by ULID: %v", err)
	}
	if got.Slug != m.Slug {
		t.Errorf("Slug = %q, want %q", got.Slug, m.Slug)
	}
}

func TestListMemoriesFiltersByTag(t *testing.T) {
	s := newStore(t)
	write(t, s, "p", "One", "a\n", "decision")
	write(t, s, "p", "Two", "b\n", "gotcha")
	write(t, s, "p", "Three", "c\n", "decision")

	all, err := s.ListMemories("p", "", 0)
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len(all) = %d, want 3", len(all))
	}

	tagged, err := s.ListMemories("p", "Decision", 0)
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(tagged) != 2 {
		t.Errorf("len(tagged) = %d, want 2 (filter must be case-insensitive)", len(tagged))
	}

	limited, err := s.ListMemories("p", "", 1)
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("len(limited) = %d, want 1", len(limited))
	}
}

func TestDeleteMemory(t *testing.T) {
	s := newStore(t)
	m := write(t, s, "p", "Doomed", "body\n")

	if err := s.DeleteMemory("p", m.Slug); err != nil {
		t.Fatalf("DeleteMemory: %v", err)
	}
	if _, err := s.ReadMemory("p", m.Slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("ReadMemory after delete: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteMemory("p", m.Slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("second DeleteMemory: err = %v, want ErrNotFound", err)
	}
}

func TestListProjectsSkipsInternalAndCounts(t *testing.T) {
	s := newStore(t)
	write(t, s, "alpha", "One", "a\n")
	write(t, s, "alpha", "Two", "b\n")
	write(t, s, "beta", "Three", "c\n")

	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2 (.mnemosyne must be skipped): %+v", len(projects), projects)
	}
	if projects[0].Slug != "alpha" || projects[0].MemoryCount != 2 {
		t.Errorf("projects[0] = %+v", projects[0])
	}
	if projects[1].Slug != "beta" || projects[1].MemoryCount != 1 {
		t.Errorf("projects[1] = %+v", projects[1])
	}
}

func TestProjectWithoutMetadataStillWorks(t *testing.T) {
	s := newStore(t)
	dir := filepath.Join(s.Root(), "dropped-in")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plain-note.md"), []byte("Just a note.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := s.GetProject("dropped-in")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if p.MemoryCount != 1 {
		t.Errorf("MemoryCount = %d, want 1", p.MemoryCount)
	}

	m, err := s.ReadMemory("dropped-in", "plain-note")
	if err != nil {
		t.Fatalf("ReadMemory: %v", err)
	}
	if m.Body != "Just a note.\n" {
		t.Errorf("Body = %q", m.Body)
	}
}

func TestDeleteProject(t *testing.T) {
	s := newStore(t)
	write(t, s, "doomed", "One", "a\n")

	if err := s.DeleteProject("doomed"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := s.GetProject("doomed"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetProject after delete: err = %v, want ErrNotFound", err)
	}
}

func TestPathTraversalIsRejected(t *testing.T) {
	s := newStore(t)
	write(t, s, "p", "Real", "body\n")

	evil := []string{"..", "../outside", "..\\outside", "a/b", "C:\\Windows"}
	for _, name := range evil {
		if _, err := s.GetProject(name); err == nil {
			t.Errorf("GetProject(%q) succeeded; traversal must be rejected", name)
		}
		if _, _, err := s.WriteMemory(WriteRequest{Project: name, Title: "x", Body: "y"}); err == nil {
			t.Errorf("WriteMemory(project=%q) succeeded; traversal must be rejected", name)
		}
		if _, err := s.ReadMemory("p", name); err == nil {
			t.Errorf("ReadMemory(memory=%q) succeeded; traversal must be rejected", name)
		}
	}

	// Nothing may have been created outside the root.
	parent := filepath.Dir(s.Root())
	if _, err := os.Stat(filepath.Join(parent, "outside")); err == nil {
		t.Fatal("a file was written outside the memory root")
	}
}

func TestWriteRequiresTitleForNewMemory(t *testing.T) {
	s := newStore(t)
	if _, _, err := s.WriteMemory(WriteRequest{Project: "p", Body: "orphan\n"}); err == nil {
		t.Fatal("WriteMemory succeeded without a title on create")
	}
}

func TestAtomicWriteReplacesExistingFile(t *testing.T) {
	s := newStore(t)
	m := write(t, s, "p", "Replace me", "first\n")

	for i := range 3 {
		if _, _, err := s.WriteMemory(WriteRequest{Project: "p", Memory: m.Slug, Body: "again\n"}); err != nil {
			t.Fatalf("rewrite %d: %v", i, err)
		}
	}

	entries, err := os.ReadDir(filepath.Join(s.Root(), "p"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temp file %q left behind", e.Name())
		}
	}
}
