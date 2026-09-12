# Dev log

Newest first. One entry per commit stage: what shipped, what it was verified
with, and any decision worth not re-litigating later.

## Stage 5 — Portable mode, install, and PATH

`internal/install`, a reworked `internal/config`, and the README. MVP complete.

- `config.Detect` returns a `Locations` for the run: installed (user config dir)
  or portable (beside the binary). Root resolution is unchanged — flag, env,
  config file, default — only the default and the settings path move.
- `mnemosyne install` copies the binary to a per-user directory and adds it to
  PATH; `uninstall` reverses both. `--machine` is the only path needing admin.
- `doctor` now reports the mode and the binary directory too.

**Verified:** `go vet ./...` clean, `go test ./...` passing — 14 new config and
PATH tests. A portable copy was then built and run for real: it reported
portable mode, wrote its config, index, and memories inside its own folder and
nothing outside it, and refused `install` with the reason.

`install` was deliberately **not** run against this machine's registry. Editing
the user's PATH is theirs to trigger, not something to do while testing.

**Decisions taken here**

- *Autostart moved to Phase 2, with the daemon.* An autostart entry that
  launches a daemon nothing can talk to is a process that burns memory to do
  nothing. It waits for the client that gives it a purpose.
- *Admin does not mean a system service.* Reasoned through in
  [`deployment.md`](deployment.md): memories live in a user profile, so a
  boot-time SYSTEM service could not reach them without an impersonation layer.
  One per-user process model instead of two.
- *Shell profiles are never edited on Unix.* The binary is installed into a
  directory already on PATH; if it turns out not to be, the user is told the one
  line to add. Rewriting someone's `.zshrc` is fragile and presumptuous.
- *`addEntry` and `removeEntry` live in the Windows file.* A vet diagnostic
  showed them dead on Unix, where PATH is never rewritten. Code belongs beside
  the platform that uses it.
- *The install copy goes through a temp file and a rename, moving any existing
  binary aside first.* Windows will not overwrite or delete a running
  executable, so this is what lets an upgrade replace a binary that is in use.
- *`uninstall` never deletes memories.* Removing a program should not destroy
  the user's data, and there is no backup yet to undo it with.

## Stage 4 — CLI and configuration

`internal/config` and `cmd/mnemosyne`. The binary now runs.

- `mnemosyne serve` — MCP over stdio, the command an agent starts.
- `mnemosyne doctor` — resolved root, which rule chose it, index state, counts.
- `mnemosyne root [path]` — print or set the memory root.

**Verified end to end, not just in tests:** `pnpm build` produced `bin/mnemosyne.exe`,
and a Python MCP client drove the real binary over stdio through initialize,
tools/list, two writes, a ranked search, list_projects, a read, and a
deliberate miss. The files it wrote were then checked on disk.

**Decisions taken here**

- *The settings file lives outside the memory root.* It is what says where the
  root is, so it cannot live inside it. Settings and the default root now share
  one directory under the user config dir, identical on every platform, instead
  of three per-OS special cases.
- *`doctor` reports which rule chose the root.* A surprising root should be
  explainable, not just stated.
- *Formatting matches the documented shape, and a test pins the exact bytes.*
  The smoke test showed block-style tag lists and quoted timestamps where the
  docs promised inline lists and bare ones. These files are edited by hand and
  printed in the docs, so the layout is part of the product; `Format` now builds
  a yaml mapping node to control field order and list style.
- *All logging goes to stderr.* Stdout carries JSON-RPC and nothing else.

## Stage 3 — MCP server

`internal/mcpserver` exposes the store over MCP using the official Go SDK, with
the six tools specified in [`mcp-tools.md`](mcp-tools.md).

Handlers are typed, so the SDK infers each tool's JSON schema from a Go struct,
validates arguments before the handler runs, and turns a returned error into a
tool error rather than a protocol error. Every handler marshals arguments, calls
exactly one store method, and marshals the result.

**Verified:** `go vet ./...` clean, `go test ./...` passing — 11 MCP tests that
drive a real server over an in-memory transport the way an agent would, covering
the round trip, tag filtering, ranked search, clamped limits, deletion,
cancellation, and rejected traversal.

**Decisions taken here**

- *Nil slice means "leave alone", empty slice means "clear".* JSON omission and
  `[]` already differ this way after unmarshalling, so the distinction the store
  needs comes for free rather than needing a flag.
- *Out-of-range limits are clamped, not rejected.* The agent guessed a number;
  failing the call over it helps nobody.
- *A reference is only scanned for as an id when it is ULID-shaped.* Found while
  writing the error-message test: any non-slug reference used to fall through to
  a scan of every file in the project, so a typo cost a full read of the project
  and then reported a vague "not found". `markdown.LooksLikeID` now separates the
  two cases, making the common mistake both cheap and clearly explained.

