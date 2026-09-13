# Deployment: install, service, portable, and the local channel

Mnemosyne has to be usable three ways, and the differences between them are
almost entirely about *where the binary sits* — not about how it works.

| Mode              | Binary location            | PATH        | Autostart | Needs admin |
| ----------------- | -------------------------- | ----------- | --------- | ----------- |
| **Portable**      | anywhere, e.g. a USB stick | never       | never             | no          |
| **User install**  | per-user bin directory     | user PATH   | at logon (Phase 2) | no          |
| **Machine install** | system bin directory     | system PATH | at logon (Phase 2) | yes         |

User install is the default. Machine install exists only to put the binary on
the PATH for every account on a shared machine.

## Why admin does not mean a system service

The obvious design is "admin gets a real boot-time system service". It is wrong
here, and the reason is worth stating so it does not get re-proposed.

Memories live under the user's profile. A Windows service or a launchd
`LaunchDaemon` starts at boot as `SYSTEM`/`root`, before anyone logs in, with no
access to a user profile and no idea which of several users it would be serving.
It would either sit idle or need a whole impersonation layer to be useful.

So **the daemon is always per-user and always starts at logon**, in every mode.
Admin rights change where the binary is written and whose PATH is edited. They
do not change the process model. One process model, not two, means one set of
behaviour to reason about and to test.

| Platform | Autostart mechanism                                        |
| -------- | ---------------------------------------------------------- |
| Windows  | Scheduled Task, logon trigger, current user                  |
| macOS    | launchd `LaunchAgent` in `~/Library/LaunchAgents`            |
| Linux    | `systemd --user` unit, enabled with lingering off            |

A Scheduled Task is used on Windows rather than the `Run` registry key because
it can be restarted on failure and does not flash a console window.

## Portable mode

Portable mode is a single self-contained directory that writes nothing outside
itself: no registry keys, no PATH edits, no files in the user profile.

It turns on when either is true:

- a file named `mnemosyne.portable` sits next to the binary, or
- the command is given `--portable`.

The marker file is what makes an unzipped folder portable without anyone having
to remember a flag, which is the whole point of a portable build.

```
mnemosyne-portable/
├── mnemosyne(.exe)
├── mnemosyne.portable        the marker; may be empty
├── config.json               settings, beside the binary
└── memories/                 the memory root
    └── .mnemosyne/index.db
```

In portable mode the root resolution order still applies — `--root` and
`MNEMOSYNE_ROOT` still win — but the fallback is `<binary dir>/memories` instead
of the user config directory, and the settings file is read from and written
beside the binary.

`mnemosyne install` refuses to run in portable mode. Installing is the opposite
of what portable means, and silently doing it anyway would leave traces on a
machine the user meant to leave clean.

## Development mode

A checkout must be invisible to an installed Mnemosyne. Before this existed,
`pnpm dev` opened the *installed* memory root, fought the installed daemon for
the same pipe, and `pnpm predev` killed the daemon the user actually relies on.

`MNEMOSYNE_DEV=1` switches it. The desktop app sets it whenever it runs
unpackaged, so `pnpm dev` needs nothing extra; a terminal sets it by hand.

| | Installed | Development |
| --- | --- | --- |
| config file | `<user config>/mnemosyne/config.json` | `.../mnemosyne-dev/config.json` |
| default root | `<user config>/mnemosyne/memories` | `.../mnemosyne-dev/memories` |
| runtime dir | `<user cache>/mnemosyne` | `<user cache>/mnemosyne-dev` |
| channel | `mnemosyne.<user>` | `mnemosyne.<user>.<hash>` |
| `install`, `service install` | work | refused |
| logon entry | registered | never |

A sibling directory, not a subdirectory: deleting the dev data must not be able
to take real memories with it.

The two prohibitions are the point of the mode. A dev build can never put
itself on PATH over the installed binary, and can never register itself to
start at logon — `mnemosyne stop` in dev mode does not even touch the logon
entry, only its own daemon. The only way to get a startup entry is to install
the real binary deliberately.

## PATH

The point of PATH is that `mnemosyne` works in any terminal, and that an MCP
client configured with the bare command `mnemosyne` finds it.

