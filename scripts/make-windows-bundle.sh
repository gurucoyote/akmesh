#!/bin/sh
# Build the portable Windows bundle: akmesh and its command names in one folder,
# started by a batch file, with nothing installed and nothing left behind.
#
# What it produces:
#   akmesh-portable/start.cmd             the only thing to start; lays out bin\
#   akmesh-portable/bin/akmesh.exe        the one binary (about 20 MB)
#   akmesh-portable/etc/profile.sh        PATH, prompt, cd to the home
#   akmesh-portable/etc/names.txt         one command name per line, from the binary
#   akmesh-portable/root/etc/apk/repositories   sources for apk: "winget" by default
#
# The binary lives in bin\ and not in the root: up there it is the first thing a
# person double clicks, and started under the name "akmesh" it cannot know which
# command was meant (asked 26.11.2026).
#   akmesh-portable.zip                   the same folder for handing over
#
# Why bin\ is not in the zip: a hardlink cannot be stored in an archive, and a
# copy per name would be 20 MB times 140 names. start.cmd creates the links in
# a second on the first run; no space is used twice.
set -e
cd "$(dirname "$0")/.."
ROOT=$(pwd)
BIN=akmesh.exe
OUT=akmesh-portable
ZIP=akmesh-portable.zip

if [ ! -f "$BIN" ]; then
	echo "make-windows-bundle: no $BIN here; run ./scripts/build-bb.sh windows" >&2
	exit 1
fi

rm -rf "$OUT" "$ZIP"
mkdir -p "$OUT/bin" "$OUT/etc" "$OUT/terminal" "$OUT/root/etc/apk"

cp "$BIN" "$OUT/bin/$BIN"

# Batch files want CRLF: cmd.exe reads a label at the end of a file badly
# without the carriage return.
sed 's/$/\r/' scripts/portable/start.cmd > "$OUT/start.cmd"
cp scripts/portable/profile.sh "$OUT/etc/profile.sh"

# apk's root inside the bundle. One source is configured: winget. A real apk
# repository is added as one more line, exactly as on Alpine.
printf '# The Windows package manager as a source for apk.\nwinget\n' > "$OUT/root/etc/apk/repositories"

NAMES=$(set +e; "./$BIN" __akmesh_list_commands__ 2>&1 | sed -n 's/^ - //p')
COUNT=$(printf '%s\n' "$NAMES" | grep -c . || true)
# The start script reads the names from this file; asking the binary from inside
# a batch for /f needs quoting that nobody wants to read later.
printf '%s\n' "$NAMES" | grep . > "$OUT/etc/names.txt"

cat > "$OUT/terminal/akmesh-profile.json" <<'JSON'
{
  // Merge this into the "profiles" -> "list" array of Windows Terminal's
  // settings.json, then close the braces. Newer Terminal versions accept
  // comments here; older ones want the comments removed.
  "name": "akmesh",
  "commandline": "\"C:\\path\\to\\akmesh-portable\\start.cmd\"",
  "startingDirectory": "%USERPROFILE%",
  "icon": "",
  "font": { "face": "Cascadia Mono" },
  "hidden": false
}
JSON

cat > "$OUT/README.txt" <<EOF
akmesh, portable for Windows
============================

Nothing is installed. This folder is the installation, and deleting it removes
everything: no registry entry, no service, no scheduled task, no file outside
this folder unless you write one yourself.

Start
-----

Double-click start.cmd, or from a command line:

    start.cmd
    start.cmd -c "ls C:/Windows | head"

The first start writes bin\\ with one hardlink per command name ($COUNT names).
so "sh", "ls", "grep", "awk", "sed", "tar", "curl", "ssh" work inside the shell.
That takes a moment and costs no extra disk. Hardlinks need NTFS on the same
drive; on FAT32 or a network share bin\\ stays empty and start.cmd says so.

Only start.cmd is meant to be started. bin\\akmesh.exe is the program behind every
name; double clicking it directly does nothing sensible, which is why it does not
sit next to start.cmd.

In Windows Terminal, add the profile from terminal\\akmesh-profile.json (adjust
the path) if you want akmesh as its own tab.

What is in the shell
--------------------

$COUNT commands, statically linked, no runtime to install: the text and file
tools you expect (ls, cat, grep, sed, awk, find, tar, gzip, md5sum, sort, diff),
the network ones (curl, wget, ssh, scp), and the document converters
(any2md, md2pdf, md2docx, md2epub, docx2md, rtf2md, pdf2md).

The prompt is two lines: "machine:directory", then "user\$". Tab completes
command names and paths. clear wipes the screen.

Not included
------------

git. Install it separately (winget install Git.Git) if you need it; akmesh does
not fight with it, they just live in the same PATH.

Known differences to Git Bash: no /c/... path translation (use C:/Users/...,
which works in every akmesh command), no job control (%1, fg, bg), and scripts
that hardcode MSYS paths need adjusting.
EOF

# The archive: built with python, since tar does not write zip and the host may
# have neither zip nor 7z.
python3 - "$OUT" "$ZIP" <<'PY'
import os, sys, zipfile
src, out = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as z:
    for base, dirs, files in os.walk(src):
        dirs.sort()
        for name in sorted(files):
            path = os.path.join(base, name)
            arc = os.path.relpath(path, src)
            # Keep the folder name inside the archive so unpacking cannot
            # scatter 140 names into the current directory.
            z.write(path, os.path.join(os.path.basename(src), arc))
PY

SIZE=$(wc -c < "$ZIP")
echo "make-windows-bundle: $OUT ($COUNT names), $ZIP ($SIZE bytes)"
