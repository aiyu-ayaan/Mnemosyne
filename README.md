# Mnemosyne

A local memory server for AI agents. Your agent stores what it learns about a
project and recalls it in the next session, instead of rediscovering it every
time.

Memories are Markdown files in a folder you choose. Mnemosyne indexes them for
search; it does not own them. Open them in any editor, put them in git, back
them up by copying a directory.

```markdown
---
id: 01JD3K7QW8ZX4N2P
title: Use FTS5, not vectors
tags: [decision, search]
created: 2026-09-12T22:40:11Z
updated: 2026-09-12T22:40:11Z
---

sqlite-vec is a loadable C extension, so adopting it means cgo or shipping
per-platform binaries. FTS5 is already compiled in and covers keyword search.
```

## Status

Early. The MCP server works and is the whole product today. The desktop app,
semantic search, the graph view, and encryption are planned — see
[`development/Todo.md`](development/Todo.md) for the roadmap and what is
deliberately not being built.

## Install

Download a build, then:

```bash
mnemosyne install          # copies the binary and adds it to your PATH
```

No admin rights needed. `--machine` installs for every account on the machine
and is the only thing that asks for elevation. `mnemosyne uninstall` reverses
it and never touches your memories.

### Portable

Unzip the portable archive and run it. It keeps its settings, index, and
memories inside its own folder and writes nothing anywhere else — no registry
keys, no PATH edits, nothing in your user profile.

```
mnemosyne-portable/
├── mnemosyne.exe
├── mnemosyne.portable     the marker that makes it portable
├── config.json
└── memories/
```

Any copy can be forced into portable mode with `--portable`.

### From source

Requires Go 1.27+ and [pnpm](https://pnpm.io).

```bash
pnpm install
pnpm build        # binary lands in bin/
pnpm test
```

## Connect it to an agent

One command, every agent on the machine:

```bash
mnemosyne agents install
```

```
Claude Code  MCP registered · hook installed
Codex        MCP registered · hook installed
Cursor       not installed on this machine — skipped
Windsurf     not installed on this machine — skipped
```

It registers the MCP server *and* the session-start hook, per client, and skips
anything not on the machine rather than inventing configuration for it.
`mnemosyne agents status` shows what is wired; `agents uninstall` takes back
exactly what it added. Restart a running client to pick it up.

By hand, if you would rather:

```bash
claude mcp add mnemosyne -- mnemosyne serve
codex mcp add mnemosyne -- mnemosyne serve
```

For any other MCP client the command is `mnemosyne serve` over stdio, and
`agents status` prints the JSON block to paste.

Then check it:

```bash
mnemosyne doctor
```

which prints where your memories live, which rule chose that location, and how
many you have.

## Make every session load it automatically

Two separate mechanisms, and it is worth knowing which is which.

**The MCP server** carries Mnemosyne's instructions to every client that
connects — Claude Code, Codex, Cursor, Windsurf, Zed, anything speaking MCP.
That is automatic once the server is registered, and it is advisory: an agent
reads it and may still not act on it.

**The session-start hook** is not advisory. Its output goes straight into the
agent's context, before your first message. `mnemosyne agents install` writes
it wherever a client supports one — today Claude Code and Codex.

Every new conversation then opens with the project slug for that directory and
the memories that already exist for it, in every repository, without you asking:

```
Mnemosyne holds this user's memory across sessions and across agents, at
C:\Users\you\AppData\Roaming\mnemosyne\memories.
The project slug for this directory is "mnemosyne".

It already holds 4:
  conventions — Conventions
  decisions — Decisions
  dev-log — Dev Log
  mcp-tooling — MCP Tooling
...
```

It covers Claude Code (`~/.claude/settings.json`) and Codex
(`~/.codex/hooks.json`) — the two clients with a session-start hook mechanism.
Each file is backed up before the first edit, only Mnemosyne's own entry is
added, and installing twice leaves one. Clients without hooks get the MCP
instructions and the prompts below.

To wire it up by hand instead, the command is `mnemosyne hook session-start`
and its stdout is the block to inject.

## First run: say this to your agent

There is no "create project" step and nothing to set up in your repository.
A project exists as soon as something is written to it, and `write_memory`
does that. If your agent asks where to put files, or starts looking for a
Mnemosyne folder in your repo, it has the wrong idea — paste this:

> Set up Mnemosyne for this repo. Call `list_projects` first; if this
> repository is not there, just `write_memory` with the project set to the
> repository's directory name — that creates it, there is no separate call and
> nothing goes in my working tree. Seed it from what this repo already
> documents: `conventions`, `decisions`, `todo`, `dev-log`, summarised in your
> own words rather than copied. Then `list_memories` and tell me what landed.

Clients that support MCP prompts have that built in — it is the **setup**
prompt, alongside **onboard** ("what do you already know about this project"),
**checkpoint** ("write down what we learned before I close this session"), and
**review-stale**. Claude Code lists them as `/mcp__mnemosyne__setup` and
friends; elsewhere, look for a prompt or slash picker. If your client has no
prompt support, the text above is the whole thing — paste it.

## Tools your agent gets

| Tool              | What it does                                              |
| ----------------- | --------------------------------------------------------- |
| `list_projects`   | Every project, with its memory count                       |
| `list_memories`   | A project's memories — titles and tags, not bodies         |
| `read_memory`     | One memory in full                                         |
| `write_memory`    | Create or update; the project is created if it is new      |
| `delete_memory`   | Delete permanently                                         |
| `search_memories` | Ranked full-text search, returning snippets                |
| `recall`          | Semantic + keyword ranking — the one to reach for first    |
| `read_backlinks`  | What else links to a memory                                |

Listing and search return metadata and snippets rather than whole memories, so
that having a lot of them stays cheap for the agent reading them.

Memories are also exposed as MCP **resources** (`mnemosyne://<project>/<slug>`),
so you can point at one yourself with your client's `@` picker instead of
hoping the agent goes looking.

## Where things are kept

| Purpose      | Path                                      |
| ------------ | ----------------------------------------- |
| Settings     | `<user config dir>/mnemosyne/config.json` |
| Memories     | `<user config dir>/mnemosyne/memories`    |

Change it with `mnemosyne root /path/you/want`, or per-run with `--root` or
`MNEMOSYNE_ROOT`.

Inside the memory root, each project is a directory and each memory is a
Markdown file. `.mnemosyne/index.db` is a derived SQLite index — delete it any
time and it rebuilds itself from the files on the next start.

## Commands

```
mnemosyne serve                 run the MCP server over stdio
mnemosyne agents install        load memory into every new agent session
mnemosyne hook session-start    the context block agents read (for hand-wiring)
mnemosyne doctor                show the current setup
mnemosyne root [path]           print or change the memory root
mnemosyne install [--machine]   install and add to PATH
mnemosyne uninstall             remove the binary and the PATH entry
mnemosyne version
```

## Development

The repo is a pnpm workspace. `apps/backend` is the Go module; `apps/desktop`
will be the Electron app.

Design notes live in [`development/devdocs/`](development/devdocs/):
[architecture](development/devdocs/architecture.md),
[storage format](development/devdocs/storage-format.md),
[MCP tools](development/devdocs/mcp-tools.md),
[deployment](development/devdocs/deployment.md),
[UI design](development/devdocs/ui-design.md), and a
[dev log](development/devdocs/dev-log.md) of what shipped when and why.

Commit conventions are in
[`development/Commit.md`](development/Commit.md).

## License

MIT. See [LICENSE](LICENSE).
