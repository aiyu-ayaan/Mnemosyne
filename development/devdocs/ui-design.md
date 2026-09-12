# UI design — VS Code shell

The desktop app (Phase 2) copies the VS Code shell, because the audience is
developers who already have that layout in muscle memory, and because a memory
browser has exactly the same shape as a code editor: a narrow navigator, a
tabbed document area, and a panel underneath.

## Layout

```
┌──┬──────────────────┬──────────────────────────────────────────────┐
│  │ SIDEBAR          │ ◇ memory-a.md  × │ ◇ memory-b.md × │         │  tabs
│A │                  ├──────────────────────────────────────────────┤
│C │ ▾ MNEMOSYNE      │                                              │
│T │   ▾ decisions    │   # Use FTS5, not vectors                    │
│I │     use-fts5     │                                              │
│V │     file-first   │   sqlite-vec is a loadable C extension…      │  editor
│I │   ▸ conventions  │                                              │
│T │                  │                                              │
│Y │ ▾ TAGS           ├──────────────────────────────────────────────┤
│  │   #decision  12  │ SEARCH │ BACKLINKS │ MCP │ OUTPUT            │  panel
│  │   #gotcha     4  │ 3 results for "sqlite"                       │
├──┴──────────────────┴──────────────────────────────────────────────┤
│ ⎇ mnemosyne   ⚙ 142 memories   ◷ 38k tokens saved            UTF-8 │  status
└─────────────────────────────────────────────────────────────────────┘
```

## Activity bar

| Icon      | View                                                        |
| --------- | ----------------------------------------------------------- |
| Explorer  | Project tree — projects, memories, tags                      |
| Search    | Full-text now, semantic in Phase 3                           |
| Graph     | React Flow graph of links and backlinks (Phase 4)            |
| MCP       | Copy-paste wiring commands for Claude Code, Codex, and others|
| Settings  | Memory root, encryption, backup                              |

## Rules

- **Dark first.** Match the VS Code Dark Modern palette; ship a light theme, but
  design against dark.
- **Keyboard parity where the muscle memory already exists.** `Ctrl/Cmd+P` for
  quick-open a memory, `Ctrl/Cmd+Shift+F` for search, `Ctrl/Cmd+B` to toggle the
  sidebar, `Ctrl/Cmd+J` for the panel. Do not invent new chords for these.
- **Editor tabs are memories.** Dirty state, close, and reopen behave as expected.
- **The panel is for results, not navigation.** Search hits, backlinks, MCP
  status, server log.
- **Status bar carries the counters** — memory count and Phase 6's token savings.

## Stack

React + TypeScript + Vite, Tailwind for styling, React Flow for the graph,
Electron for the shell. State from the REST API with SSE for live updates, so an
edit made by an agent appears in the window without a refresh.

## What we are not copying

The command palette as a general command runner, the extension marketplace, the
debugger, and the multi-root workspace model. Borrow the layout and the
shortcuts people already know; skip the machinery underneath them.
