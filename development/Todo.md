# Mnemosyne — Development Roadmap

Living checklist for the whole product. Phase 1 is the MVP; everything after it
is scoped but deliberately deferred. See [`devdocs/architecture.md`](devdocs/architecture.md)
for why the phases are ordered this way, and [`devdocs/dev-log.md`](devdocs/dev-log.md)
for what actually shipped at each stage.

Legend: `[ ]` not started · `[~]` in progress · `[x]` done

---

## Phase 1 — MVP: a working MCP memory server

**Goal:** an AI agent (Claude Code, Codex, …) can store and recall per-project
memories through Mnemosyne, and the user can read and edit those memories as
plain Markdown files in a folder they chose.

That is the whole product value in one sentence. Everything else — GUI, graph
view, vectors, encryption — makes it nicer, not functional. So it comes later.

- [x] **1.1 Repo skeleton** — pnpm workspace, Go module under `apps/backend`, `.gitignore`, MIT `LICENSE`
- [x] **1.2 Storage layer** — Markdown + YAML frontmatter on disk, project = directory
  - [x] Memory read/write/delete, slug generation, frontmatter round-trip
  - [x] Project create/list/delete
  - [x] Path containment guard (no escaping the memory root)
  - [x] Unit tests
- [x] **1.3 Search index** — SQLite (`modernc.org/sqlite`, pure Go) + FTS5
  - [x] Schema, created on open
  - [x] Index sync on write/delete; stat-based reconcile on startup
  - [x] Ranked full-text search with snippets
  - [x] Unit tests
- [x] **1.4 MCP server** — stdio JSON-RPC via the official Go SDK
  - [x] Tools: `list_projects`, `list_memories`, `read_memory`, `write_memory`,
        `delete_memory`, `search_memories`
  - [x] Tool-level tests driving a real server over an in-memory transport
- [ ] **1.5 CLI + config** — `mnemosyne serve`, `mnemosyne doctor`, config file
      resolution (flag → env → config file → OS default dir)
- [ ] **1.6 Docs + release** — README, install and MCP wiring instructions for
      Claude Code and Codex, `devdocs` refresh

**MVP done when:** `claude mcp add mnemosyne -- mnemosyne serve` gives an agent
working memory that survives restarts, and the files are legible in any editor.

---

## Phase 2 — REST API + desktop shell

The GUI needs a transport; the transport needs a reason to exist. Both land here.

- [ ] **2.1 REST API** — same core, HTTP surface: projects, memories, search
- [ ] **2.2 SSE** — push index/file changes to connected clients
- [ ] **2.3 File watcher** — external edits (an agent, an editor, git) reindex live
- [ ] **2.4 Electron + React + TS + Vite + Tailwind shell**
- [ ] **2.5 VS Code–style layout** — see [`devdocs/ui-design.md`](devdocs/ui-design.md):
      activity bar, collapsible sidebar, editor tabs, bottom panel, status bar, dark-first
- [ ] **2.6 Memory editor** — Markdown editing with frontmatter form
- [ ] **2.7 Settings** — configurable memory root, matching the backend's config

---

## Phase 3 — Search that understands meaning

- [ ] **3.1 `sqlite-vec`** — vector table alongside FTS5 (needs cgo or a loadable
      extension; that build-system cost is exactly why it is not in the MVP)
- [ ] **3.2 Embeddings** — local model by default, pluggable provider
- [ ] **3.3 Hybrid ranking** — reciprocal-rank fusion over FTS5 + vector hits
- [ ] **3.4 `recall` MCP tool** — semantic entry point for agents

---

## Phase 4 — Graph view

- [ ] **4.1 Link parsing** — `[[wikilinks]]` and tags become edges
- [ ] **4.2 Backlinks** — stored in the index, exposed over API and MCP
- [ ] **4.3 React Flow graph** — Obsidian-style, click to open
- [ ] **4.4 CodeGraph integration** — bundle [colbymchenry/codegraph](https://github.com/colbymchenry/codegraph)
      (MIT) in the binary for codebase visualization; no separate install

---

## Phase 5 — Safety: encryption, backup, portability

- [ ] **5.1 At-rest encryption** — age/XChaCha20-Poly1305, passphrase-derived key
- [ ] **5.2 Key management** — OS keychain, explicit opt-in, clear recovery story
- [ ] **5.3 Backup + restore** — snapshot the memory root and index
- [ ] **5.4 Export / import** — portable archive between machines and users

---

## Phase 6 — Token accounting

- [ ] **6.1 Token counting** — per memory, per project
- [ ] **6.2 Usage tracking** — tokens spent per project
- [ ] **6.3 Savings estimate** — tokens served from memory instead of re-read context
- [ ] **6.4 GUI surfacing** — status bar and project overview

---

## Phase 7 — Distribution

- [ ] **7.1 Cross-platform builds** — Windows, macOS, Linux
- [ ] **7.2 Packaged desktop app** — backend binary bundled in the Electron app
- [ ] **7.3 CI** — build, test, release pipeline
- [ ] **7.4 Docusaurus site** — user-facing docs under `docs/`

---

## Explicitly not doing (yet)

Recorded so they stay decided rather than getting rediscovered every few weeks:

- **Multi-user / server hosting.** Mnemosyne is a local, single-user tool.
- **Cloud sync.** The memory root is a plain folder — Dropbox, Syncthing, or git
  already solve this better than we would.
- **A plugin system.** No second implementation of anything exists yet, so there
  is nothing to abstract over.
- **Postgres / pgvector.** The initial sketch mentioned it; SQLite covers a
  single-user desktop app completely, and one database is simpler than two.
