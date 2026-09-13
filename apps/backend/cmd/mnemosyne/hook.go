package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aiyu-ayaan/mnemosyne/internal/markdown"
	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

const hookUsage = `Usage: mnemosyne hook session-start

Prints what this directory's project already holds, for an agent to read at the
start of a session. Wire it in with "mnemosyne agents install".
`

// hookMemories is how many memory titles the block lists. The point is to show
// the agent what exists, not to be the index — past this many, the shape of the
// project is clear and the rest is a recall away.
const hookMemories = 12

// hookCmd is run by an agent's session-start hook, so its contract is unusual:
// whatever it prints lands in the agent's context, and it must never fail the
// session. A missing root, an unreadable index, no project for this directory —
// all of them mean "nothing to say", which is printing nothing and exiting 0.
func hookCmd(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(hookUsage)
		return nil
	}
	if args[0] != "session-start" {
		return fmt.Errorf("unknown hook %q\n\n%s", args[0], hookUsage)
	}

	if block := sessionStartBlock(args[1:]); block != "" {
		fmt.Print(block)
	}
	return nil
}

// sessionStartBlock renders the context, or "" when there is nothing worth
// spending the agent's context on.
func sessionStartBlock(args []string) string {
	slug := projectSlugForCwd()
	if slug == "" {
		return ""
	}

	loc, root, _, err := locate(flag.NewFlagSet("hook", flag.ContinueOnError), args)
	if err != nil {
		return ""
	}
	s, err := store.Open(root)
	if err != nil {
		return ""
	}
	defer s.Close()

	// A development build reads a different root than the installed one. An
	// agent about to write memories deserves to know which.
	mode := ""
	if loc.Dev {
		mode = " (development build)"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Mnemosyne holds this user's memory across sessions and across agents, at %s%s.\n", root, mode)
	fmt.Fprintf(&b, "The project slug for this directory is %q.\n\n", slug)

	metas, err := s.ListMemories(slug, "", hookMemories)
	switch {
	case err != nil || len(metas) == 0:
		// Saying the project is empty is worth more than saying nothing: it
		// tells the agent not to spend a recall, and that a write here creates
		// the project rather than failing.
		b.WriteString("Nothing is stored for it yet. A project is created by its first write, so\n")
		b.WriteString("write_memory with that slug is all it takes — there is no create-project call,\n")
		b.WriteString("and nothing goes in the working tree.\n")
	default:
		fmt.Fprintf(&b, "It already holds %d:\n", len(metas))
		for _, m := range metas {
			if m.Title != "" && m.Title != m.Slug {
				fmt.Fprintf(&b, "  %s — %s\n", m.Slug, m.Title)
				continue
			}
			fmt.Fprintf(&b, "  %s\n", m.Slug)
		}
		b.WriteString("\nRead the ones that bear on this task before exploring the code, and recall\n")
		b.WriteString("for anything not in that list. Do not restate them back to the user unasked.\n")
	}

	b.WriteString("\nBefore the session ends, write down what is still true after it: decisions and\n")
	b.WriteString("why, corrections, conventions, gotchas. Update the memory that already covers\n")
	b.WriteString("a fact rather than writing a second one near it.\n")
	return b.String()
}

// projectSlugForCwd derives the project slug from the working directory, which
// is where the agent was started and therefore the repository the user means.
//
// The git root is preferred over the raw directory so that a session started in
// apps/backend belongs to the same project as one started at the top.
func projectSlugForCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if root := gitRoot(cwd); root != "" {
		cwd = root
	}
	slug, err := markdown.Slugify(filepath.Base(cwd))
	if err != nil {
		return ""
	}
	return slug
}

// gitRoot walks up looking for a .git entry. Shelling out to git would be one
// process spawn on every session start for an answer four os.Stat calls give.
func gitRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
