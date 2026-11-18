#!/bin/sh
# Fork a u-root command into this repository, mechanically.
#
#   ./scripts/fork-uroot.sh head cmds/core/head/head.go
#   ./scripts/fork-uroot.sh grep cmds/core/grep/grep.go pkg/uroot/unixflag/unixflag.go
#
# What it does, and nothing else:
#   - checks that .build/uroot sits on the pinned commit from build-bb.sh;
#   - copies the given upstream files into internal/forked/<name>/;
#   - renames "package main" to "package <name>";
#   - adds one provenance line under the copyright header (the header itself
#     stays untouched, the copyright is not ours);
#   - records the upstream file hash in internal/forked/manifest.json.
#
# What it does NOT do: adapt main() into Run(), resolve pkg imports, add the
# command to the table. Those are judgement calls and belong to the person or
# agent doing the fork; the script prints the checklist at the end.
#
# The drift rule lives in internal/forked/drift_test.go: the manifest names the
# commit and the upstream hashes, and the test fails when the build pin moves.
set -eu

NAME=${1:-}
shift 2>/dev/null || true
if [ -z "$NAME" ] || [ "$#" -eq 0 ]; then
	echo "usage: $0 <name> <upstream-file>..." >&2
	exit 2
fi

ROOT=$(cd "$(dirname "$0")/.." && pwd)
UROOT_DIR="$ROOT/.build/uroot"
UROOT_COMMIT=$(sed -n 's/^UROOT_COMMIT=//p' "$ROOT/scripts/build-bb.sh")
if [ -z "$UROOT_COMMIT" ]; then
	echo "UROOT_COMMIT not found in scripts/build-bb.sh" >&2
	exit 1
fi
if [ ! -d "$UROOT_DIR/.git" ]; then
	echo "$UROOT_DIR is not a checkout; run scripts/build-bb.sh once" >&2
	exit 1
fi
actual=$(git -C "$UROOT_DIR" rev-parse HEAD)
if [ "$actual" != "$UROOT_COMMIT" ]; then
	echo "checkout is at $actual, pin says $UROOT_COMMIT: run scripts/build-bb.sh" >&2
	exit 1
fi

DEST="$ROOT/internal/forked/$NAME"
mkdir -p "$DEST"

for rel in "$@"; do
	src="$UROOT_DIR/$rel"
	if [ ! -f "$src" ]; then
		echo "no such upstream file: $rel" >&2
		exit 1
	fi
	# Only the command's own directory keeps the bare file name. Everything
	# from pkg/ gets its parent as a prefix, because pkg/find/find.go and
	# cmds/core/find/find.go would otherwise land on the same file.
	file=$(basename "$rel")
	parent=$(basename "$(dirname "$rel")")
	case "$rel" in
	cmds/core/"$NAME"/*) ;;
	*) file="${parent}_${file}" ;;
	esac
	cp "$src" "$DEST/$file"
	# package main -> package <name>, so the copy can be imported.
	sed -i "s/^package main$/package $NAME/" "$DEST/$file"
	# Provenance goes under the copyright header, the header stays as it is.
	python3 - "$DEST/$file" "$UROOT_COMMIT" "$rel" <<'PY'
import sys
path, commit, rel = sys.argv[1], sys.argv[2], sys.argv[3]
lines = open(path).read().split("\n")
insert = len(lines)
for i, line in enumerate(lines):
    if line.startswith("//") or line.strip() == "":
        continue
    insert = i
    break
# Back up over blank separator lines so the note sits inside the comment block.
while insert > 0 and lines[insert - 1].strip() == "":
    insert -= 1
note = [
    "//",
    "// Forked from u-root %s, %s." % (commit, rel),
    "// Local changes are listed below this note; the rest of this file is the",
    "// upstream source and keeps its copyright.",
]
lines[insert:insert] = note
open(path, "w").write("\n".join(lines))
PY
	python3 - "$ROOT/internal/forked/manifest.json" "$NAME" "$UROOT_COMMIT" "$rel" "$src" <<'PY'
import hashlib, json, os, sys
path, name, commit, rel, src = sys.argv[1:6]
data = {"forks": {}}
if os.path.exists(path):
    data = json.load(open(path))
entry = data["forks"].setdefault(name, {"commit": commit, "files": []})
entry["commit"] = commit
digest = hashlib.sha256(open(src, "rb").read()).hexdigest()
entry["files"] = [f for f in entry["files"] if f["upstream"] != rel]
entry["files"].append({"upstream": rel, "sha256": digest})
entry["files"].sort(key=lambda f: f["upstream"])
json.dump(data, open(path, "w"), indent=2, sort_keys=True)
open(path, "a").write("\n")
PY
	echo "copied $rel -> internal/forked/$NAME/$file"
done

cat <<EOF

Fork $NAME angelegt. Noch zu tun:
  1. func main() in func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int
     umbauen, flag-Paket raus (die Argumente kommen als Slice).
  2. Kopierte pkg-Dateien importieren als "$NAME"-Symbole im selben Paket;
     "internal/forked/$NAME" von internal/tools aus aufrufen.
  3. internal/tools/<name>.go als duenne Huelle, Eintrag in der Tabelle,
     cmd/$NAME/main.go, $NAME in OURS_ALL in scripts/build-bb.sh und den Namen
     aus scripts/cmds.txt streichen - sonst gewinnt weiter u-root und der Fork
     landet nicht im Binary (gemessen: head -5 lief weiter in den u-root-Befehl).
  4. Lokale Aenderungen unter dem Provenienz-Hinweis auflisten.
  5. Test in internal/forked/$NAME, Zeile in docs/measurements.md, Hinweis in
     docs/design/acmesh.md und in der Lizenzliste.
EOF
