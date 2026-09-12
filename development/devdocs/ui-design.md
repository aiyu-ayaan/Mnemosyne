# UI design — VS Code shell

The desktop app (Phase 2) borrows the VS Code shell, because the audience is
developers who already have that layout in muscle memory, and because a memory
browser has exactly the same shape as a code editor: a narrow navigator, a
tabbed document area, and a panel underneath.

Borrows, not clones. The bones are VS Code's — the layout and the chords people
already know. The surface is Mnemosyne's: an accent rule on the active tab
rather than VS Code's, per-kind marks on the four convention slugs, and the
app's own palette. Someone who lives in VS Code should be able to use this
without learning anything; they should not mistake a screenshot of it for VS
Code.

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

| Icon      | Opens                                                        |
| --------- | ------------------------------------------------------------ |
| Explorer  | Sidebar: project tree — projects, memories, tags              |
| Search    | Sidebar: full-text and hybrid                                 |
| Graph     | Tab: canvas graph of links and backlinks                      |
| MCP       | Tab: copy-paste wiring for Claude Code, Codex, and others     |
| Theme     | Tab: Theme Studio, the one place a theme is chosen            |
| Settings  | Tab: memory root, diagnostics; later encryption and backup    |

Explorer and Search drive the sidebar and collapse when clicked again. The rest
open a tab, so they sit alongside whatever memory is open rather than replacing
it.

## Rules

- **Dark first.** Match the VS Code Dark Modern palette; ship a light theme, but
  design against dark.
- **Keyboard parity where the muscle memory already exists.** `Ctrl/Cmd+P` for
  quick-open a memory, `Ctrl/Cmd+Shift+P` for the command palette,
  `Ctrl/Cmd+Shift+F` for search, `Ctrl/Cmd+B` to toggle the sidebar,
  `Ctrl/Cmd+J` for the panel. Do not invent new chords for these.
- **One tab strip holds everything.** Memories and views — Graph, MCP, Theme,
  Settings — are one array with one active key, discriminated by `kind`.
- **The panel is for results, not navigation.** Search hits, backlinks, MCP
  status, server log.
- **Status bar carries the counters** — memory count and Phase 6's token savings.
- **One setting lives in one place.** Theme is Theme Studio's, reachable from the
  status bar and the palette; it is not also a second copy inside Settings.

## One tab strip

Memories and views used to be two parallel systems: a `tabs` array and a
`mainMode` enum. They could not coexist, so opening Settings hid every memory you
had open, and the tab bar carried four hand-copied blocks that could not be a
list because they were not in one.

`OpenTab` is now a discriminated union of `MemoryTab | ViewTab`. That single
change deleted the `mainMode` state machine, collapsed the four blocks into one
`map`, removed the tab bar's right-hand button strip (which duplicated the
activity bar two centimetres away), and took ~180 lines out of `App.tsx` and
`Tabs.tsx` between them. Settings now opens beside a memory, the way an editor
does it.

Callers that only mean something for a memory narrow through `asMemory` once,
rather than checking `kind` at every use.

## The command palette

`ui-design.md` used to rule the palette out under "what we are not copying". That
was wrong, and the reason is worth recording: the objection was to VS Code's
palette as a *general command runner* over an extension surface Mnemosyne does
not have. But the palette is also how a user finds an action they do not know
the chord for, and that problem exists here as soon as there is more than one
view.

It is the same overlay as quick-open, because in an editor it is the same
overlay: `>` switches modes, and the arrow keys, the filter, and Enter are shared.
Commands that need an open memory grey out rather than disappear, so the list
does not change shape as you move between tabs.

## Stack

React + TypeScript + Vite, Tailwind for styling, Electron for the shell. State
from the REST API with SSE for live updates, so an edit made by an agent appears
in the window without a refresh.

The graph is hand-rolled SVG with a small force simulation, not React Flow —
the plan named React Flow and the implementation never took the dependency.
Recorded because the doc claimed otherwise for a while, and because the choice
should be made deliberately if the graph grows: React Flow brings panning,
zooming, and node types worth having, at the cost of a large dependency for one
view.

## What we are not copying

The extension marketplace, the debugger, and the multi-root workspace model.
Borrow the layout and the shortcuts people already know; skip the machinery
underneath them.

The palette used to be on this list. See above for why it came off.
