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

```bash
claude mcp add mnemosyne -- mnemosyne serve
```

For Codex or any other MCP client, the command is `mnemosyne serve` over stdio.

Then check it:

```bash
mnemosyne doctor
```

which prints where your memories live, which rule chose that location, and how
many you have.

## Tools your agent gets

| Tool              | What it does                                              |
| ----------------- | --------------------------------------------------------- |
| `list_projects`   | Every project, with its memory count                       |
| `list_memories`   | A project's memories — titles and tags, not bodies         |
| `read_memory`     | One memory in full                                         |
| `write_memory`    | Create or update; the project is created if it is new      |
| `delete_memory`   | Delete permanently                                         |
| `search_memories` | Ranked full-text search, returning snippets                |

Listing and search return metadata and snippets rather than whole memories, so
that having a lot of them stays cheap for the agent reading them.

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
