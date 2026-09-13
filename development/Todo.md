# Mnemosyne — Development Roadmap

Living checklist for the whole product. Phase 1 is the MVP; everything after it
is scoped but deliberately deferred. See [`devdocs/architecture.md`](devdocs/architecture.md)
for why the phases are ordered this way, and [`devdocs/dev-log.md`](devdocs/dev-log.md)
for what actually shipped at each stage.

Legend: `[ ]` not started · `[~]` in progress · `[x]` done

---

## What's next

Phases 1–4 are done. The next three pieces of work, in the order they should be
picked up and with the reason each is where it is:

**1. Finish the desktop shell — 2.13 to 2.17.** The shell is structurally right
now (one tab strip, a palette), but the editor is still a form over a textarea
with a from-scratch Markdown renderer inside it. Start with **2.13**: roughly
240 of `Editor.tsx`'s 646 lines reimplement `marked`, badly — no code
highlighting, no tables — and replacing them is a net deletion. Then **2.16**,
the Trash view, because Stage 13 made deletes recoverable on disk and nothing in
the GUI can yet recover one. **2.14** and **2.15** are polish and can slip.

**2. Phase 6 — token accounting.** This is the differentiator. Every competitor
claims to save context; none shows the user the number, and Mnemosyne already
has the event bus it needs. Do **6.5** (the audit trail) first: recalls are
reads, so nothing publishes them today, which means there is no honest way to
compute 6.3's savings estimate — and "which memories are actually being
recalled" is worth seeing on its own.

**3. Phase 5 — encryption, backup, export.** Deliberately after 6: the memory
root is plain Markdown in a folder the user chose, so git or Syncthing already
covers most of the risk, and export/import matters more than at-rest encryption
for a local single-user tool. Do **5.3/5.4** before **5.1/5.2**.

**Interrupted by, and now done:** development mode, the console window the logon
task left on the desktop, the MCP panel handing out a registration command that
pointed at a different memory root than the panel described, the
session-start hook that loads memory into every new agent conversation, and
first-class CLI `--dev` flag routing. Stages 15 to 21 in the dev log. None of it
changed the order above.

**Not next, on purpose.** Phase 3.5's retrieval work (reranking, entity
matching) is blocked on **3.5.4**, an evaluation set. There is no way to tell
whether any ranking change helps without a fixture of questions and the memories
that should answer them, and shipping ranking changes on vibes is how a search
system gets quietly worse.

Phase 3.6 is not blocked on that and can be picked up whenever a library has
grown messy enough to want it — **3.6.2** (`supersedes` links) is the smaller
of the two and the one that makes recall honest about corrections.

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
  - [x] `write_memory` `mode`: `append` / `prepend`, so adding a line to `todo`
        or `dev-log` stops costing a `read_memory` plus the whole body back
  - [x] Resources — every memory at `mnemosyne://<project>/<slug>`, kept live
        off the change bus, so a user can `@`-mention one in their client
  - [x] Prompts — `setup`, `checkpoint`, `onboard`, `review-stale`: user-invoked,
        so they cost nothing per session unlike `instructions`
  - [x] An entry point for an agent that has never seen Mnemosyne — `setup`,
        two sentences in `instructions`, and a not-found error that names the
        write that would have worked. An agent told to "put the docs in
        Mnemosyne" otherwise goes looking for a folder in the working tree
  - [x] `age` on every hit, and the instruction to distrust an old fact
  - [x] `similar` on create — the near-duplicates that already exist
  - [x] Delete moves to `.mnemosyne/trash/` instead of unlinking
- [x] **1.5 CLI + config** — `serve`, `doctor`, `root`, `version`; root
      resolution (flag → env → config file → OS default dir)
