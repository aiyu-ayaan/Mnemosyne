package markdown

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"gopkg.in/yaml.v3"
)

const delimiter = "---"

// Memory is one memory file: YAML frontmatter plus a Markdown body.
type Memory struct {
	ID      string
	Title   string
	Tags    []string
	Links   []string
	Created time.Time
	Updated time.Time

	// Extra holds frontmatter keys Mnemosyne does not define. They are carried
	// through a read/write cycle untouched: another tool putting a key in this
	// file is not ours to discard.
	Extra map[string]any

	Body string
}

// NewID returns a ULID: unique, and lexically sortable by creation time.
func NewID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// known lists the frontmatter keys Mnemosyne owns, so Parse can separate them
// from the ones it must preserve verbatim.
var known = map[string]bool{
	"id": true, "title": true, "tags": true,
	"links": true, "created": true, "updated": true,
}

// Parse reads a memory file. A file without frontmatter is not an error: it is
// treated as a bare body, so a folder of ordinary Markdown notes can be dropped
// into the memory root and still work.
func Parse(data []byte) (*Memory, error) {
	body, front, err := split(data)
	if err != nil {
		return nil, err
	}

	m := &Memory{Body: body, Extra: map[string]any{}}
	if len(front) == 0 {
		return m, nil
	}

	raw := map[string]any{}
	if err := yaml.Unmarshal(front, &raw); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	m.ID, _ = raw["id"].(string)
	m.Title, _ = raw["title"].(string)
	m.Tags = stringSlice(raw["tags"])
	m.Links = stringSlice(raw["links"])
	m.Created = parseTime(raw["created"])
	m.Updated = parseTime(raw["updated"])

	for k, v := range raw {
		if !known[k] {
			m.Extra[k] = v
		}
	}
	return m, nil
}

// split separates the frontmatter block from the body. It returns empty
// frontmatter when the file does not open with a delimiter.
func split(data []byte) (body string, front []byte, err error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, delimiter+"\n") {
		return text, nil, nil
	}

	rest := text[len(delimiter)+1:]
	end := strings.Index(rest, "\n"+delimiter)
	if end < 0 {
		return "", nil, fmt.Errorf("frontmatter opened with %q but never closed", delimiter)
	}

	front = []byte(rest[:end])
	// rest[end:] is "\n---" plus whatever follows. Step over the closing
	// delimiter, then its line terminator, then the one blank line that
	// conventionally separates the frontmatter from the body.
	after := strings.TrimPrefix(rest[end+1+len(delimiter):], "\n")
	return strings.TrimPrefix(after, "\n"), front, nil
}

// Format renders the memory back to file bytes. Known keys are written in a
// fixed order so that an unchanged memory produces an unchanged file and diffs
// stay readable; preserved keys follow, sorted by yaml.
func (m *Memory) Format() ([]byte, error) {
	front := map[string]any{}
	for k, v := range m.Extra {
		if !known[k] {
			front[k] = v
		}
	}

	var buf bytes.Buffer
	buf.WriteString(delimiter + "\n")

	// yaml.Marshal on a map sorts keys alphabetically, which would scatter the
	// fields a human reads first. The ordered ones are written by hand.
	enc := func(key string, value any) error {
		out, err := yaml.Marshal(map[string]any{key: value})
		if err != nil {
			return fmt.Errorf("encode %s: %w", key, err)
		}
		buf.Write(out)
		return nil
	}

	ordered := []struct {
		key   string
		value any
		omit  bool
	}{
		{"id", m.ID, m.ID == ""},
		{"title", m.Title, m.Title == ""},
		{"tags", m.Tags, len(m.Tags) == 0},
		{"links", m.Links, len(m.Links) == 0},
		{"created", m.Created.UTC().Format(time.RFC3339), m.Created.IsZero()},
		{"updated", m.Updated.UTC().Format(time.RFC3339), m.Updated.IsZero()},
	}
	for _, f := range ordered {
		if f.omit {
			continue
		}
		if err := enc(f.key, f.value); err != nil {
			return nil, err
		}
	}

	if len(front) > 0 {
		out, err := yaml.Marshal(front)
		if err != nil {
			return nil, fmt.Errorf("encode preserved frontmatter: %w", err)
		}
		buf.Write(out)
	}

	buf.WriteString(delimiter + "\n\n")
	buf.WriteString(strings.TrimLeft(m.Body, "\n"))
	if !strings.HasSuffix(m.Body, "\n") {
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

func stringSlice(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseTime accepts both a yaml-native timestamp and an RFC 3339 string, since
// a hand-edited file may quote the value or not.
func parseTime(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t.UTC()
	case string:
		parsed, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return time.Time{}
		}
		return parsed.UTC()
	}
	return time.Time{}
}
