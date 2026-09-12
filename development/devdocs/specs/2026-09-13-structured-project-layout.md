# Structured project layout

**Status:** approved, not yet implemented
**Date:** 2026-09-13

Three changes that together turn a project from a flat bag of Markdown into a
shaped workspace: memories nest in folders, memories cite code, and a new
project arrives with its base files already written.

They are specified together because they share one seam — the memory
*reference* — and splitting them would mean changing that seam twice.

---

## 1. Nested memory paths

### What changes

A memory reference becomes a `/`-separated path of slugs rather than a single
slug. `dev/architecture` and `env/prod` are memories; so is `conventions`.

```
memories/mnemosyne/
├── project.json
├── todo.md
├── decisions.md
├── conventions.md
├── dev-log.md
├── dev/
│   ├── architecture.md
│   └── storage-format.md
└── env/
    ├── local.md
    └── prod.md
```

Nesting is additive. Every flat memory that exists today keeps working with no
migration, and a project that never nests looks exactly as it does now.

### `markdown`

```go
// MaxPathDepth caps how deep a memory may nest. Four is enough for
// env/prod or dev/adr/0001 and shallow enough that a path stays readable.
const MaxPathDepth = 4

// ValidPath reports whether ref is a well-formed memory path: one to
// MaxPathDepth slug segments separated by "/".
func ValidPath(ref string) bool
```

`ValidPath` is `ValidSlug` applied per segment, so it rejects `dev/`, `dev//a`,
`../x` and an empty ref by construction. `ValidSlug` keeps its current meaning
and remains the only thing that touches the regex.

`Slugify` is unchanged: it produces one segment. A caller wanting a section
names it (`memory: "dev/architecture"`) rather than passing a separate field —
one identifier, not two.

### `store`

| Function | Change |
| --- | --- |
| `memoryPath` | `ValidPath` instead of `ValidSlug`; `filepath.FromSlash` before the join. Containment check unchanged. |
| `memorySlugs` | `filepath.WalkDir` instead of `os.ReadDir`. Returns `/`-joined relative paths via `filepath.ToSlash`. Still skips `.`-prefixed names, now at any depth. |
| `WriteMemory` | `os.MkdirAll(filepath.Dir(path))` before the atomic write. `ValidSlug(req.Memory)` at :180 becomes `ValidPath`. |
| `WriteMemory` :168 | Currently `filepath.Join(dir, slug+memoryExt)`, bypassing the containment guard. Must call `memoryPath`. |
| `resolveRef` :266, :290 | The same two fixes: `ValidPath`, and `memoryPath` instead of a raw join. |
| `DeleteMemory` | After removing the file, prune parent directories that are now empty, stopping at the project directory. |
| `uniqueSlug` | Collision suffixes apply to the final segment only. |

Routing both remaining raw joins through `memoryPath` is the part of this
section that matters most: it is the containment guard, and today two call
sites skip it.

### `index`

No schema change. `slug` is already `TEXT`, and a graph node id is
`project + "/" + slug`, which stays unique when the slug contains a slash.

### REST — a forced route change

Go's `ServeMux` matches `{memory}` against exactly one path segment, and a
multi-segment `{memory...}` must be the final segment of a pattern. The
existing backlinks route sits after the memory wildcard, so both cannot stand:

```go
// before
GET /v1/projects/{project}/memories/{memory}
GET /v1/projects/{project}/memories/{memory}/backlinks

// after
GET    /v1/projects/{project}/memories/{memory...}
PUT    /v1/projects/{project}/memories/{memory...}
DELETE /v1/projects/{project}/memories/{memory...}
GET    /v1/backlinks?project={project}&memory={memory}
```

Backlinks moves to a query-parameter route. `apps/desktop` is the only caller
and is updated in the same commit. This is the one breaking change in the spec,
and it is forced rather than chosen.

### MCP

No signature changes. `project` and `memory` arguments already take strings;
they now accept a path. Tool descriptions say so, and the server `instructions`
gain the section convention described below.

---

## 2. Code references

### What changes

A memory can cite code, and the citation is a graph edge rather than prose.

```yaml
---
title: Why WriteMemory upserts
tags: [decision]
links: [storage-is-file-first]
code:
  - apps/backend/internal/store/memory.go#WriteMemory
  - apps/backend/internal/mcpserver/server.go#writeMemory
---
```

`code:` already round-trips today, but as an opaque entry in `Memory.Extra`:
nothing reads it, indexes it, or draws it. This promotes it to a known key.

A ref is `<path>#<symbol>`, with `#<symbol>` optional. The path is
repo-relative, and Mnemosyne does not resolve it itself — CodeGraph does, which
is why the format is what `codegraph query` already accepts.

### `markdown`

