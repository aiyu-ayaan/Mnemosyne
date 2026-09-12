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
        `delete_memory`, `search_memories`, `recall`, `read_backlinks`
  - [x] Tool-level tests driving a real server over an in-memory transport
  - [x] Discoverability: server `instructions`, tool titles and annotations, so
        agents reach for Mnemosyne without being told to
  - [x] `mnemosyne tools` / `mnemosyne call` for driving tools by hand
- [x] **1.5 CLI + config** — `serve`, `doctor`, `root`, `version`; root
      resolution (flag → env → config file → OS default dir)
- [x] **1.6 Portable mode + install** — see [`devdocs/deployment.md`](devdocs/deployment.md)
  - [x] Portable mode: `mnemosyne.portable` marker or `--portable`, everything
        beside the binary, nothing written outside its own folder
  - [x] `mnemosyne install` / `uninstall`, per-user by default, no admin needed
  - [x] PATH registration: `HKCU\Environment` on Windows, `~/.local/bin` on
        Unix; `--machine` for the system PATH, the only path needing admin
  - Logon autostart moved to Phase 2, where the daemon it would start exists
- [x] **1.7 Docs** — README with install, portable, MCP wiring, and the tool table

**MVP done when:** `claude mcp add mnemosyne -- mnemosyne serve` gives an agent
working memory that survives restarts, and the files are legible in any editor.

---

## Phase 2 — Daemon, local channel, and the desktop shell

The GUI needs a transport; the transport needs a reason to exist. The daemon
needs a client. All three land together — a background service with nothing able
to talk to it is a process that burns memory and does nothing.

- [x] **2.1 API surface** — same core, JSON: projects, memories, search, settings,
      health; `internal/api`, with `ErrInvalid` so a bad slug is a 400, not a 500
- [x] **2.2 Local channel** — named pipe on Windows, unix socket elsewhere; no
      TCP port, OS permissions as the boundary, token file as defence in depth
- [x] **2.3 `mnemosyne daemon`** — long-running, serves the channel, per-user
- [x] **2.4 Logon autostart** — Scheduled Task, LaunchAgent, or `systemd --user`,
      plus `service status|start|stop|install|uninstall` from the CLI;
      `mnemosyne install` registers it, `uninstall` removes it
- [x] **2.5 Change events** — `internal/events` bus, pushed over SSE at `/v1/events`
- [x] **2.6 File watcher** — a reconcile ticker in the daemon, so an external edit
      reindexes and reaches connected clients as the same event a write would
- [x] **2.7 Electron + React + TS + Vite + Tailwind shell** — `apps/desktop`; the
      main process bridges the channel, the renderer has no Node access at all
- [x] **2.8 VS Code–style layout** — see [`devdocs/ui-design.md`](devdocs/ui-design.md):
      activity bar, collapsible sidebar, editor tabs, bottom panel, status bar, dark-first,
      plus `Ctrl+P`, `Ctrl+Shift+F`, `Ctrl+B`, `Ctrl+J`, `Ctrl+S`, `Ctrl+W`
- [x] **2.9 Memory editor** — title, tags, and links as a form over a Markdown
      body; dirty tabs, discard confirmation, delete confirmation
- [x] **2.10 Settings** — memory root with a native picker, applied live by the
      daemon; theme toggle; installation details

---

## Phase 3 — Search that understands meaning

- [x] **3.1 `sqlite-vec` / vector table** — vector table alongside FTS5 (pure-Go cosine search over normalised float32 blobs, cascade delete, content hashing)
- [x] **3.2 Embeddings** — local model (Ollama) by default, pluggable provider (OpenAI-compatible, none), unit normalisation
- [x] **3.3 Hybrid ranking** — reciprocal-rank fusion over FTS5 + vector hits, async embedding on write, batch reconcile
- [x] **3.4 `recall` MCP tool** — semantic entry point for agents (`recall`), `/v1/embeddings` endpoint, hybrid search in desktop UI

---

## Phase 4 — Graph view

- [x] **4.1 Link parsing** — `[[wikilinks]]` and tags become edges
- [x] **4.2 Backlinks** — stored in the index, exposed over API and MCP
- [x] **4.3 React Flow graph** — Obsidian-style canvas graph, click to open
- [x] **4.4 CodeGraph integration** — bundle [colbymchenry/codegraph](https://github.com/colbymchenry/codegraph)
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
- [ ] **7.2 Portable archives** — zip/tarball per platform, marker file included
- [ ] **7.3 Packaged desktop app** — backend binary bundled in the Electron app,
      installer calling the same `mnemosyne install` the CLI exposes
- [ ] **7.4 CI** — build, test, release pipeline
- [ ] **7.5 Docusaurus site** — user-facing docs under `docs/`

---

## Explicitly not doing (yet)

Recorded so they stay decided rather than getting rediscovered every few weeks:

- **Multi-user / server hosting.** Mnemosyne is a local, single-user tool.
- **A boot-time system service.** Memories live in a user profile, so a service
  running as SYSTEM before login could not reach them without an impersonation
  layer. The daemon is per-user and starts at logon in every install mode; see
  [`devdocs/deployment.md`](devdocs/deployment.md).
- **A loopback TCP port.** The local channel uses a named pipe or a unix socket,
  so there is no port to firewall, collide with, or reach from the LAN.
- **Editing shell profiles.** PATH is handled with a symlink into a directory
  already on PATH, or the Windows registry. Dotfiles belong to the user.
- **Cloud sync.** The memory root is a plain folder — Dropbox, Syncthing, or git
  already solve this better than we would.
- **A plugin system.** No second implementation of anything exists yet, so there
  is nothing to abstract over.
- **Postgres / pgvector.** The initial sketch mentioned it; SQLite covers a
  single-user desktop app completely, and one database is simpler than two.
