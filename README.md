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

- Working: one binary per platform, 10.0 MiB under Windows and 10.1 MiB under
  Linux. It carries the 48 u-root commands, the shell (gosh), the ssh client,
  strace on Linux, and our own tools where u-root has a gap or builds only for
  Linux.
- Our own tools inside the binary: awk, cut, diff, patch, sed, sha1sum,
  sha256sum, sha512sum, split, stat, tac, nl, plus hostname, sync, timeout and
  uname, which u-root does not build for Windows and whose timeout takes a
  different command line on Linux.
- Missing on Windows: df, du, free, kill, ps - they do not build there. A native
  replacement is the next piece of work.
- Not yet in the binary: the policy layer (path rules in one wrapper for every
  command, then Landlock/Job Objects for the kernel side).

See `docs/design/acmesh.md` for the architecture and the measurements, and
`docs/measurements.md` for the numbers with the commands that produced them.

## Build

    ./scripts/build-bb.sh  windows  # bb.exe: everything, one binary
    ./scripts/build-bb.sh  linux    # bb
    ./scripts/build-ext.sh windows  # ext.exe: only our own tools, for testing
    ./scripts/build-ext.sh linux

Checks run against either shape:

    ./scripts/test-ext.sh ./bb.exe
    ./scripts/test-ext.sh ./bb

The script pins the u-root commit and the command list, so a build is
reproducible. It needs Go 1.27 (`GOTOOLCHAIN=auto` handles that) and
`makebb` from github.com/u-root/gobusybox.

Build artifacts (`bb.exe`, `bb`) are not committed.

## Use

    bb.exe ls C:/Windows
    bb.exe grep -c Microsoft C:/Windows/System32/drivers/etc/hosts
    bb.exe gosh -c 'for f in *.exe; do echo "$f"; done'
    bb.exe awk '{s+=$2} END {print s}' data.txt
    bb.exe diff a.txt b.txt > change.diff && bb.exe patch change.diff
    bb.exe sed -i 's/alt/neu/' datei.txt
    bb ssh -F config host command
    bb strace -o trace.txt cat /etc/hostname      (Linux only)

Without arguments the binary lists the commands it carries. The symlink form
(`ln -s bb ls`) also works on Linux; on Windows use the subcommand form.

## Licence note

u-root and gobusybox are BSD-3-Clause, mvdan/sh is BSD-3-Clause. A binary that
links them carries their notices; see `docs/design/acmesh.md`.
