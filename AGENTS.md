# Agents Guide for akmesh

A shell and userland in one Go binary, for harnesses that must not hand an
agent the real system shell. Code, comments, identifiers and commit messages
are English; documents meant for Martin are German.

## Project Layout

- `scripts/build-bb.sh`: the only build entry point. Pins the u-root commit and
  the command list, calls `makebb`.
- `scripts/cmds.txt`: the command list per platform (one name per line,
  `#` comments allowed). Adding a command means adding it here. A line may start
  with `linux:`, which keeps u-root names that only build for Linux out of the
  Windows build (`linux: mount umount mknod blkid losetup`).
- `internal/tools/`: our own commands in one package with one table. The table is
  what the multicall binary dispatches on and what the per-name entry points use.
- `internal/usage/`: the shared `--help` answer (one table of short texts, plus
  `usage.Run`/`RunIO`, which every `cmd/<name>` main calls before its parser
  runs). See docs/design/akmesh.md, "Hilfe-Texte"; the gate is
  `scripts/help-audit.sh --check`.
- `cmd/<name>/`: one tiny `main` per own command, so that the combined binary
  reaches it under its plain name (`akmesh sed`). Adding a command means: implement
  in `internal/tools`, register in the table, add `cmd/<name>`.
- `scripts/make-image.sh`: the image. Builds statically, reads the command names
  from the binary itself (it prints them when called with an unknown name), lays
  out `/bin` with one symlink per name, tars the rootfs and `podman import`s it
  as `akmesh:latest`. A name list written down here would drift; the binary is
  the source. `--verify-only` re-runs the checks against the existing image.
- `scripts/test-akmesh.sh`: the live battery, run against the built binary for
  either target (`./scripts/test-akmesh.sh ./akmesh.exe`).
- `scripts/build-bb.sh` writes `.build/go.work` (both modules) and passes the
  u-root packages plus our `cmd/<name>` packages to `makebb`.
- `docs/design/akmesh.md`: architecture, decisions, licence notes.
- `docs/measurements.md`: every number with the command that produced it.
- `progress.txt`, `learnings.txt`: short German notes per finished step.

## Build

    GOTOOLCHAIN=auto ./scripts/build-bb.sh windows

- Go 1.27 is required by the u-root module; `GOTOOLCHAIN=auto` downloads it.
  Do not pin u-root to an older commit to avoid that.
- `GOCACHE=/root/akmesh/.cache` must be set: the default cache is read-only
  here and produces misleading toolchain errors. The script sets it.
- `makebb` takes GOOS/GOARCH from the environment, it has no `-goos` flag. It is
  pinned to v0.6.0 in the script and re-read from the installed tool with
  `go version -m`; a different version is replaced, not used.
- The build renames gobusybox's two dispatcher messages from "busybox" to
  "akmesh" in the pinned checkout and rebuilds `makebb` when it still embeds the
  old text (go:embed). It fails instead of shipping the old wording.
- Build artifacts (`akmesh`, `akmesh.exe`, `.cache/`, `.gopath/`) stay untracked.
- A profile can drop a whole block of the command list by its id instead of
  naming every command: `exclude-group services` in `scripts/profiles/<name>.txt`.
  The names come from the one table in `internal/cmdlist`
  (`internal/cmdlist/cmdlisttool` prints them), so a new daemon is excluded
  automatically and the profile cannot drift from the real list. An unknown id
  fails the build. This is why the daemons live in their own block: a harness
  that only wants file and text tools must be able to leave login, getty, crond
  and syslogd out in one line.
- Every size reported to Martin is human readable and names the linking and the
  stripping, because a bare byte count says little: `17.4 MiB (18276476 bytes),
  statically linked, stripped`. The build prints that line itself
  (`report: ...`); take the numbers from there instead of writing your own.
  Stripping is a fact to state, not an assumption: read it from the binary.

## Policy first

The binary exists so a harness does not hand the agent a real shell. When
`AKMESH_POLICY` is set, the rule is applied to the process with Landlock before
any command runs, and the kernel enforces it for every child. Do not add
per-command path checks in Go: that would be the guessing this project exists to
avoid. A broken policy must exit nonzero, never fall back to running unconfined.
See docs/policy.md.

