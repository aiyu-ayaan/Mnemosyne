package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/events"
	"github.com/aiyu-ayaan/mnemosyne/internal/markdown"
)

const memoryExt = ".md"

// Meta is a memory without its body. Listings return these so that enumerating
// a large project cannot flood an agent's context window.
type Meta struct {
	Project string    `json:"project"`
	Slug    string    `json:"slug"`
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Tags    []string  `json:"tags,omitempty"`
	Links   []string  `json:"links,omitempty"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// Memory is a full memory, body included.
type Memory struct {
	Meta
	Body string `json:"body"`
}

// WriteRequest describes a create-or-update. Tags and Links are pointers so
// that "leave them alone" and "set them to empty" stay distinguishable.
type WriteRequest struct {
	Project string
	Memory  string // existing slug or ULID; empty creates a new memory
	Title   string
	Body    string
	Tags    *[]string
	Links   *[]string

	// Mode is how Body meets the existing body: ModeReplace (the default),
	// ModeAppend, or ModePrepend. It only applies to an update — on create
	// there is nothing to join, so Body is written as given whatever it says.
	Mode string
}

// How a write joins an existing body. Appending is the common case an agent
// would otherwise pay for twice: read the whole memory, then write the whole
// memory back with one line added.
const (
	ModeReplace = "replace"
	ModeAppend  = "append"
	ModePrepend = "prepend"
)

// joinBody applies a write mode. Bodies are separated by a blank line, so
// appended Markdown stays valid: a list item glued to a paragraph is not.
func joinBody(existing, incoming, mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ModeReplace:
		return incoming, nil
	case ModeAppend:
		if strings.TrimSpace(existing) == "" {
			return incoming, nil
		}
		return strings.TrimRight(existing, "\n") + "\n\n" + strings.TrimLeft(incoming, "\n"), nil
	case ModePrepend:
		if strings.TrimSpace(existing) == "" {
			return incoming, nil
		}
		return strings.TrimRight(incoming, "\n") + "\n\n" + strings.TrimLeft(existing, "\n"), nil
	default:
		return "", fmt.Errorf("unknown write mode %q: use replace, append, or prepend: %w", mode, ErrInvalid)
	}
}

// memoryPath validates a memory slug and returns its file path.
func (s *Store) memoryPath(dir, slug string) (string, error) {
	if !markdown.ValidSlug(slug) {
		return "", fmt.Errorf("invalid memory name %q: use lowercase letters, digits, and hyphens: %w", slug, ErrInvalid)
	}
	path := filepath.Join(dir, slug+memoryExt)
	if err := s.contained(path); err != nil {
		return "", err
	}
	return path, nil
}

// memorySlugs lists the memory slugs in a project directory, sorted.
func (s *Store) memorySlugs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read project directory: %w", err)
	}
	slugs := []string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, memoryExt) || strings.HasPrefix(name, ".") {
			continue
		}
		slugs = append(slugs, strings.TrimSuffix(name, memoryExt))
	}
	sort.Strings(slugs)
	return slugs, nil
}

// ListMemories returns metadata for a project's memories. An empty tag matches
// everything; limit <= 0 means no limit.
func (s *Store) ListMemories(project, tag string, limit int) ([]Meta, error) {
	dir, err := s.projectDir(project)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("project %q: %w", project, ErrNotFound)
	}

	slugs, err := s.memorySlugs(dir)
	if err != nil {
		return nil, err
	}

	out := []Meta{}
	for _, slug := range slugs {
		m, err := s.ReadMemory(project, slug)
		if err != nil {
			// One unreadable file must not make the whole project unlistable.
			continue
		}
		if tag != "" && !hasTag(m.Tags, tag) {
			continue
		}
		out = append(out, m.Meta)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// ReadMemory loads one memory by slug or ULID.
func (s *Store) ReadMemory(project, ref string) (*Memory, error) {
	dir, err := s.projectDir(project)
	if err != nil {
		return nil, err
	}
	slug, err := s.resolveRef(project, dir, ref)
	if err != nil {
		return nil, err
	}
	path, err := s.memoryPath(dir, slug)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("memory %q in project %q: %w", ref, project, ErrNotFound)
	}
	parsed, err := markdown.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse memory %q: %w", slug, err)
	}

	return &Memory{Meta: metaFrom(project, slug, parsed), Body: parsed.Body}, nil
}

// WriteMemory creates or updates a memory, creating the project if needed.
// It reports whether a new memory was created.
func (s *Store) WriteMemory(req WriteRequest) (*Memory, bool, error) {
	if strings.TrimSpace(req.Title) == "" && req.Memory == "" {
		return nil, false, fmt.Errorf("a new memory needs a title: %w", ErrInvalid)
	}
	// Rejected here rather than where the bodies are joined, so a misspelled
	// mode fails on a create too instead of being silently ignored.
	if _, err := joinBody("", "", req.Mode); err != nil {
		return nil, false, err
	}
	if _, err := s.EnsureProject(req.Project); err != nil {
		return nil, false, err
	}
	dir, err := s.projectDir(req.Project)
	if err != nil {
		return nil, false, err
	}

	// Truncated to the precision RFC 3339 frontmatter stores, so the value
	// returned here is identical to what a later read of the file yields.
	now := time.Now().UTC().Truncate(time.Second)
	var (
		doc     *markdown.Memory
		slug    string
		created bool
	)

	switch {
	case req.Memory != "":
		slug, err = s.resolveRef(req.Project, dir, req.Memory)
		switch {
		case err == nil:
			raw, err := os.ReadFile(filepath.Join(dir, slug+memoryExt))
			if err != nil {
				return nil, false, fmt.Errorf("read memory %q: %w", slug, err)
			}
			if doc, err = markdown.Parse(raw); err != nil {
				return nil, false, fmt.Errorf("parse memory %q: %w", slug, err)
			}

		// A slug the caller named but that does not exist yet is a create, not
		// an error: callers keep well-known memories ("todo", "decisions") at a
		// fixed slug, and the first write to one must not have to be a special
		// case. An id-shaped ref still fails, since an id cannot be chosen.
		case errors.Is(err, ErrNotFound) && markdown.ValidSlug(req.Memory):
			slug = req.Memory
			doc = &markdown.Memory{ID: markdown.NewID(), Created: now, Extra: map[string]any{}}
			created = true
			if strings.TrimSpace(req.Title) == "" {
				doc.Title = titleFromSlug(slug)
			}

		default:
			return nil, false, err
		}

	default:
		slug, err = s.uniqueSlug(dir, req.Title)
		if err != nil {
			return nil, false, err
		}
		doc = &markdown.Memory{ID: markdown.NewID(), Created: now, Extra: map[string]any{}}
		created = true
	}

	if strings.TrimSpace(req.Title) != "" {
		doc.Title = req.Title
	}
	if doc.ID == "" {
		doc.ID = markdown.NewID()
	}
	if doc.Created.IsZero() {
		doc.Created = now
	}
	if req.Tags != nil {
		doc.Tags = normaliseList(*req.Tags)
	}
	if req.Links != nil {
		doc.Links = normaliseList(*req.Links)
	}
	// A create has no existing body to join, so the mode is inert rather than
	// an error: "append to todo" must work the first time as well as the tenth.
	if created {
		doc.Body = req.Body
	} else if doc.Body, err = joinBody(doc.Body, req.Body, req.Mode); err != nil {
		return nil, false, err
	}
	doc.Updated = now

	data, err := doc.Format()
	if err != nil {
		return nil, false, fmt.Errorf("format memory %q: %w", slug, err)
	}
	path, err := s.memoryPath(dir, slug)
	if err != nil {
		return nil, false, err
	}
	if err := writeFileAtomic(path, data); err != nil {
		return nil, false, err
	}

	written := &Memory{Meta: metaFrom(req.Project, slug, doc), Body: doc.Body}
	s.reindex(written)
	s.publish(events.MemoryWritten, req.Project, slug)
	return written, created, nil
}

// TrashDir is where deleted memories are kept, under the root's internal
// directory so it is never mistaken for a project.
const TrashDir = "trash"

// DeleteMemory removes a memory from its project and returns where the file
// went. It is a move into the trash, not an unlink: an agent that deletes the
// wrong memory should cost the user a restore, not the memory.
//
// The index row and the change event are the same either way — as far as every
// reader is concerned the memory is gone.
func (s *Store) DeleteMemory(project, ref string) (string, error) {
	dir, err := s.projectDir(project)
	if err != nil {
		return "", err
	}
	slug, err := s.resolveRef(project, dir, ref)
	if err != nil {
		return "", err
	}
	path, err := s.memoryPath(dir, slug)
	if err != nil {
		return "", err
	}

	trash := filepath.Join(s.InternalPath(TrashDir), project)
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return "", fmt.Errorf("create trash directory: %w", err)
	}
	// Stamped, so deleting two memories that share a slug across time does not
	// have the second silently overwrite the first.
	grave := filepath.Join(trash, fmt.Sprintf("%s-%d%s", slug, time.Now().Unix(), memoryExt))
	if err := os.Rename(path, grave); err != nil {
		return "", fmt.Errorf("memory %q in project %q: %w", ref, project, ErrNotFound)
	}

	s.unindex(project, slug)
	s.publish(events.MemoryDeleted, project, slug)
	return grave, nil
}

// resolveRef turns a slug or ULID into a slug. A slug hit is a single stat; a
// ULID falls back to scanning the project, which is what lets an id survive a
// rename.
func (s *Store) resolveRef(project, dir, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("memory reference is empty: %w", ErrInvalid)
	}
	if markdown.ValidSlug(ref) {
		if path, err := s.memoryPath(dir, ref); err == nil {
			if _, err := os.Stat(path); err == nil {
				return ref, nil
			}
		}
	} else if !markdown.LooksLikeID(ref) {
		// Neither a usable slug nor an id. Saying so beats scanning the project
		// to conclude the same thing, and tells the caller what was wrong.
		return "", fmt.Errorf("invalid memory name %q: use a slug or an id: %w", ref, ErrInvalid)
	}

	// Only an id-shaped reference is worth a scan, which is what lets an id
	// survive a rename. A mistyped slug fails above instead of reading every
	// file in the project.
	if !markdown.LooksLikeID(ref) {
		return "", fmt.Errorf("memory %q in project %q: %w", ref, project, ErrNotFound)
	}

	slugs, err := s.memorySlugs(dir)
	if err != nil {
		return "", fmt.Errorf("memory %q in project %q: %w", ref, project, ErrNotFound)
	}
	for _, slug := range slugs {
		data, err := os.ReadFile(filepath.Join(dir, slug+memoryExt))
		if err != nil {
			continue
		}
		doc, err := markdown.Parse(data)
		if err == nil && doc.ID != "" && strings.EqualFold(doc.ID, ref) {
			return slug, nil
		}
	}
	return "", fmt.Errorf("memory %q in project %q: %w", ref, project, ErrNotFound)
}

// uniqueSlug derives a slug from the title, suffixing -2, -3, and so on when
// the name is already taken in this project.
func (s *Store) uniqueSlug(dir, title string) (string, error) {
	base, err := markdown.Slugify(title)
	if err != nil {
		return "", err
	}
	candidate := base
	for n := 2; ; n++ {
		path, err := s.memoryPath(dir, candidate)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return candidate, nil
		}
		suffix := fmt.Sprintf("-%d", n)
		trimmed := base
		if len(trimmed)+len(suffix) > markdown.MaxSlugLen {
			trimmed = strings.Trim(base[:markdown.MaxSlugLen-len(suffix)], "-")
		}
		candidate = trimmed + suffix
	}
}

func metaFrom(project, slug string, doc *markdown.Memory) Meta {
	return Meta{
		Project: project,
		Slug:    slug,
		ID:      doc.ID,
		Title:   doc.Title,
		Tags:    doc.Tags,
		Links:   doc.Links,
		Created: doc.Created,
		Updated: doc.Updated,
	}
}

// normaliseList lowercases, trims, and de-duplicates a tag or link list so that
// "Decision" and "decision " cannot become two different tags.
func normaliseList(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range in {
		v := strings.ToLower(strings.TrimSpace(item))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// hasTag matches against normalised tags, so the caller's filter does not have
// to be spelled exactly as the tag was stored.
func hasTag(tags []string, tag string) bool {
	return slices.Contains(tags, strings.ToLower(strings.TrimSpace(tag)))
}

// titleFromSlug gives a memory created at a caller-chosen slug a readable
// title when none was supplied: "dev-log" becomes "Dev Log".
func titleFromSlug(slug string) string {
	words := strings.FieldsFunc(slug, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
