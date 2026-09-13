# Debugging with other AI agents

Mnemosyne is a thing agents talk to, so the fastest way to debug it is to *be*
the agent, or to point a real one at a development build and watch.

Every command below assumes the development build. Set `MNEMOSYNE_DEV=1` in the
shell first (PowerShell: `$env:MNEMOSYNE_DEV = "1"`), or it talks to the
installed copy and its real memories.

## Without an agent: drive MCP by hand

The commands that make the MCP surface inspectable from a terminal:

    bin\mnemosyne.exe tools                        the tool list an agent sees
    bin\mnemosyne.exe tools --full                 plus the full instructions text
    bin\mnemosyne.exe call recall {"query":"x"}    invoke one tool, print the result

`call` runs the same code path a real client hits, so a tool that misbehaves
here misbehaves for every agent. Reproduce with `call` before blaming the agent.

## With an agent: wire the dev binary in

Register the development binary under a *different server name* than the
installed one, so both can exist and you always know which one answered.

    claude mcp add mnemosyne-dev --env MNEMOSYNE_DEV=1 -- <repo>/bin/mnemosyne.exe serve
    codex  mcp add mnemosyne-dev --env MNEMOSYNE_DEV=1 -- <repo>/bin/mnemosyne.exe serve

Then ask the agent to exercise what you changed — recall something, write a
memory, list projects — and compare what it reports with what `call` returns. A
gap between them is a protocol bug; the same wrong answer in both is a store or
index bug.

Rebuild between rounds with `pnpm build:backend`. An agent spawns a fresh
`serve` per session, so restarting its session is what picks up the new binary.

## Reading what happened

- **stdout is JSON-RPC and nothing else.** All logging goes to stderr. Anything
  printed to stdout outside the protocol breaks every client, silently.
- The daemon's stderr is the log. Run `mnemosyne daemon` in a terminal and it
  keeps that console; started by the logon task or by the app it frees the
  console, so a terminal run is the way to watch it.
- `mnemosyne doctor` answers "which root, chosen how, how much is in it", and is
  the first command to run when an agent reports an empty or surprising result.
- `mnemosyne channel` and `mnemosyne service status` answer "is anything
  listening, and is it the one I think".

## Two agents at once

The daemon and the MCP server are equal clients of the same store — WAL, atomic
renames, and reconcile are what make that safe. Running the desktop app and one
or more agents against the same dev root is therefore supported, and is a good
way to find races: write with one, and the others should see it within a poll
interval without anyone restarting anything.

## The things that actually go wrong

| Symptom | First check |
| --- | --- |
| Agent sees no memories | `doctor` — wrong root, or `MNEMOSYNE_DEV` not set where you thought |
| `recall` returns keyword hits only | the embedding provider is down; semantic ranking degrades, it does not fail |
| Agent cannot start the server | run `mnemosyne serve` by hand and read stderr |
| Changes not picked up | the agent still runs the previous binary; rebuild, then restart its session |
