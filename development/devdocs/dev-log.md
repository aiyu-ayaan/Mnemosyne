# Dev log

Newest first. One entry per commit stage: what shipped, what it was verified
with, and any decision worth not re-litigating later.

## Stage 15 — A dev build that cannot touch the installed one, and no console window

Two reports, one underlying cause: the development build and the installed
build were the same installation.

**A black console window sat on the desktop.** The daemon is a console-subsystem
binary, so the logon Scheduled Task gave it a console of its own and it stayed
for the whole session. `Hidden` in the task XML hides the *task*, not the
window. The fix is in the daemon rather than in each launcher: on Windows it
calls `GetConsoleProcessList`, and frees the console when it is the only
process attached — a console Windows created for it. Launched from a terminal
the console is shared with the shell, the count is higher than one, and the
logs keep printing where the developer is looking.

**`pnpm dev` ran against real memories.** The checkout resolved the same config
file, the same memory root and the same pipe as the installed copy, and
`predev` shut the user's daemon down on every start. `MNEMOSYNE_DEV=1` now
moves config, root, runtime directory and channel to a `mnemosyne-dev` sibling,
and `install` and `service install` refuse outright: a dev build must never
overwrite the installed binary, take its PATH entry, or register itself at
logon. `stop` in dev mode leaves the logon entry alone too. The desktop app
sets the variable whenever it runs unpackaged, so `pnpm dev` needs nothing, and
`predev` now stops only the dev daemon — matched by executable path, not by a
name glob that caught the user's.

Verified: `mnemosyne channel --json` with and without the variable returns
different pipes, tokens and roots; both install commands refuse under it; a new
`TestDetectDevLayout` asserts no dev path equals its installed counterpart.

New: [`debugging.md`](debugging.md) — how to debug Mnemosyne with other agents,
which is mostly `tools`/`call` to reproduce without an agent, and a separate
`mnemosyne-dev` MCP server name so it is always clear which build answered.

## Stage 14 — One tab strip, a command palette, and one home for the theme

Two reported bugs turned out to sit on top of a structural one.

**Theme was configurable in four places** — Settings, Theme Studio, the status
bar, and the Welcome screen. Settings lost its copy (the whole `THEMES.map`
swatch grid, ~80 lines) and its `theme`/`onTheme`/`onOpenThemeStudio` props;
Theme Studio is the one place a theme is chosen, reachable from the status bar
and the palette. Settings is storage root and diagnostics now.

**The settings icon was a sun.** `<circle r=3>` plus six disconnected line stubs
at uneven angles reads as a sun at 24px and as noise at 14px. Replaced with one
closed 8-tooth cog outline on the same 24×24 / `strokeWidth 1.5` grid as the
other icons, generated rather than hand-fitted: flat tooth tops at r=9.7, flat
valleys at r=6.7, eight evenly spaced teeth, concentric hub. Checked at both
sizes side by side against the old one before keeping it.

**The structural bug: two parallel tab systems.** `App.tsx` carried both a
`tabs` array for memories and a `mainMode` enum for Graph/MCP/Theme/Settings.
They could not coexist — opening Settings hid every memory you had open — and
`Tabs.tsx` repeated the same tab block four times because the views were not in
a list to map over.

`OpenTab` became a discriminated union of `MemoryTab | ViewTab`. That deleted the
`mainMode` state machine and its five-branch `onActivity`, collapsed the four
copied blocks into one `map`, and removed the tab bar's right-hand
Graph/MCP/Theme/⚙ strip, which duplicated the activity bar two centimetres away.
Net −182 lines across the desktop app; `Tabs.tsx` went 206 → 127. TypeScript
found every memory-only path for free once the union existed — `saveTab`,
`deleteActive`, the backlinks effect, `dirtyCount`, the project-delete filter —
which is the argument for the union over a nullable field.

**A command palette**, `Ctrl+Shift+P`, in the overlay `QuickOpen` already had:
`>` switches modes and the arrow keys, filter, and Enter are shared, because in
an editor they are one overlay. Commands that need an open memory grey out
rather than disappear. `ui-design.md` had ruled the palette out; the reversal and
its reason are recorded there.

