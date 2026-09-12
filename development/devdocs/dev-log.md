# Dev log

Newest first. One entry per commit stage: what shipped, what it was verified
with, and any decision worth not re-litigating later.

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