`Memory.Code []string`, parsed with the existing `stringSlice`, added to
`known` so it leaves `Extra`, and written by `Format` immediately after `links`
in the fixed key order.

### `index`

```sql
CREATE TABLE IF NOT EXISTS code_refs (
  project TEXT NOT NULL,
  slug    TEXT NOT NULL,
  ref     TEXT NOT NULL,
  PRIMARY KEY (project, slug, ref)
);
CREATE INDEX IF NOT EXISTS code_refs_ref ON code_refs(ref);
```

Written and cleared alongside `links`, in the same transaction, so a reindex or
a delete cannot leave one populated and the other stale.

`GraphNode` gains `Kind string` (`"memory"` or `"code"`). `Graph()` emits one
synthetic node per distinct ref, id `code:<ref>`, plus an edge from each citing
memory. Memories and symbols land on one canvas, which is the point of storing
the ref at all.

New: `Index.MemoriesByCodeRef(project, ref) ([]Hit, error)`.

### Surfaces

| Surface | Change |
| --- | --- |
| `store.Meta` | `Code []string` |
| `store.WriteRequest` | `Code *[]string` — a pointer, like `Tags` and `Links`, so "leave alone" and "clear" stay distinguishable |
| MCP `write_memory` | `code` argument |
| MCP `read_memory` | returns `code` |
| MCP `list_memories` | `code` filter — answers "what do I know about `WriteMemory`?" |
| REST | `code` in the memory JSON; `GET /v1/codegraph/refs?project=` |

`/v1/codegraph/refs` runs each distinct ref through the existing `codegraph
query` shell-out and reports `{ref, memories, resolved}`. It **reports**
staleness and never repairs it: rewriting a user's frontmatter because a symbol
moved is a decision that belongs to the user.

When the `codegraph` binary is absent the endpoint returns `available: false`,
exactly as `/v1/codegraph/status` already does. Code refs keep working as
stored data; only resolution needs the binary.

---

## 3. Scaffolding on project create

`EnsureProject`, on genuine creation only, writes four base memories and two
empty section directories:

| Slug | Title | Starter body |
| --- | --- | --- |
| `todo` | TODO | a `- [ ]` checklist heading |
| `decisions` | Decisions | one entry per choice, and why |
| `conventions` | Conventions | style, commits, testing headings |
| `dev-log` | Dev Log | newest first |

Plus `dev/` and `env/`, created empty. These are the four memories the MCP
`instructions` have always told agents to keep; scaffolding them closes the gap
between what the instructions promise and what an agent actually finds.

**Filename case.** The file is `todo.md`, not `TODO.md`. Every identifier that
reaches the filesystem passes `ValidSlug`, which is lowercase by definition, and
an uppercase exception would need a case rule threaded through path validation,
resolution, indexing and the graph. The frontmatter `title` renders as `TODO`.

**Recursion.** `WriteMemory` calls `EnsureProject`, and scaffolding writes
memories, so ordering matters: `project.json` is written *first*, which makes
the nested `EnsureProject` calls find an existing project and return
immediately. One level, not a loop.

**Section meanings**, added to the server `instructions`:

- `dev/` — architecture, design notes, anything about how the code is built
- `env/` — one memory per deploy target: `local`, `dev`, `staging`, `prod`

---

## Testing

Unit, in `apps/backend`:

- `ValidPath` — depth limit, empty segments, traversal attempts, trailing slash
- `memorySlugs` — recursive walk, `.`-prefixed skipped at depth, sort order
- write/read/delete round-trip at `dev/architecture`, including empty-parent pruning
- containment — a `../` ref is rejected at both guards
- `code` frontmatter round-trip, and survival of an unrelated `Extra` key
- `code_refs` cleared on delete and rebuilt on reindex
- `Graph` — code nodes and edges present, `Kind` correct
- `EnsureProject` — scaffolds once, is idempotent, does not scaffold an existing project

End-to-end, through the real MCP transport (`mnemosyne call`), on this repo:

1. scaffold a throwaway project, assert the six entries exist
2. write `dev/architecture` carrying two code refs
3. `list_memories` with the `code` filter returns it
4. `/v1/graph` shows the code nodes
5. `recall` finds the nested memory
6. delete the throwaway project

Then reorganize this repo's own live memories into the new layout. `domus` is
left flat and untouched — it is the regression check that nesting stayed
additive.

---

## Not doing

- **Migrating flat memories automatically.** Moving a user's files without
  asking is worse than a mixed layout.
- **Enforcing the section names.** `dev/` and `env/` are a convention the
  instructions teach, not a schema the store validates. A project that wants
  `ops/` should get `ops/`.
- **Resolving code refs on write.** It would make every write depend on an
  external binary, to catch a staleness the refs endpoint reports on demand.
- **A `section` field on `WriteRequest`.** The path already says it.