**Own skin, not a clone**: an accent rule along the top of the active tab rather
than VS Code's, and per-kind marks on the four convention slugs (`todo`,
`decisions`, `conventions`, `dev-log`) so a row of tabs is scannable instead of
five identical document icons.

**Verified with:** `tsc --noEmit`, `vite build`, and the built renderer driven in
Electron against a stubbed bridge — screenshots confirming Settings opens beside
an open memory tab, the palette renders with its groups and disabled entries, and
Settings holds no theme picker. Also corrected `ui-design.md`, which claimed the
graph uses React Flow; it is hand-rolled SVG and always was.

## Stage 13 — An MCP surface that is cheap to use and honest about what it knows

The server worked; it was thin in the places that cost tokens every session, and
silent in the two places 2026 research says agent memory actually fails —
staleness and duplicates. Seven changes, plus two bugs found while making them.

**`write_memory` gained `mode`.** Replacing the whole body meant adding one line
to `dev-log` cost a `read_memory` and the entire body sent back — paying twice
for the most common write there is. `append` and `prepend` do it in one call.
Bodies join on a blank line so appended Markdown stays valid; on a create there
is nothing to join, so the mode is inert rather than an error, because the first
"append to `todo`" has to work like the tenth. An unknown mode is rejected up
front, including on create, so a typo cannot pass silently.

**Resources.** Every memory is now `mnemosyne://<project>/<slug>`, registered at
startup and kept in step from the change bus, so a user can `@`-mention a memory
in their client instead of asking the agent to go find it. The watcher runs only
under `Serve` — `New` gives a correct snapshot, and a goroutine that outlives its
server is worse than a list that does not move.

**Prompts.** `checkpoint`, `onboard`, `review-stale`. Prompts are the right home
for a multi-step routine: `instructions` are paid for on every session, a tool is
a choice on every call, a prompt costs nothing until the user picks it.

**`age` on every hit** — `today`, `12d`, `8mo` — and one instruction to distrust
an old fact and write the answer back. Coarse on purpose: the reader is a
language model, and "8mo" is weighed without date arithmetic against a today it
may be wrong about.

**`similar` on create.** Up to three existing memories that may already cover the
ground, so the agent merges instead of forking. It never blocks the write.

**Delete moves to `.mnemosyne/trash/`** instead of unlinking, stamped so two
memories that shared a slug across time do not overwrite each other. The result
names the path. No `restore_memory` tool: restoring is a human decision about
something an agent already got wrong, so it belongs in the GUI.

**A `global` project** for facts about the user rather than about a codebase. A
project-scoped `recall` searches it too and merges by score, so preferences are
stated once instead of once per repository.

### Two bugs found on the way

**`age` did not work, because `updated` was never selected.** `Hit` had `Created`
and `Updated` fields whose comment admitted they were "empty when the query did
not select them" — and neither `search` nor `GetHit` selected them, so they were
empty on every search path. Only `Backlinks` filled them. Both queries select the
timestamps now.

**`similar` found nothing, because the lookup ran after the write.** The new
memory matches its own title exactly, and one exact hit is enough to stop the FTS
query falling back to OR — so a loosely-worded duplicate found nothing but
itself, which was then filtered out, leaving an empty and falsely reassuring
answer. The lookup runs before the write. The original test passed throughout,
because its two titles shared a phrase that matched under AND;
`TestCreateReportsLooselySimilarMemories` pins the shape that actually broke.

Also fixed while in the files: a dead `if textErr != nil` nested inside
`if textErr != nil && …` in hybrid recall, and two missing `rows.Err()` checks in
the graph query that would have silently truncated the graph on a mid-iteration
error. Deleted `cmd/snapshot`, an unformatted scratch tool whose only behaviour
was writing a `repo` memory listing every file in the repo — which is what
`git ls-files` already says.

**Verified with:** `go test ./...`, `go vet ./...`, `gofmt -l .` clean, and the
real binary against a scratch root — `prepend` producing newest-first without a
read, `similar` catching both near-duplicates, `age` reading `today`, and
`delete_memory` returning its trash path. `mnemosyne tools` now prints prompts
and resources alongside the tools, since it claims to show what an agent sees.

