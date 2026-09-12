# MCP tool surface

Transport: stdio JSON-RPC 2.0, via `github.com/modelcontextprotocol/go-sdk`.
Server name `mnemosyne`. Started with `mnemosyne serve`.

Eight tools. The set is deliberately small: every tool is a thing an agent has to
read the description of and choose between, so each one has to earn its place.

## Discoverability

Tool names alone told an agent nothing about *when* to reach for Mnemosyne, so
every session used to start with the user saying "use the memory server". Two
protocol features carry that now.

**`instructions`** (`mcpserver.Instructions`, sent in the initialize result).
MCP clients inject it into the agent's context, so it is the only lever that
makes usage automatic. It covers when to recall unprompted, when to write, the
project convention below, and which tool to pick. It is paid for on every
session, so it stays short and is about *when*; the tool descriptions cover
*how*.

**Annotations.** Every tool declares `title` and hints. Reads are
`readOnlyHint` + `idempotentHint`, so a client can auto-approve them;
`delete_memory` is `destructiveHint`, so it prompts. `openWorldHint` is false
throughout — the store is local and closed.

`TestDiscoverability` pins all of it: instructions reach the client, every tool
is annotated with the right read-only status, and no tool ships without a title
or with a description too short to say when to call it.

## Project convention

Convention, not schema — there is no `init_project` tool and nothing is
enforced. The instructions tell agents to use the repository directory name as
the project slug and to keep four memories current rather than accumulating
loose notes:

| Slug          | Holds                                              |
| ------------- | -------------------------------------------------- |
| `todo`        | the living checklist: done, in progress, planned    |
| `decisions`   | choices and why; one entry per choice              |
| `conventions` | how the codebase does things: style, commits, tests |
| `dev-log`     | what actually shipped, newest first                |

Anything else gets its own descriptive slug. This is why `write_memory` creates
at a caller-named slug that does not exist yet: the first write to `todo` must
not be a special case.

## Driving the tools by hand

```
mnemosyne tools            # every tool, signature, read/write/destructive
mnemosyne tools --full     # plus the instructions the client injects
mnemosyne call recall '{"query":"commit messages"}'
```

Or `pnpm mcp:tools`, `pnpm mcp:instructions`, `pnpm mcp:call <tool> '<json>'`,
which rebuild the binary first. Arguments can be piped on stdin instead, for
large bodies.

Both commands go through the real MCP server over an in-memory transport rather
than calling the store, so they exercise schema validation, annotations, and
error mapping: what they print is what the agent gets.

## `list_projects`

No arguments. Returns every project with its slug, name, description, and memory
count.

## `list_memories`

| Arg       | Type   | Required | Notes                        |
| --------- | ------ | -------- | ---------------------------- |
| `project` | string | yes      | Project slug                 |
| `tag`     | string | no       | Filter to memories with tag  |
| `limit`   | int    | no       | Default 50, max 500          |

Returns slug, title, tags, and `updated` per memory — metadata only. Bodies are
fetched deliberately, so listing a large project cannot blow up a context window.

## `read_memory`

| Arg       | Type   | Required |
| --------- | ------ | -------- |
| `project` | string | yes      |
| `memory`  | string | yes      | slug or ULID |

Returns full frontmatter and body.

## `write_memory`

Creates or updates. Upsert, because "did this already exist" is a round trip the
agent should not have to make.

| Arg       | Type     | Required | Notes                                           |
| --------- | -------- | -------- | ----------------------------------------------- |
| `project` | string   | yes      | Created if it does not exist                     |
| `title`   | string   | no       | Seeds the slug on create; required unless `memory` names one, where it defaults from the slug |
| `content` | string   | yes      | Markdown body                                    |
| `memory`  | string   | no       | Slug to write at: updated if it exists, created if not. An id-shaped ref must already exist, since an id cannot be chosen. Omit and the title picks the slug. |
| `tags`    | []string | no       | Replaces existing tags when present              |
| `links`   | []string | no       | Replaces existing links when present             |

Returns the slug and whether it created or updated.

## `delete_memory`

| Arg       | Type   | Required |
| --------- | ------ | -------- |
| `project` | string | yes      |
| `memory`  | string | yes      |

Deletes the file and its index row. Not recoverable in the MVP — Phase 5 adds
backup, and until then the honest answer in the tool description is that this is
permanent.

## `search_memories`

| Arg       | Type   | Required | Notes                                  |
| --------- | ------ | -------- | -------------------------------------- |
| `query`   | string | yes      | FTS5 query over title, tags, and body  |
| `project` | string | no       | Omit to search every project           |
| `limit`   | int    | no       | Default 10, max 50                     |

Returns ranked hits with a highlighted snippet, not full bodies. The agent reads
what it decides is relevant.

Phase 3 adds semantic ranking behind this same tool rather than a second one —
an agent choosing between "search" and "recall" is a choice we should be making
for it.

## Error convention

Failures return an MCP tool error with a plain sentence: `project "foo" not
found`, `memory "bar" not found in project "foo"`. The reader is a language
model; an error that says what to do next is worth more than an error code.
