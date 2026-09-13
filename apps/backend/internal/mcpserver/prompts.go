package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Prompts are the workflows that are worth doing but not worth a tool.
//
// The difference matters for cost. Instructions are injected into every
// session, so they can only afford the few rules that must always hold. A tool
// is a choice the agent pays to consider on every call. A prompt is free until
// the user picks it out of their client's slash menu — which makes it the right
// home for a multi-step routine the user triggers deliberately: "check in
// everything we learned", "tell me what you already know about this repo".
//
// Each one is text, not code. The agent already has the tools; what it lacks at
// the moment the user asks is the order to use them in.

// projectArg is shared: every prompt is about one project, and a client that
// can complete the argument does it the same way each time.
func projectArg(required bool) *mcp.PromptArgument {
	return &mcp.PromptArgument{
		Name:        "project",
		Title:       "Project",
		Description: "Project slug, normally the repository directory name",
		Required:    required,
	}
}

// prompt is one registered prompt: metadata plus the text it expands to.
type prompt struct {
	name  string
	title string
	desc  string
	args  []*mcp.PromptArgument
	// body renders the message, given the arguments the client supplied.
	body func(args map[string]string) string
}

func registerPrompts(srv *mcp.Server) {
	for _, p := range prompts() {
		srv.AddPrompt(&mcp.Prompt{
			Name:        p.name,
			Title:       p.title,
			Description: p.desc,
			Arguments:   p.args,
		}, promptHandler(p))
	}
}

func promptHandler(p prompt) mcp.PromptHandler {
	return func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		args := map[string]string{}
		if req != nil && req.Params != nil {
			args = req.Params.Arguments
		}
		return &mcp.GetPromptResult{
			Description: p.desc,
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: p.body(args)},
			}},
		}, nil
	}
}

// scope renders the project argument into a phrase, so a prompt invoked with no
// project still reads as an instruction rather than as a template with a hole
// in it.
func scope(args map[string]string) string {
	if p := strings.TrimSpace(args["project"]); p != "" {
		return fmt.Sprintf("the %q project", p)
	}
	return "this repository's project (its directory name is the slug)"
}

func prompts() []prompt {
	return []prompt{{
		name:  "setup",
		title: "Set up memory for this repo",
		desc: "First run: create this repository's project and seed it from what the repo " +
			"and this session already know.",
		args: []*mcp.PromptArgument{projectArg(false)},
		body: func(args map[string]string) string {
			return fmt.Sprintf(`Set up Mnemosyne for %s. Everything below happens through the
Mnemosyne tools — do not create folders or files anywhere on disk, and do not go
looking for a Mnemosyne directory in this repository. There is not one: memories
live in Mnemosyne's own root, wherever the user configured it.

1. call list_projects. If a slug for this repository is already there, use it and
   skip to step 3; it is the repository's directory name unless the list says
   otherwise.
2. There is no create-project call and you do not need one. A project exists as
   soon as something is written to it, so step 3 creates it.
3. Seed it. Read what this repository already documents about itself — its
   README, its docs or development folder, its contributing and commit
   conventions — and write_memory one memory per convention slug, skipping any
   you have nothing real for:
     - conventions  how this codebase does things: style, commits, tests, layout
     - decisions    choices already made and why, one entry per choice
     - todo         what is done, in progress, and planned
     - dev-log      what actually shipped, newest first
   Summarise in your own words. A memory that restates the README is a second
   copy to keep in sync, not memory.
4. call list_memories to confirm what landed, then tell me in a few lines what
   you stored and what you deliberately left out.

If this repository documents nothing yet, write the one memory that is true —
what the project is and what it is for — and say the rest is waiting on real
decisions rather than inventing them.`, scope(args))
		},
	}, {
		name:  "checkpoint",
		title: "Checkpoint what we learned",
		desc: "End of session: write this conversation's durable decisions, corrections, " +
			"and gotchas into Mnemosyne before the context is lost.",
		args: []*mcp.PromptArgument{projectArg(false)},
		body: func(args map[string]string) string {
			return fmt.Sprintf(`Check in what this session learned, into %s.

1. Look back over this conversation and pick out only what is still true after
   it ends: decisions and the reasoning behind them, corrections I gave you,
   conventions you inferred, gotchas that cost time. Skip anything the code, the
   tests, or the git log already says - those are not memory, they are lookups.
2. For each one, recall first to find where it belongs. Update the memory that
   already covers it rather than writing a second one near it.
3. Then write, using the convention:
     - decisions    a choice and why, one entry per choice
     - conventions  how this codebase does things
     - dev-log      what actually shipped, prepend so newest stays first
     - todo         the checklist, updated to match reality
   Use mode 'append' or 'prepend' so you are not resending bodies you did not
   change. Anything outside those four gets its own descriptive slug.
4. Preferences about me rather than about this codebase go in the "global"
   project instead.

Then tell me in a few lines what you wrote and where. If nothing in this session
was durable, say that instead of inventing something to store.`, scope(args))
		},
	}, {
		name:  "onboard",
		title: "What do you already know?",
		desc: "Start of session: load and summarise what Mnemosyne already holds " +
			"about this project, before reading any code.",
		args: []*mcp.PromptArgument{projectArg(false)},
		body: func(args map[string]string) string {
			return fmt.Sprintf(`Before touching any code, load what you already know about %s.

1. list_memories for the project to see what exists.
2. read_memory the convention memories that are there: decisions, conventions,
   todo, dev-log. Read the others only if their titles bear on what I am about
   to ask.
3. recall anything about my preferences - a project-scoped recall also returns
   the "global" project, so one call covers both.

Then give me a short briefing: what this project is, the decisions already made
and why, the conventions you will follow, and what the todo says is in flight.
Flag anything whose age suggests it may have gone stale, and say plainly if
Mnemosyne holds nothing yet rather than padding the summary from the code.`, scope(args))
		},
	}, {
		name:  "review-stale",
		title: "Review stale memories",
		desc: "Find memories old enough to have been overtaken, and verify, update, " +
			"or retire each one.",
		args: []*mcp.PromptArgument{projectArg(false), {
			Name:        "older_than_days",
			Title:       "Older than (days)",
			Description: "Age threshold in days; defaults to 90",
			Required:    false,
		}},
		body: func(args map[string]string) string {
			days := strings.TrimSpace(args["older_than_days"])
			if days == "" {
				days = "90"
			}
			return fmt.Sprintf(`Audit %s for memories that may have gone stale.

1. list_memories for the project and pick out everything last updated more than
   %s days ago.
2. For each one, read it and check it against the code as it is now.
3. Then, per memory, do exactly one of:
     - still true            leave it alone, and say so
     - true but incomplete   write_memory with mode 'append' to bring it current
     - wrong now             rewrite it, and keep the correction visible: what
                             changed matters more than what it says today
     - no longer relevant    propose delete_memory, but ask me before calling it

Report as a short table - memory, age, verdict, what you did. Ask me about
anything you cannot settle from the code; a memory you guess at is worse than
one you flag.`, scope(args), days)
		},
	}}
}