## Stage 12 — Graceful daemon shutdown and Windows file lock resilience

A backend rebuild during `pnpm dev` failed with `open ..\..\bin\mnemosyne.exe: The process cannot access the file because it is being used by another process.` when a previous dev session or active MCP client was running.

- **Daemon shutdown route & CLI command**: Added `POST /v1/shutdown` endpoint to `internal/api` and `internal/daemon`, and a top-level `mnemosyne stop` command that cleanly shuts down the local channel daemon before stopping the Scheduled Task.
- **Lock-resilient Go build wrapper**: Added `apps/backend/scripts/build.cjs` which safely renames any currently running `.exe` to `.old` before compiling, allowing Go to create a fresh binary even if an active agent/MCP session is holding the previous binary.
- **Desktop predev order & dev lifecycle**: Corrected `predev` to stop the stale daemon before building (`node ../../scripts/stop.cjs && pnpm --filter @mnemosyne/backend build`). Updated Electron `main.cjs` to track the dev daemon process and cleanly shut it down on exit or SIGINT/SIGTERM rather than leaking a detached orphan process.
- **Root stop script**: Added `scripts/stop.cjs` to cleanly shut down any running daemon and clean up lingering dev processes.

**Verified with:** `go test ./...` in backend, `TestShutdownEndpoint`, `TestDaemonShutdownOverLocalChannel`, `pnpm test`, `pnpm lint`, and building while an active daemon is running.

## Stage 11 — MCP discoverability, a dev loop, and the search bug behind both

Agents could not find their way around the server, and the user had to name it
in every session. Three things were wrong, and the third was the real one.

**The server never said what it was for.** Eight tool names, an empty
`instructions` field. Filled it: when to recall unprompted, when to write, the
four-memory project convention (`todo`, `decisions`, `conventions`, `dev-log`),
and which tool to pick. Added `title` and annotations to every tool — reads are
read-only and idempotent so clients can auto-approve, `delete_memory` is
destructive so it prompts. `TestDiscoverability` pins it, because it is easy to
drop when editing tool definitions. See [`mcp-tools.md`](mcp-tools.md).

**No way to exercise a tool without an agent.** Added `mnemosyne tools` and
`mnemosyne call <tool> '<json>'`, both going through the real MCP server over an
in-memory transport, so what they print is what the agent gets. Root scripts:
`pnpm mcp:tools`, `pnpm mcp:instructions`, `pnpm mcp:call`, `pnpm stop`,
`pnpm doctor`.

**Natural-language search returned nothing.** The real reason the server looked
broken. Search passed the query to FTS5 raw, and FTS5 ANDs its terms, so "how
should I write commit messages" required every one of those words to appear in a
memory. It matched nothing and returned an empty result with no error. `recall`
is built on the same path, so its default mode was silently empty for exactly
the phrasing agents use — single-word queries worked, which is why it looked
like it worked at all. The raw query still goes first so `NEAR`, `prefix*`, and
an explicit `OR` keep working; when it comes back empty, or is not valid FTS5 at
all, it is retried as quoted terms joined with `OR` and ranked by bm25.

Two more found by running every tool end to end against a real store:

- `write_memory` rejected a slug that did not exist yet, so the convention the
  instructions describe failed on its first write. A named slug is now a create.
- `read_backlinks` returned zero-valued timestamps: the index query never
  selected the `created`/`updated` columns it already stored.

**Verified with:** `pnpm test` (all packages), plus every tool driven through
`mnemosyne call` against the real memory root — write, create-at-slug, update,
list, search, recall in all three modes, backlinks, read, delete, read-after-delete.
Semantic mode errors without a local embedding provider, which is expected and
explicit; hybrid falls back to text.

**Left undone:** recall ranking is bm25-only without an embedding provider
running, so results are ordered but not sharply. `development/` is documented in
`Commit.md` as a submodule; it is currently plain tracked files in the main repo.

## Stage 10 — UI/UX overhaul and multi-theme system