| Platform             | What install does                                            |
| -------------------- | ------------------------------------------------------------ |
| Windows, user        | Appends to `HKCU\Environment` `Path`, broadcasts `WM_SETTINGCHANGE` |
| Windows, machine     | Appends to the machine `Path` under `HKLM`                     |
| macOS / Linux, user  | Copies into `~/.local/bin`, which is on PATH by default         |
| macOS / Linux, system| Copies into `/usr/local/bin`                                    |

Rules:

- **Never edit shell profiles.** `.bashrc`, `.zshrc`, and friends belong to the
  user. Installing into a directory that is already on PATH achieves the same
  thing without parsing and rewriting someone's dotfiles.
- **Appending to PATH is idempotent.** Install twice, appear once.
- **Uninstall removes exactly what install added**, and nothing it did not.
- If `~/.local/bin` turns out not to be on PATH, print the one line to add
  rather than adding it. Telling the user beats editing their shell behind them.

The broadcast on Windows matters: without `WM_SETTINGCHANGE`, already-open
terminals and Explorer keep the old PATH until the next logon, and the install
looks broken.

## The local channel

The desktop app and any other local client talk to the daemon over a channel
that never touches the network.

| Platform | Channel                                                      |
| -------- | ------------------------------------------------------------ |
| Windows  | Named pipe `\\.\pipe\mnemosyne.<user>`, DACL limited to that user |
| Unix     | Unix socket `$XDG_RUNTIME_DIR/mnemosyne/daemon.sock`, dir `0700`, socket `0600` |

**The OS is the security boundary.** A named-pipe DACL and unix file permissions
already enforce "only this user, on this machine". There is no listening TCP
port, so nothing on the LAN can reach it, no firewall prompt appears on first
run, and there is no port to collide with.

On top of that, a random token is written to `<runtime dir>/daemon.token` with
the same `0600` permissions, and every request must present it. This is defence
in depth, not the boundary: it guards against another process running *as the
same user* that finds the socket path. Anything with that user's full privileges
can read the token file anyway — which is exactly why the token is not treated
as the real protection.

The payload is the same JSON the REST API speaks, carried over the pipe or
socket instead of TCP, so the GUI has one client implementation regardless of
platform.

**A loopback TCP fallback is deliberately not shipped.** If some future client
genuinely cannot open a pipe or unix socket, the requirements are written down
here in advance: bind `127.0.0.1` only, a random port published in the runtime
directory, the token required on every request, and `Origin` headers rejected
outright so a web page in the user's browser cannot reach it.

## Concurrency: no exclusive owner

The daemon does not own the memory root, and clients do not proxy through it.

They do not need to. SQLite runs in WAL mode, which is built for multiple
processes; memory files are written through a temp file and an atomic rename;
and any drift is repaired by the reconcile pass that already runs at startup.

That means an agent's `mnemosyne serve` and the desktop app's daemon can both be
running against the same root, and neither has to know about the other. The
alternative — a proxy layer so that one process is the sole writer — would add a
protocol, a fallback path for when the daemon is absent, and a new class of bug,
to protect an invariant the storage design already holds.

## Commands

```
mnemosyne install [--machine] [--no-path] [--no-autostart]   shipped
mnemosyne uninstall [--machine]                shipped
mnemosyne daemon                               Phase 2
mnemosyne service status|start|stop            Phase 2
```

The daemon, the logon autostart, and the channel land together in Phase 2. An
autostart entry that launches a daemon nothing can talk to is a process that
burns memory to do nothing, so it waits for the client that gives it a purpose.

- `install` without flags does the per-user install: copy the binary and add its
  directory to the user PATH. It never requires admin.
- `--machine` writes to the system location and the system PATH, and is the only
  path that needs elevation. When autostart lands it will still be per-user.
- The binary is copied through a temp file and a rename, and an existing copy is
  moved aside first, so an upgrade can replace a binary that is currently
  running and an interrupted install cannot leave a truncated executable on PATH.
- `uninstall` reverses it, and never touches the memory root. Removing a program
  should not delete the user's data, and there is no backup yet to undo it with.
- `install` prints the exact MCP wiring command afterwards, since being on PATH
  is only useful if the user knows what to do with it.
