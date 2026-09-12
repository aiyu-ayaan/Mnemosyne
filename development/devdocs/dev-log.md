# Dev log

Newest first. One entry per commit stage: what shipped, what it was verified
with, and any decision worth not re-litigating later.

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