Complete UI overhaul implementing `/ui-ux-pro-max` guidelines and a 6-theme system:

- **6 Developer Themes**: VS Code Dark Modern, Obsidian Midnight (OLED Deep Black), Tokyo Night,
  Nord Arctic Slate, Catppuccin Mocha, and Clean Paper Light.
- **Instant Theme Switcher**: Available in Status Bar popup and Settings View with live visual color swatches.
- **Enhanced Markdown Editor**:
  - View modes: Edit, Split (side-by-side editing and live preview), and Rendered Preview.
  - Safe Markdown renderer with headings, bold, italic, code blocks with Copy button, blockquotes,
    checkboxes, bullet lists, and clickable `[[wikilinks]]`.
  - Rich formatting toolbar for rapid syntax insertion.
  - Interactive `#tag` chips with remove/add, `[[wikilink]]` pills, word/character/token counters,
    and save status indicators.
- **Enhanced Explorer**: Quick filter input, folder icons with open/closed states, tag cloud with count badges,
  and hover action controls.
- **Enhanced Graph View**: Obsidian-style physics force tuning HUD (repulsion, spring length, gravity, show labels),
  dual-mode CodeGraph symbol explorer, and node hover cards.
- **Enhanced Search & Panel**: Segmented ranking pills (Keyword FTS5, Hybrid RRF, Semantic Vector),
  expandable bottom panel, badge counters, and formatted event logs.

## Stage 9 — Graph view, backlinks, and CodeGraph integration

`internal/markdown` link parsing, `internal/index` links table, `internal/store` backlinks and graph,
MCP `read_backlinks` tool, `/v1/graph` and `/v1/codegraph/*` API routes, and Obsidian-style canvas graph in desktop app. Phase 4 complete.

- `internal/markdown` extracts `[[target]]` / `[[target|alias]]` wikilinks and body `#tags`
  (ignoring markdown `# Heading`), seamlessly merging them into `Memory.Links` and `Memory.Tags`.
- `internal/index` schema adds `links` table indexed by `(project, source_slug)` and
  `(project, target_slug)`. Memory writes replace outbound links; deletions clean up links.
- `Store.Backlinks` returns all memories linking to a given slug; `Store.Graph` returns all nodes and edges
  for a project (or across all projects).
- MCP server exposes `read_backlinks` tool accepting `project` and `memory` (slug or ULID).
- Daemon API serves `GET /v1/projects/{project}/memories/{memory}/backlinks`, `GET /v1/graph`,
  `GET /v1/codegraph/status`, and `GET /v1/codegraph/query`.
- Desktop app implements a high-performance HTML5 canvas force-directed graph view (`GraphView.tsx`)
  and sidebar controller (`GraphSidebar`), offering Obsidian-style physics, zoom/pan/drag, degree-scaled
  node radii, tag/project coloring, hover tooltips, click to inspect, and CodeGraph codebase symbol exploration.

**Decisions taken here**

- *Canvas-based force-directed simulation over bulky 3rd-party graph packages.* Provides 60fps
  performance on high-DPI screens without introducing heavy dependencies or React 19 version
  conflicts.
- *Dual-view workflow.* Activity bar "Graph" button displays the interactive graph across the main area
  while keeping a filterable overview in the sidebar, with quick toggling between editor tabs and graph view.
- *CodeGraph integration via local CLI.* Exposes `/v1/codegraph/status` and `/v1/codegraph/query` directly,
  allowing users to navigate codebase symbols and structure inside the same unified graph interface.

## Stage 8 — Semantic search, embeddings, and hybrid recall

`internal/embed`, `internal/index` vector tables, `internal/store/recall.go`,
MCP `recall` tool, and `/v1/embeddings` API endpoint. Phase 3 complete.

- `internal/embed` implements a pluggable provider interface with support for
  local Ollama (`nomic-embed-text`), OpenAI-compatible embedding endpoints, and
  a no-op `none` provider. Embeddings are L2-normalised so similarity is a fast
  dot product.
