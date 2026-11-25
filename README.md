# akmesh

A self-contained shell and userland in one Go binary, for harnesses that should
not hand their agent the real system shell.

akmesh is spoken as Ak Mesh: a mesh of tools rather than one big shell. It runs
on Linux and Windows from the same source.

## Why

A harness that gives an agent `bash` gives it everything: any path, any binary,
any privilege the process has. akmesh exists so that a harness hands over one
binary instead, whose command set is fixed and whose file access can be limited
in one place - by the kernel, not by a check the agent could talk its way past.
Everything it does is meant to be measured, and every number in
`docs/measurements.md` names the command that produced it.

## What is in it

One binary carries 143 commands:

| Group | Count | What |
| --- | --- | --- |
| Documents | 14 | any2md, pdf2md, docx2md, epub2md, rtf2md, html2md, pdf2html, epub2html, md2txt-l, and the writers md2pdf, md2docx, md2epub, html2docx, html2epub |
| Files and text | 64 | awk, sed, grep (real PCRE2 for `grep -P`), diff, patch, tar, cpio, gzip, unzip, find, jq, iconv, rg, split, sort, tr, xargs and the rest of a small userland |
| System | 45 | ps, kill, pgrep, pkill, df, du, free, mount, mknod, losetup, blkid, passwd, id, apk, apkbuild, mkcert, verify, certinfo, policy and friends |
| Services | 7 | init, getty, login, crond, syslogd, logger, logread |
| Network | 10 | curl, wget, ssh, scp, httpd, ping, ip, netcat, netstat, dhclient |
| Shell | 2 | sh and gosh - mvdan/sh as a library, so the shell the agent gets is the shell the policy covers |
| Other | 1 | strace (Linux only) |

Two things are worth naming, because agents rely on them and busybox does not
have them:

- **Familiar spellings.** `ps -ef`, `ps aux`, `find -maxdepth 1`, `head -5`,
  `grep -rn`, combined short flags like `curl -fsSL`, and `sed -i`. The rule is
  to behave the way the agent already expects, not the way we would prefer it.
  Where a command is a forked copy of u-root for exactly that reason, the copy
  keeps the u-root copyright header and names its upstream commit.
- **One place for the policy.** With `AKMESH_POLICY` set to a JSON file, the
  rule is applied to the process before any command runs, and the kernel -
  Landlock on Linux - enforces it for every child. Without the variable the
  binary is an ordinary akmesh and restricts nothing. A broken policy exits
  nonzero; it never falls back to running unconfined. See `docs/policy.md`.

## Use

    akmesh ls C:/Windows
    akmesh grep -c Microsoft C:/Windows/System32/drivers/etc/hosts
    akmesh gosh -c 'for f in *.exe; do echo "$f"; done'
    akmesh awk '{s+=$2} END {print s}' data.txt
    akmesh diff a.txt b.txt > change.diff && akmesh patch change.diff
    akmesh sed -i 's/alt/neu/' datei.txt
    akmesh any2md report.pdf > report.md
    akmesh md2pdf -o report.pdf report.md
    akmesh sh -c 'apk add --root ./rootfs busybox'      (Linux, needs root)
    akmesh strace -o trace.txt cat /etc/hostname        (Linux only)

Without arguments the binary lists the commands it carries, grouped. The symlink
form (`ln -s akmesh ls`) works on Linux; on Windows use the subcommand form.

Container images:

    ./scripts/make-image.sh          # akmesh:latest,       22.8 MB, one /bin
    ./scripts/make-image.sh --split  # akmesh-split:latest, 57.8 MB, five binaries

## The split: five binaries instead of one

A harness that only reads documents does not need a TLS stack, and one that only
runs scripts does not need the PDF writers. The same source builds four smaller
binaries beside the socket, all behind one shared dispatcher:

| Binary | Linux | Windows | Carries |
| --- | --- | --- | --- |
| akmesh | 21.5 MiB | 19.7 MiB | everything |
| akmesh-docs | 5.8 MiB | 5.9 MiB | the readers, md2txt-l, rg |
| akmesh-write | 9.2 MiB | 9.3 MiB | md2pdf, md2docx, md2epub, html2docx, html2epub |
| akmesh-net | 10.0 MiB | 10.3 MiB | curl, wget, ssh, scp, httpd, mkcert, verify, certinfo |
| akmesh-shell | 8.6 MiB | 8.9 MiB | sh, gosh, apk, apkbuild, apkreport, apkguard |

All five are stripped Go binaries. The reasoning and the measurements behind the
split are in `docs/design/split.md` and `docs/measurements.md`.

## Build

    GOTOOLCHAIN=auto ./scripts/build-bb.sh linux     # akmesh and the four parts
    GOTOOLCHAIN=auto ./scripts/build-bb.sh windows   # the same, as .exe

Needs Go 1.27 (`GOTOOLCHAIN=auto` fetches it) and `makebb` from
github.com/u-root/gobusybox, which the script pins. The u-root commit and the
command list are pinned in one place, so a build is reproducible. Two builds in
the same tree do not get along: they share `.build/`, so build one target at a
time.

Checks run against the built binary, not against the source only:

    ./scripts/test-akmesh.sh ./akmesh                           # full battery
    ./scripts/test-akmesh.sh ./akmesh.exe
    ./scripts/test-akmesh.sh ./akmesh-net -c "curl wget httpd"   # one group
    ./scripts/help-audit.sh --check                              # --help answers

Build artifacts are not committed.

## Repository layout

    internal/tools/      our own commands, in one table that dispatch and tests share
    internal/<name>/     one package per command or family
    internal/multicall/  the shared dispatcher for the split binaries
    cmd/<name>/          one tiny main per own command
    scripts/             build, image, security scan, battery
    docs/design/         architecture and decisions
    docs/measurements.md every number, with the command that produced it

## Licence

BSD-3-Clause, the licence u-root uses; see `LICENSE`. Commands under
`internal/forked` are copies of u-root source under the same licence and keep
their copyright headers. A binary that links all this carries the notices of its
dependencies: `NOTICE` names them with their pinned versions, and
`docs/design/akmesh.md` says how a distribution has to be handled.
