# Storage format

## Layout

```
<memory-root>/                     configurable; default below
├── .mnemosyne/
│   └── index.db                   SQLite + FTS5 — derived, safe to delete
├── mnemosyne/                     a project = a directory
│   ├── project.json               project metadata
│   ├── use-fts5-not-vectors.md
│   └── storage-is-file-first.md
└── some-other-project/
    └── ...
```

## Where the root lives

The settings file cannot live inside the memory root, because it is what says
where that root is. It sits in the user config directory instead, and the
default root sits beside it:

| Purpose       | Path                                        |
| ------------- | ------------------------------------------- |
| Settings      | `<user config dir>/mnemosyne/config.json`   |
| Default root  | `<user config dir>/mnemosyne/memories`      |

`<user config dir>` is `%APPDATA%` on Windows, `~/Library/Application Support`
on macOS, and `$XDG_CONFIG_HOME` (usually `~/.config`) on Linux. One layout on
every platform beats three special cases that are easy to confuse.

Resolution order, most specific first:

1. `--root <path>`
2. `MNEMOSYNE_ROOT`
3. `root` in the settings file
4. the default above

`mnemosyne doctor` prints the resolved root and which of the four chose it, so a
surprising answer can be explained rather than guessed at.

## A memory

A UTF-8 Markdown file with YAML frontmatter. The frontmatter is the metadata;
the body is the memory.

```markdown
---
id: 01JD3K7QW8ZX4N2P
title: Use FTS5, not vectors, for the MVP
tags: [decision, search]
links: [storage-is-file-first]
created: 2026-09-12T22:40:11Z
updated: 2026-09-12T22:40:11Z
---

sqlite-vec is a loadable C extension, so adopting it means cgo or shipping
per-platform binaries plus an embedding model. FTS5 is already compiled into
modernc.org/sqlite and covers keyword search completely.

Revisit in Phase 3, when hybrid ranking needs both indexes anyway.
```

| Field     | Type      | Notes                                                     |
| --------- | --------- | --------------------------------------------------------- |
| `id`      | string    | ULID. Stable across renames; sorts by creation time.       |
| `title`   | string    | Human label. Seeds the filename but is edited freely after.|
| `tags`    | []string  | Flat, lowercase. Optional.                                 |
| `links`   | []string  | Slugs of related memories. Becomes graph edges in Phase 4. |
| `created` | RFC 3339  | Set once, UTC.                                             |
| `updated` | RFC 3339  | Set on every write, UTC.                                   |

YAML — not JSON — because a human edits this file in a text editor, and because
every Markdown tool already understands frontmatter. "JSON metadata" in the
brief meant structured metadata; YAML frontmatter is the idiomatic spelling of
that in a Markdown file.

Unknown frontmatter keys are preserved on write. Someone else's tooling putting
a key there is not our business to discard.

## Slugs and filenames

The filename minus `.md` is the memory's slug, and the slug is its address in
the API and MCP tools. Slugs are derived from the title: lowercased, non
`[a-z0-9]` runs collapsed to `-`, trimmed, capped at 80 characters, and suffixed
`-2`, `-3`, … on collision within the project.

The `id` is what survives a rename; the slug is what a human types. Both are
indexed, and lookups accept either.

## A project

A directory whose name is the project slug, plus `project.json`:

```json
{
  "id": "01JD3K7QW8ZX4N2Q",
  "name": "Mnemosyne",
  "description": "The memory server itself.",
  "created": "2026-09-12T22:38:02Z"
}
```

A directory without `project.json` is still treated as a project — the file is
written on first access. Dropping a folder of Markdown into the root and having
it just work is worth more than enforcing our own bookkeeping.

Directories beginning with `.` are ignored.

## Path safety

Project and memory identifiers are slugs, validated against `^[a-z0-9][a-z0-9-]*$`.
Every resolved path is additionally checked to be inside the memory root after
symlink evaluation. Two independent guards, because a path traversal here writes
arbitrary files on the user's machine.

## Atomic writes

Write to `<target>.tmp-<random>` in the same directory, `fsync`, then rename over
the target. Same-directory rename is atomic on every platform we support, so a
crash mid-write leaves either the old file or the new one, never a truncated one.

## The index

`.mnemosyne/index.db` mirrors what is on disk. Nothing lives only in the index.
On startup, the store reconciles: files whose `mtime`/size differ from the
indexed values are reparsed, indexed rows with no file are dropped. Deleting the
database costs one rebuild and nothing else.
