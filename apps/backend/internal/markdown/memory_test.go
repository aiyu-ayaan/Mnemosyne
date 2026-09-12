package markdown

import (
	"strings"
	"testing"
	"time"
)

func TestParseFullFrontmatter(t *testing.T) {
	src := `---
id: 01JD3K7QW8ZX4N2P
title: Use FTS5, not vectors
tags: [decision, search]
links: [storage-is-file-first]
created: 2026-09-12T22:40:11Z
updated: 2026-09-13T08:00:00Z
reviewer: ayaan
---

Body line one.

Body line two.
`

	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if m.ID != "01JD3K7QW8ZX4N2P" {
		t.Errorf("ID = %q", m.ID)
	}
	if m.Title != "Use FTS5, not vectors" {
		t.Errorf("Title = %q", m.Title)
	}
	if got := strings.Join(m.Tags, ","); got != "decision,search" {
		t.Errorf("Tags = %q", got)
	}
	if got := strings.Join(m.Links, ","); got != "storage-is-file-first" {
		t.Errorf("Links = %q", got)
	}
	if !m.Created.Equal(time.Date(2026, 9, 12, 22, 40, 11, 0, time.UTC)) {
		t.Errorf("Created = %v", m.Created)
	}
	if m.Extra["reviewer"] != "ayaan" {
		t.Errorf("Extra[reviewer] = %v; unknown keys must be preserved", m.Extra["reviewer"])
	}
	if !strings.HasPrefix(m.Body, "Body line one.") {
		t.Errorf("Body = %q", m.Body)
	}
}

func TestParseWithoutFrontmatter(t *testing.T) {
	m, err := Parse([]byte("Just a note.\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Body != "Just a note.\n" {
		t.Errorf("Body = %q", m.Body)
	}
	if m.Title != "" {
		t.Errorf("Title = %q, want empty", m.Title)
	}
}

func TestParseUnclosedFrontmatterFails(t *testing.T) {
	if _, err := Parse([]byte("---\ntitle: oops\n")); err == nil {
		t.Fatal("Parse succeeded on unclosed frontmatter; want error")
	}
}

func TestParseCRLF(t *testing.T) {
	m, err := Parse([]byte("---\r\ntitle: Windows\r\n---\r\n\r\nBody.\r\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Title != "Windows" {
		t.Errorf("Title = %q", m.Title)
	}
	if m.Body != "Body.\n" {
		t.Errorf("Body = %q", m.Body)
	}
}

func TestRoundTripPreservesEverything(t *testing.T) {
	now := time.Date(2026, 9, 12, 22, 40, 11, 0, time.UTC)
	original := &Memory{
		ID:      NewID(),
		Title:   "Round trip",
		Tags:    []string{"a", "b"},
		Links:   []string{"other"},
		Created: now,
		Updated: now,
		Extra:   map[string]any{"reviewer": "ayaan"},
		Body:    "Some body.\n",
	}

	data, err := original.Format()
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.ID != original.ID || got.Title != original.Title {
		t.Errorf("identity lost: %+v", got)
	}
	if strings.Join(got.Tags, ",") != "a,b" || strings.Join(got.Links, ",") != "other" {
		t.Errorf("lists lost: tags=%v links=%v", got.Tags, got.Links)
	}
	if !got.Created.Equal(now) || !got.Updated.Equal(now) {
		t.Errorf("times lost: %v / %v", got.Created, got.Updated)
	}
	if got.Extra["reviewer"] != "ayaan" {
		t.Errorf("Extra lost: %v", got.Extra)
	}
	if got.Body != original.Body {
		t.Errorf("Body = %q, want %q", got.Body, original.Body)
	}
}

func TestFormatIsStable(t *testing.T) {
	src := &Memory{ID: "X", Title: "T", Created: time.Now(), Updated: time.Now(), Body: "b\n"}
	first, err := src.Format()
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	parsed, err := Parse(first)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	second, err := parsed.Format()
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("format is not idempotent:\n%s\n---\n%s", first, second)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Use FTS5, not vectors":  "use-fts5-not-vectors",
		"  Leading & trailing  ": "leading-trailing",
		"Hello___World":          "hello-world",
		"C++ / Go":               "c-go",
	}
	for title, want := range cases {
		got, err := Slugify(title)
		if err != nil {
			t.Errorf("Slugify(%q): %v", title, err)
			continue
		}
		if got != want {
			t.Errorf("Slugify(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestSlugifyRejectsUnusableTitle(t *testing.T) {
	if _, err := Slugify("!!! ???"); err == nil {
		t.Fatal("Slugify succeeded on a title with no usable characters")
	}
}

func TestSlugifyTruncatesLongTitles(t *testing.T) {
	got, err := Slugify(strings.Repeat("word ", 50))
	if err != nil {
		t.Fatalf("Slugify: %v", err)
	}
	if len(got) > MaxSlugLen {
		t.Errorf("slug length %d exceeds %d", len(got), MaxSlugLen)
	}
	if strings.HasSuffix(got, "-") {
		t.Errorf("slug %q ends in a separator after truncation", got)
	}
}

func TestValidSlugRejectsTraversal(t *testing.T) {
	bad := []string{"..", "../etc", "a/b", "a\b", "A", "-lead", "", "has space", "x:y"}
	for _, s := range bad {
		if ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = true; must reject", s)
		}
	}
	for _, s := range []string{"a", "a-b", "use-fts5-not-vectors", "01jd3k"} {
		if !ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = false; must accept", s)
		}
	}
}