- `internal/index/vector.go` stores vectors as float32 blobs alongside FTS5 in
  SQLite, with cascading deletes when memories are removed. `Nearest` scans
  stored vectors linearly and ranks by cosine similarity.
- `internal/store/recall.go` implements Reciprocal Rank Fusion (RRF) combining
  FTS5 lexical matches and vector nearest-neighbour matches. Memories are embedded
  asynchronously on write and reconciled in batches.
- MCP server exposes `recall` tool accepting `query`, `project`, `limit`, and
  `mode` (`hybrid`, `semantic`, `text`).
- API serves `GET /v1/embeddings` and supports `mode` query parameter on `GET /v1/search`.

**Decisions taken here**

- *Pure-Go vector table over cgo sqlite-vec.* Linear scan over float32 blobs
  executes in single-digit milliseconds for thousands of vectors while keeping
  pure-Go compilation and zero cgo complexity across all platforms.
- *Async embedding on memory write.* `WriteMemory` writes to disk and updates FTS5
  synchronously, firing embedding asynchronously so slow or offline embedding
  providers never block memory creation.
- *Reciprocal Rank Fusion with k=60.* Standard RRF formula merges disparate score
  distributions from BM25 and cosine distance robustly without needing ad-hoc
  score calibration.

## Stage 7 — The desktop shell

`apps/desktop`: Electron, React, TypeScript, Vite, Tailwind v4, and a
`mnemosyne channel` command to bridge them. Phase 2 complete.

- The main process is the only thing with Node. It resolves the daemon, starts
  one if none is answering, and speaks HTTP over the channel; the renderer gets
  four calls and two subscriptions through a preload bridge.
- The layout is the one in `ui-design.md`: activity bar, collapsible sidebar
  with Explorer/Search/Graph/MCP/Settings, editor tabs, a four-tab bottom panel,
  and a status bar. Dark by default, light as a root-class override.
- The editor is a frontmatter form over a Markdown textarea. Tabs carry dirty
  state, and closing a dirty tab or deleting anything asks first.
- Settings moves the memory root through `PUT /v1/settings`, which the daemon
  applies without a restart.

**Decisions taken here**

- *`mnemosyne channel --json` instead of reimplementing path resolution in
  TypeScript.* Root resolution, portable detection, and per-platform endpoint
  naming already exist in Go. One subprocess call means the app and the backend
  cannot disagree about where the daemon is.
- *HTTP over `socketPath`, not a custom protocol.* Node's `http` accepts both a
  Windows named pipe and a unix socket there, so the same client code serves
  both platforms and the payload is the same JSON the API already speaks.
- *The main process is plain CommonJS.* The renderer is where the application
  lives and where TypeScript earns its keep; a second build pipeline to
  transpile two small files would be more machinery than the files are long.
- *A textarea, not a code editor component.* The files are Markdown the user can
  open in their own editor, and syntax highlighting a memory does not justify
  the dependency. Revisit if editing turns out to be where people live.
- *Inline SVG, not an icon package.* Five icons and a chevron.
- *A `Prompt` modal rather than `window.prompt`.* Electron does not implement
  `prompt`, and a delete that silently did nothing because a dialog returned
  undefined would be a data-loss bug.
- *Library metadata is fetched for every project up front.* That is what makes
  global tag counts and quick-open work without a second index in the renderer;
  no bodies are fetched. Marked with a `ponytail:` comment and the upgrade path
  — one aggregate endpoint — if a root ever holds enough projects to notice.
- *Events are coalesced before a refetch.* One write publishes twice (the write,
  then the reindex), so reloading per event would refetch the library several
  times for one user action.
- *The daemon is left running when the window closes.* It is the same per-user
  process the logon entry starts, and an agent's session may be relying on it.
- *The renderer loads the build when no dev server answers.* `electron .` after
  a build shows the app instead of a blank window, which is also how this stage
  was verified.
- *Phase 3 and 4 routes are called and their absence tolerated.* The search
  view offers semantic ranking only if `/v1/embeddings` says it is available,
  and the Backlinks panel treats a missing route as an empty list, so the
  window lights up as those phases land rather than needing a rewrite.