- [x] **1.6 Portable mode + install** — see [`devdocs/deployment.md`](devdocs/deployment.md)
  - [x] Portable mode: `mnemosyne.portable` marker or `--portable`, everything
        beside the binary, nothing written outside its own folder
  - [x] `mnemosyne install` / `uninstall`, per-user by default, no admin needed
  - [x] PATH registration: `HKCU\Environment` on Windows, `~/.local/bin` on
        Unix; `--machine` for the system PATH, the only path needing admin
  - [x] Development mode — `MNEMOSYNE_DEV=1` gives a checkout its own config,
        memory root, runtime directory and channel, and refuses `install`,
        `service install` and `agents install`. A dev build can never take the
        installed binary's PATH entry or register itself at logon
  - [x] **`mnemosyne agents` — one entry point for every AI client.**
        `install|status|uninstall` does both halves per client: registers the
        MCP server (`~/.claude.json`, `codex mcp add`, `~/.cursor/mcp.json`,
        Windsurf's `mcp_config.json`) and writes `mnemosyne hook session-start`
        wherever a session-start hook exists, so a new conversation opens with
        the project slug and what is already stored for it. Clients that are not
        on the machine are skipped; anything else speaking MCP gets the JSON
        `agents status` prints. See the note in "Explicitly not doing" about
        editing someone else's config
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
      `mnemosyne install` registers it, `uninstall` removes it. The daemon frees
      a console it is the only process attached to, so the logon task does not
      leave a black window on the desktop for the session
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
      daemon; installation details. Theme lives only in Theme Studio
- [x] **2.11 One tab strip** — `MemoryTab | ViewTab`, so Settings and the graph
      open beside a memory instead of replacing every open tab
- [x] **2.12 Command palette** — `Ctrl+Shift+P`, sharing quick-open's overlay

### Phase 2 — still open

- [ ] **2.13 Drop the hand-rolled Markdown renderer** — ~240 of `Editor.tsx`'s
      646 lines are a from-scratch renderer with no code highlighting and no
      tables. `marked` + `DOMPurify`, keeping the `[[wikilink]]` extension,
      which is the one part that is genuinely ours
- [ ] **2.14 Custom title bar** — `◈ Mnemosyne`, a `project › memory`
      breadcrumb, window controls. The biggest remaining "not VS Code" signal
- [ ] **2.15 Resizable sidebar and panel** — both are a fixed `w-64` today
- [ ] **2.16 Trash in the Explorer** — restore or purge what `delete_memory`
      moved aside (Stage 13). Restoring is a human decision, which is why there
      is no MCP tool for it
- [ ] **2.17 Activity panel** — a fourth panel tab fed by the existing SSE
      stream: what agents wrote, updated, and deleted, live

---

## Phase 3 — Search that understands meaning

- [x] **3.1 `sqlite-vec` / vector table** — vector table alongside FTS5 (pure-Go cosine search over normalised float32 blobs, cascade delete, content hashing)
- [x] **3.2 Embeddings** — local model (Ollama) by default, pluggable provider (OpenAI-compatible, none), unit normalisation
- [x] **3.3 Hybrid ranking** — reciprocal-rank fusion over FTS5 + vector hits, async embedding on write, batch reconcile
- [x] **3.4 `recall` MCP tool** — semantic entry point for agents (`recall`), `/v1/embeddings` endpoint, hybrid search in desktop UI

---

## Phase 3.5 — Retrieval quality

Scoped against the 2026 state of the art rather than invented: the published
comparisons ([mem0's State of AI Agent Memory 2026](https://mem0.ai/blog/state-of-ai-agent-memory-2026),
[Zep/Graphiti](https://arxiv.org/abs/2501.13956)) agree on which parts of a
memory system move recall numbers, and Mnemosyne has the first two of three
retrieval signals.

- [ ] **3.5.1 Entity matching** — a third signal beside semantic and keyword.
      The reported gains are concentrated in multi-hop questions, which is what
      "why did we do it this way" actually is
- [ ] **3.5.2 Reranking** — a second-pass ordering over the fused candidates.
      Worth measuring before adopting: it costs a model call per recall, and the
      whole point of Mnemosyne is that it runs locally and for free
- [ ] **3.5.3 Async writes** — embedding already happens off the write path;
      make the same true of reconcile so a large import cannot stall a call
- [ ] **3.5.4 An evaluation set** — the real gap. There is no way to tell
      whether any of the above helps without a fixture of questions and the
      memories that should answer them. This lands before 3.5.1, not after

## Phase 3.6 — Memory hygiene

Two entries moved out of "explicitly not doing" once the question was put
properly. Neither adds a data model or an inference budget: both are the
existing "agent proposes, user decides" shape applied to memories that have
already accumulated.

- [ ] **3.6.1 A `consolidate` prompt** — user-invoked, like `review-stale`, so
      it costs nothing per session. Recall across a project, surface the clusters
      that overlap, and propose a merge per cluster: which memory survives, what
      gets appended to it, what gets deleted. Nothing is written without the
      user saying yes. `similar` on create is the same idea at write time; this
      is it for a library that was written before `similar` existed. The
      temptation to skip the confirmation is the whole reason this sat in
      "not doing" — it must not be a tool an agent can call unattended
- [ ] **3.6.2 `supersedes` links** — one frontmatter field naming what a memory
      replaced. Recall can then say "this corrected an earlier note" instead of
      leaving two contradictory memories at different ages, `read_backlinks`
      walks the chain, and the graph draws it as an edge it already understands.
      No second store, no invalidation engine, no bi-temporal query language:
      a link, in a file the user can read

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

The differentiator. Every competitor claims to save context; none of them shows
the user the number. Mnemosyne already has the event bus this needs.

- [ ] **6.1 Token counting** — per memory, per project
- [ ] **6.2 Usage tracking** — tokens spent per project
- [ ] **6.3 Savings estimate** — tokens served from memory instead of re-read context
- [ ] **6.4 GUI surfacing** — status bar and project overview
- [ ] **6.5 Audit trail** — which memories were actually recalled, and by what.
      Recalls are reads, so nothing publishes them today. Memory you cannot
      audit is memory you cannot trust, and this is also what makes 6.3 honest
      rather than a guess

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
  `mnemosyne agents install` is the one thing that writes into a file the user
  owns, and it is not an exception to the rule so much as the same rule applied:
  it is an explicit command rather than something `install` does behind them, it
  backs the file up first, it adds only its own entry, and `agents uninstall`
  restores the document byte for byte. Nothing is ever rewritten that does not
  parse.
- **Cloud sync.** The memory root is a plain folder — Dropbox, Syncthing, or git
  already solve this better than we would.
- **A plugin system.** No second implementation of anything exists yet, so there
  is nothing to abstract over.
- **A large tool surface.** The nearest competitor ships 54 MCP tools; that is a
  feature table, not a design. Every tool is a choice an agent pays to consider
  on every call, so the surface grows sideways into resources and prompts, which
  cost nothing to ignore.
- **LLM-driven consolidation — resolved, see 3.6.1.** Automatic summarising or
  merging is still refused: it needs an inference budget a local tool does not
  have, and it silently rewrites what the user wrote. But "never automatic" is
  not the same as "never", and the deferral was hiding that. What ships instead
  is the same shape as every other judgement call here — the agent proposes, the
  user decides. `similar` on create already covers the duplicate at the moment
  it would be created; **3.6.1** covers the ones that accumulated before it.
- **A temporal knowledge graph — resolved, see 3.6.2.** A second data model for
  bi-temporal edge invalidation (Zep/Graphiti-style) is still refused: that is
  the right answer at conversation scale, and Mnemosyne's unit is a Markdown file
  a human edits. What the deferral was throwing away with it is the one part
  that earns its keep at file scale — knowing *what a fact replaced*. `age` and
  `review-stale` tell you a memory is old; neither tells you it was superseded,
  or by what. **3.6.2** records that as a link, which the graph already draws.
- **Postgres / pgvector.** The initial sketch mentioned it; SQLite covers a
  single-user desktop app completely, and one database is simpler than two.
