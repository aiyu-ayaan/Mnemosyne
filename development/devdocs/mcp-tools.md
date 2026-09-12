# MCP surface

Transport: stdio JSON-RPC 2.0, via `github.com/modelcontextprotocol/go-sdk`.
Server name `mnemosyne`. Started with `mnemosyne serve`.

Three primitives, each paying for something the others cannot:

| Primitive     | Count | Who triggers it | Costs                          |
| ------------- | ----- | --------------- | ------------------------------ |
| **Tools**     | 8     | the agent       | a choice on every call         |
| **Resources** | *n*   | the user        | one list on connect            |
| **Prompts**   | 3     | the user        | nothing until picked           |

Eight tools. The set is deliberately small: every tool is a thing an agent has to
read the description of and choose between, so each one has to earn its place.
Growing the surface happens sideways into resources and prompts, which an agent
does not pay to ignore, rather than into a ninth and tenth tool.

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

## Resources

Every memory is also an MCP resource at `mnemosyne://<project>/<slug>`,
`text/markdown`. Tools and resources answer different questions: a tool is the
agent deciding to go looking, a resource is the *user* pointing at something in
their client's own picker — `@decisions` — before the agent has decided
anything.

The advertised set is registered at startup from what is on disk, then kept in
step from the change bus while `serve` is running: `AddResource` on
`memory.written` (adding a URI again is how the SDK replaces it, so create and
update are one path), `RemoveResources` on `memory.deleted`, and a full rebuild
on the coarse events — a reconcile after an external edit, a root change. The
SDK emits `notifications/resources/list_changed` for each, so a memory an agent
writes appears in the user's picker without a reconnect.

Capped at `maxResources` (500), because `resources/list` is sent whole and a
large store would otherwise spend a client's context on a directory listing.

The watcher runs only under `Serve`, not `New`: a snapshot that cannot move is
better than a goroutine that outlives the server it updates.

## Prompts

User-invoked, so unlike `instructions` they cost nothing per session. Each is
text, not code — the agent already has the tools; what it lacks when the user
asks is the order to use them in.

| Prompt         | Arguments                   | For                                               |
| -------------- | --------------------------- | ------------------------------------------------- |
| `checkpoint`   | `project?`                  | end of session: write down what is still true after it |
| `onboard`      | `project?`                  | start of session: load and summarise what is known |
| `review-stale` | `project?`, `older_than_days?` | verify, update, or retire memories past an age  |

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

One project is not a codebase. **`global`** holds facts about the user that hold
everywhere — tools they always reach for, conventions they carry between repos.
A project-scoped `recall` searches `global` too and merges by score, so an agent
gets standing preferences back without knowing to ask twice, and the user states
them once instead of once per repository. It is still an ordinary project
directory; nothing enforces it.

## Driving the tools by hand

```
mnemosyne tools            # tools with signatures, then prompts, then resources
mnemosyne tools --full     # plus the instructions the client injects
mnemosyne call recall '{"query":"commit messages"}'
mnemosyne call write_memory '{"project":"p","memory":"dev-log","content":"- shipped x","mode":"prepend"}'
```

Both accept `--root <dir>`, which is how to try a destructive tool without
writing into the real store.

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
| `mode`    | string   | no       | `replace` (default), `append`, `prepend`         |

Returns the slug and whether it created or updated.

**`mode` is the token fix.** Replacing a whole body means an agent adding one
line to `dev-log` has to `read_memory` first and send the rest back unchanged —
paying for the entire memory twice on the most common write there is. `append`
and `prepend` do it in one call; `prepend` is what makes `dev-log`'s
"newest first" convention cheap. Bodies are joined with a blank line, so
appended Markdown stays valid. On a create there is nothing to join, so the mode
is inert rather than an error — the first "append to `todo`" must work like the
tenth. An unknown mode is rejected on create too, so a typo cannot pass silently.

**`similar` guards against duplicates.** When the caller lets the title pick the
slug, the result also carries up to three existing memories that may already
cover the same ground. It never blocks the write — a false positive costs one
read, a false negative costs a library of near-duplicates nobody notices until
recall starts returning three of them.

The lookup runs *before* the write, and the ordering is load-bearing: afterwards
the new memory matches its own title exactly, and one exact hit is enough to stop
the FTS query falling back to OR, so a loosely-worded duplicate finds nothing but
itself. `TestCreateReportsLooselySimilarMemories` pins that shape.

## `delete_memory`

| Arg       | Type   | Required |
| --------- | ------ | -------- |
| `project` | string | yes      |
| `memory`  | string | yes      |

Removes the index row and moves the file to
`<root>/.mnemosyne/trash/<project>/<slug>-<unix>.md`, returning that path as
`trashed`. It is a move, not an unlink: an agent deleting the wrong memory
should cost the user a restore, not the memory. The timestamp in the name means
deleting two memories that shared a slug across time does not have the second
silently overwrite the first.

There is no `restore_memory` tool. Restoring is a human decision about something
an agent already got wrong, so it belongs in the GUI, not in the tool list.

## `search_memories`

| Arg       | Type   | Required | Notes                                  |
| --------- | ------ | -------- | -------------------------------------- |
| `query`   | string | yes      | FTS5 query over title, tags, and body  |
| `project` | string | no       | Omit to search every project           |
| `limit`   | int    | no       | Default 10, max 50                     |

Returns ranked hits with a highlighted snippet, not full bodies. The agent reads
what it decides is relevant.

Every hit carries `age` — `today`, `12d`, `8mo` — alongside `updated`. Staleness
is the failure mode agent memory actually has: a high-relevance fact that became
wrong when circumstances changed, recalled confidently forever. A coarse phrase
is deliberate: the reader is a language model, and "8mo" is weighed without date
arithmetic against a today it may be wrong about. The instructions tell agents to
verify an old fact and write the answer back rather than leaving it to be
recalled again next session.

`age` needs `updated`, and neither `search` nor `GetHit` used to select the
timestamp columns at all — so the field existed and was empty on every search
path. Both queries select them now.

Phase 3 adds semantic ranking behind this same tool rather than a second one —
an agent choosing between "search" and "recall" is a choice we should be making
for it.

## Error convention

Failures return an MCP tool error with a plain sentence: `project "foo" not
found`, `memory "bar" not found in project "foo"`. The reader is a language
model; an error that says what to do next is worth more than an error code.