**Also here:** `.gitignore` had a bare `mnemosyne` pattern for the built binary,
which also matched `cmd/mnemosyne/` — so `main.go` had never been committed. The
patterns are anchored now, and `apps/desktop/dist/` is ignored.

**Verified:** `pnpm build` and `pnpm test` green across the workspace (`tsc
--noEmit` for the renderer, `go test ./...` for the backend). Then for real: two
memories were hand-written as Markdown into a scratch root, `doctor` found both
through reconcile, the Electron app started its own daemon and loaded the UI
with no renderer errors, and a script using the same Node `socketPath` path the
main process uses read `/v1/health`, `/v1/projects`, and `/v1/search?q=vectors`
over the named pipe — the last returning the hand-written memory.

## Stage 6 — The daemon, the local channel, and the JSON API

`internal/events`, `internal/api`, `internal/channel`, `internal/daemon`,
`internal/service`, and two new CLI commands. Phase 2.1-2.6 complete.

- `internal/events` is a broadcast bus. The store publishes what it changed;
  anything holding a connection open forwards it. `/v1/events` is the SSE
  transport for it.
- `internal/api` serves the same store methods as JSON. Handlers unmarshal, call
  one store method, and marshal — the same shape as the MCP handlers, so the two
  surfaces cannot drift.
- `internal/channel` is the local transport: a named pipe on Windows via
  `go-winio`, a unix socket elsewhere. Node's `http` speaks to both through
  `socketPath`, so the desktop app needs no protocol code of its own.
- `internal/daemon` ties them together and runs the reconcile ticker.
- `internal/service` registers the logon entry per platform by driving
  `schtasks`, `launchctl`, and `systemctl` — the documented interfaces, already
  installed, and a fraction of the code of the equivalent API bindings.

**Decisions taken here**

- *The watcher polls `Reconcile` rather than using fsnotify.* Reconcile is
  already the stat-based mechanism for "a change we did not make", so a ticker
  reuses it for no new dependency, no debounce logic, and no recursive-watch
  bookkeeping when a project directory appears. Marked with a `ponytail:`
  comment naming the ceiling — one stat per memory per tick — and the upgrade
  path if a large root ever shows up in a profile.
- *`store.ErrInvalid` was added alongside `ErrNotFound`.* Without it the API
  could only tell "missing" from "everything else", and a mistyped project name
  would surface in the GUI as a 500. Transports match on the sentinel, so the
  mapping cannot drift with error wording.
- *The event bus is shared across a root change.* `store.OpenWith` exists so
  that `PUT /v1/settings` can close one store and open another while the clients
  streaming `/v1/events` stay subscribed. The new root is opened before the
  config file is written, so a bad path leaves the daemon serving what it had.
- *A slow SSE subscriber drops events rather than blocking the writer.* A
  dropped event costs a stale row until the next one; blocking a memory write on
  a wedged GUI socket would cost the write.
- *A fresh token per daemon start.* A token left behind by a previous run cannot
  be replayed against this one. It is still defence in depth — the pipe DACL and
  the socket mode are the boundary.
- *The Windows task is registered from XML, not `schtasks /SC ONLOGON`.* The two
  reasons `deployment.md` gives for choosing a Scheduled Task over the Run
  registry key — restart on failure and no console flash — are only reachable
  through the XML definition.
- *`service status` probes the channel as well as the platform.* Only an actual
  request tells a registered-but-crashed daemon apart from a healthy one, and it
  exercises the channel, the token, and the store in one call.

**Fixed on the way:** `.gitignore` had a bare `mnemosyne` pattern intended for
the built binary, which also matched the `cmd/mnemosyne/` directory — so the
binary's own `main.go` had never been committed. The patterns are now anchored.

**Verified:** `go vet ./...` clean for windows, linux, and darwin; `go test
./...` passing, including a daemon test that runs over the real platform channel
and an API test asserting a write arrives on the SSE stream. Then for real: a
daemon started against a scratch root, `service status` reached it over
`\\.\pipe\mnemosyne.<user>` and reported the root it was serving, and
reported "not reachable" with the missing token path once it was stopped.

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