## Windows: no traces in the host

What akmesh does on Windows stays local to akmesh. Write logs and state into
files we own (below the working directory, or a path the caller names), never
into the Windows Event Log, the registry, a scheduled task or a service. Do not
register an SCM service: a service is a host install, survives the session and is
exactly the trace we do not want. A daemon runs in the foreground, under whatever
already supervises the caller - if nobody supervises it, it simply ends.

The reason is the same as for the policy: a harness must be able to run this
binary without leaving something behind that outlives it. When in doubt, keep a
feature Linux-only rather than reach for a host facility.

## Guiding principle for commands

Implement every command the way an agent will expect it, not the way we would
like agents to behave. When a flag spelling or an output format is in doubt,
follow the tool the agent already knows (procps, GNU, curl) instead of inventing
a cleaner one. Educating the agent costs more than accepting a familiar shape.

Concretely: `ps -ef` must work, `-s` in curl must not swallow the body, combined
short flags like `-fsSL` must be understood, and a missing command makes agents
reach for powershell or python, which is worse than a slightly larger binary.

## Testing

- A build is not a proof. The binary is run on the target: on Linux directly,
  on Windows from this WSL shell (`./akmesh.exe ls C:/Windows`), which needs no wine.
- GNU comparisons run in the pinned reference container: image plus digest and
  the tool version are in `docs/reference-image.txt`, and each measurement
  section names the version it compared against.
- Every behaviour claim in a report names the command that produced it.
- A command that u-root ships with different flags is a finding, not a bug in
  akmesh: `find` has no `-maxdepth`, `head` has no `-3`. Record it in
  `docs/measurements.md` instead of patching the upstream command.

## Adding our own commands

- One package per command under `internal/<name>`, with the Windows and the
  unix implementation separated by build tags (`//go:build windows` and
  `//go:build !windows`).
- Every command goes through the shared policy wrapper; a command that opens a
  path on its own is a defect, because the rules then do not hold.
- A command is done when it runs on both targets, not when it compiles.

## Working process (for agents)

1. Plan: read the code, settle the intent with Martin. No code yet.
2. Spec and tasks: what must hold at the end, split into 2-5 minute steps.
3. Test first where a test can exist (RED, GREEN, REFACTOR).
4. One commit per step, naming the ticket.
5. Numbers into `docs/measurements.md`, the outcome into the ticket.

## Tickets: bd

- The tracker lives in `.beads/` with the prefix `akmesh` (`akmesh-abc`).
- Every bd call uses `--no-daemon`. After a change:
  `bd export -o .beads/issues.jsonl`, stage that file, commit it.
  If bd answers `Database out of sync with JSONL`, run
  `bd sync --import-only --no-daemon` and read again - the tickets are not lost.
- `bd close <id> -r "<reason>"`: the reason needs the flag, a positional text is
  read as a second ID and the close fails silently.
- Close only after the build and the run on the target: a ticket whose proof is
  missing stays open with a comment saying what is missing.

## Git

- Local only. **Never push, never pull, never fetch** unless Martin asks for it
  in that session.
- Commit atomically: one topic per commit, imperative one-line message, the
  ticket ID in it when there is one.
- Keep `.beads/issues.jsonl` in the same commit as the change it belongs to.
- Never commit a build artifact or a copy of a foreign project's source.

## Licence

u-root and gobusybox are BSD-3-Clause, mvdan/sh is BSD-3-Clause. A distributed
binary must carry their notices; `docs/design/akmesh.md` keeps the list.

## Session close

There is no remote. Work is complete when:

1. Remaining work is a ticket, not a sentence in the chat.
2. The build ran on the target and the result is in `docs/measurements.md`.
3. `bd export -o .beads/issues.jsonl` ran and the file is committed with the change.
4. `git status` is clean and every commit message names the work.
5. `progress.txt` has one German line saying what got done.

Never push, never claim work is done because it compiles.
