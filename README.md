# acmesh

A self-contained shell and userland, built in Go, for harnesses that should not
hand their agent the real system shell.

acmesh is one binary. It contains a POSIX/bash interpreter (mvdan/sh), the
commands of a small userland (u-root, compiled in with gobusybox), and its own
additions where u-root does not build on the target platform. It runs on Linux
and Windows from the same source.

The name is the point: "a mesh" of tools, not one big shell.

## Why

A harness (lait, KleinKai) that gives an agent `bash` gives it everything: any
path, any binary, any privilege the process has. acmesh exists to put the shell
and the tools into one binary whose file access and command set can be limited
in one place, and whose behaviour can be measured instead of guessed.

## State

- Working: one Windows binary, 8.9 MB, 48 commands plus the shell, built with
  gobusybox from u-root commit 869f00f, run and checked under Windows.
- Missing on Windows: df, du, free, hostname, kill, ps, sync, timeout, uname -
  they do not build there (build constraints). Six of them are small enough to
  write ourselves.
- Not yet in the binary: the policy layer (path rules in one wrapper for every
  command, then Landlock/Job Objects for the kernel side).

See `docs/design/acmesh.md` for the architecture and the measurements, and
`docs/measurements.md` for the numbers with the commands that produced them.

## Build

    ./scripts/build-bb.sh windows   # bb.exe for Windows
    ./scripts/build-bb.sh linux     # bb for Linux

The script pins the u-root commit and the command list, so a build is
reproducible. It needs Go 1.27 (`GOTOOLCHAIN=auto` handles that) and
`makebb` from github.com/u-root/gobusybox.

Build artifacts (`bb.exe`, `bb`) are not committed.

## Use

    bb.exe ls C:/Windows
    bb.exe grep -c Microsoft C:/Windows/System32/drivers/etc/hosts
    bb.exe gosh -c 'for f in *.exe; do echo "$f"; done'

Without arguments the binary lists the commands it carries. The symlink form
(`ln -s bb ls`) also works on Linux; on Windows use the subcommand form.

## Licence note

u-root and gobusybox are BSD-3-Clause, mvdan/sh is BSD-3-Clause. A binary that
links them carries their notices; see `docs/design/acmesh.md`.