## Stage 2 — Search index

`internal/index` (SQLite + FTS5) and its wiring into the store.

- Schema: a `memories` table for metadata and file stat values, plus an
  `memories_fts` virtual table over title, tags, and body, joined by rowid.
- `Store.Open` now opens the index and reconciles it; `Store.Close` releases it.
- Writes and deletes update the index in place. `Store.Search` returns ranked
  hits with snippets.

**Verified:** `go vet ./...` clean, `go test ./...` passing — 12 new search and
reconcile tests, including an index deleted from under a live root rebuilding
itself from the files.

**Decisions taken here**

- *`index` must not import `store`.* The store owns the index and converts its
  own types into `index.Record`; the reverse would be an import cycle.
- *Index failures on write are logged, not returned.* The memory is already on
  disk. Failing a write that succeeded would be a worse lie than a search result
  that is missing until the next reconcile.
- *Reconcile compares mtime and size.* Startup then costs time proportional to
  what changed rather than to how much is stored.
- *Invalid FTS5 syntax is retried as quoted phrases.* An agent searching for
  `C++` should get results, not a lecture about a query language it never saw.
- *A query of pure punctuation is an error, not zero results.* Caught while
  writing the test for `*`: returning an empty list would read as "no memory
  matches" and send the caller looking for content that was never searched for.
- *Tests close the store.* Windows will not delete an open database file, so
  `t.Cleanup` closing the index is what makes `t.TempDir` cleanup work.

## Stage 1 — Workspace and storage layer

The repo became a pnpm workspace and grew its first two packages.

- `pnpm-workspace.yaml` + root `package.json` — `build`, `test`, `lint`, `dev`
  fan out to every package. The Go module lives at `apps/backend` and is driven
  through its own `package.json` script wrapper, so one command covers both
  halves of the product.
- `internal/markdown` — frontmatter parse/format, slug generation, slug validation.
- `internal/store` — projects, memories, path safety, atomic writes.

**Verified:** `go vet ./...` clean, `go test ./...` passing — 10 markdown tests
and 14 store tests, covering frontmatter round-trips, CRLF input, slug collisions,
tag normalisation, lookup by ULID, and rejected path traversal.

**Decisions taken here**

- *pnpm drives the backend too.* The Go package.json carries no dependencies; it
  exists so `pnpm test` at the root reaches Go through the same workspace graph
  as the frontend will.
- *Frontmatter keys we do not define are preserved.* Another tool writing a key
  into a memory file is not ours to discard.
- *A directory without `project.json` is still a project.* Dropping a folder of
  Markdown into the memory root should just work; the metadata file is written
  on first access instead of being demanded up front.
- *Tags and links are pointers in `WriteRequest`.* Otherwise "leave these alone"
  and "clear these" are the same call, and an update that omits tags would
  silently erase them.
- *Timestamps are truncated to whole seconds.* Caught by a test: RFC 3339
  frontmatter stores seconds, so an untruncated value returned from a write did
  not match what the next read produced. The file is the source of truth, so the
  in-memory value bends to it.
- *Windows needs the target removed before rename.* `os.Rename` will not replace
  an existing file there, so the temp file is fsynced before that window opens.

## Stage 0 — Planning and dev docs

Wrote the roadmap and the design docs before any code.

- [`Todo.md`](../Todo.md) — seven phases; Phase 1 is the MVP.
- [`architecture.md`](architecture.md) — files are truth, SQLite is a derived index.
- [`storage-format.md`](storage-format.md) — on-disk layout, frontmatter schema, path safety.
- [`mcp-tools.md`](mcp-tools.md) — the six MVP tools.
- [`ui-design.md`](ui-design.md) — VS Code shell for the Phase 2 desktop app.

**Decisions taken here**

- *MVP is the MCP server, not the GUI.* The GUI needs the REST API, the REST API
  needs the core, and the core is what makes the product useful on its own. An
  agent with working memory is shippable; a window with nothing behind it is not.
- *FTS5 instead of `sqlite-vec` for now.* `sqlite-vec` is a loadable C extension,
  so it drags in cgo or per-platform binaries plus an embedding model. Phase 3
  wants hybrid ranking over both indexes anyway, so nothing is thrown away.
- *No Postgres/pgvector.* The initial sketch mentioned it. One database is
  simpler than two, and SQLite covers a single-user desktop app completely.
- *Verified before committing to them:* Go 1.27.1 toolchain, and both
  `modernc.org/sqlite` and `github.com/modelcontextprotocol/go-sdk` resolve.
