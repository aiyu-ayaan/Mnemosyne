# Architecture

## The one idea

**Markdown files on disk are the source of truth. SQLite is a derived index.**

Everything else follows from that. If the index is deleted, `mnemosyne` rebuilds
it from the files and loses nothing. If Mnemosyne is uninstalled, the user still
has a folder of readable Markdown. Backup is copying a folder; export is zipping
it; sync is whatever the user already uses for folders.

The alternative — memories as rows in a database — would need bespoke export,
bespoke backup, bespoke diffing, and would make the files invisible to the
editors and agents the user already has. The file-first design deletes all four
problems instead of solving them.

## Layers

```
              MCP client (Claude Code, Codex)        Desktop GUI  [Phase 2]
                          │ stdio JSON-RPC                │ REST + SSE
                          ▼                               ▼
              ┌───────────────────────────────────────────────────┐
              │                  mnemosyne (Go)                   │
              │                                                   │
              │   mcp/          transport: tool defs, marshalling │
              │   http/         transport: REST + SSE   [Phase 2] │
              │        ─────────────────────────────────────────  │
              │   store/        the core: projects, memories,     │
              │                 search — no transport knowledge   │
              │        ─────────────────────────────────────────  │
              │   markdown/     files: frontmatter, slugs, paths  │
              │   index/        SQLite + FTS5, derived, throwaway │
              └───────────────────────────────────────────────────┘
                          │                               │
                          ▼                               ▼
                 <root>/<project>/*.md          <root>/.mnemosyne/index.db
```

Transports are thin. A tool handler in `mcp/` and a route handler in `http/`
both call the same `store` method and differ only in how they marshal. There is
one place where behaviour lives, so the two surfaces cannot drift apart.

## Why these choices

**Go for the backend.** One static binary, no runtime to install, and it can be
embedded in the Electron app later without asking the user to install anything.

**Pure-Go SQLite (`modernc.org/sqlite`).** No cgo. Cross-compiling to three
platforms from CI stays a `GOOS=… go build`, and Windows contributors do not
need a C toolchain. It ships FTS5, which is all the MVP's search needs.

**FTS5 now, `sqlite-vec` in Phase 3.** `sqlite-vec` is a loadable C extension:
adopting it means cgo or shipping per-platform `.so`/`.dll`/`.dylib` files, plus
an embedding model and its download. That is a real build-system project. FTS5
gives good keyword search today for zero added cost, and the Phase 3 work slots
in beside it rather than replacing it — hybrid ranking wants both anyway.

**MCP over stdio first.** It is the interface that makes the product useful. The
REST API exists to serve the GUI, and there is no GUI yet, so building it now
would be shipping an untested surface with no caller.

## Concurrency

One process owns the memory root. SQLite runs in WAL mode so reads never block
the writer. Writes go through the store, which serialises them per project. The
file watcher (Phase 2) is the only path for changes that did not originate here,
and it reconciles rather than assuming.

## Failure posture

The index is disposable, so index errors degrade rather than fail: a memory that
cannot be indexed is still written to disk and still readable by path, it just
will not appear in search until the next reconcile. File writes are the opposite
— they are atomic (write to a temp file in the same directory, then rename) and
a failure surfaces to the caller. Losing a memory is unacceptable; losing a
search result until the next startup is not.

## Repository layout

The repo is a **pnpm workspace**, so one command drives the Go backend and the
Electron frontend and contributors do not have to remember which tool builds
which half.

```
package.json          workspace scripts: build, test, lint, dev
pnpm-workspace.yaml   packages: apps/*
apps/
  backend/            Go module — package.json wraps go build/test/vet
    cmd/mnemosyne/    the binary
    internal/
      markdown/       frontmatter and slugs; one file in, one file out
      store/          the core: projects, memories, path safety, atomic writes
      index/          SQLite + FTS5
      mcpserver/      MCP tool definitions
  desktop/            Electron + React + Vite            [Phase 2]
development/          roadmap, dev docs, commit guidelines
```

`apps/backend/package.json` holds no npm dependencies — it exists so that
`pnpm build` and `pnpm test` at the root reach the Go toolchain through the same
workspace graph as the frontend.

| Command      | Effect                                              |
| ------------ | --------------------------------------------------- |
| `pnpm build` | Builds every package; the Go binary lands in `bin/` |
| `pnpm test`  | `go test ./...`, and the frontend suite once it exists |
| `pnpm lint`  | `go vet ./...`                                      |
| `pnpm dev`   | Runs the MCP server from source                     |

## Processes

Two processes run the same core, and neither is privileged over the other:

```
  agent (Claude Code, Codex)            desktop app
        │ spawns per session                  │ connects
        ▼                                     ▼
  mnemosyne serve                       mnemosyne daemon        [Phase 2]
  (MCP over stdio)                      (named pipe / unix socket)
        └──────────────┬──────────────────────┘
                       ▼
              the same memory root
```

There is no proxy layer and no exclusive owner. SQLite runs in WAL mode, memory
files are written through an atomic rename, and reconcile repairs drift — so
concurrent processes are safe by construction rather than by coordination.
Adding a proxy would mean a protocol, a fallback for when the daemon is absent,
and a new class of bug, all to protect an invariant the storage design already
holds.

Install modes, the logon autostart, the local channel, and portable mode are
specified in [`deployment.md`](deployment.md).
