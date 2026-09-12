# MCP tool surface

Transport: stdio JSON-RPC 2.0, via `github.com/modelcontextprotocol/go-sdk`.
Server name `mnemosyne`. Started with `mnemosyne serve`.

Six tools. The set is deliberately small: every tool is a thing an agent has to
read the description of and choose between, so each one has to earn its place.

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
| `title`   | string   | yes      | Seeds the slug on create                         |
| `content` | string   | yes      | Markdown body                                    |
| `memory`  | string   | no       | Target an existing memory; omit to create        |
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
