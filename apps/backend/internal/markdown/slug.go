// Package markdown reads and writes memories as Markdown files with YAML
// frontmatter. It owns the on-disk representation and nothing else: no storage
// decisions, no indexing, no I/O beyond a single file.
package markdown

import (
	"fmt"
	"regexp"
	"strings"
)

// MaxSlugLen caps generated slugs so that a long title cannot produce a
// filename that breaks on filesystems with path length limits.
const MaxSlugLen = 80

var (
	slugValid    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	slugStrip    = regexp.MustCompile(`[^a-z0-9]+`)
	slugCollapse = regexp.MustCompile(`-{2,}`)
)

// Slugify converts a human title into a filesystem- and URL-safe slug.
// It returns an error when the title contains nothing usable, because silently
// inventing a name for an unnamed memory hides the caller's bug.
func Slugify(title string) (string, error) {
	s := slugStrip.ReplaceAllString(strings.ToLower(title), "-")
	s = slugCollapse.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if len(s) > MaxSlugLen {
		s = strings.Trim(s[:MaxSlugLen], "-")
	}
	if s == "" {
		return "", fmt.Errorf("title %q contains no characters usable in a slug", title)
	}
	return s, nil
}

// ValidSlug reports whether s is already a well-formed slug. Every identifier
// that reaches the filesystem is checked with this first — it is the outer
// guard against path traversal, before path containment is checked again.
func ValidSlug(s string) bool {
	return len(s) <= MaxSlugLen && slugValid.MatchString(s)
}
