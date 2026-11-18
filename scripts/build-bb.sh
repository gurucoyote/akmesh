#!/bin/sh
# Build acmesh for one platform: one binary that carries the u-root commands,
# the ssh client and the tools from internal/tools.
#
#   ./scripts/build-bb.sh windows   -> acmesh.exe
#   ./scripts/build-bb.sh linux     -> acmesh
#
# A second argument picks a profile from scripts/profiles: full (default) or fs
# (no networking at all: the code is excluded, not switched off at runtime).
#
# Two things make the single binary possible:
#   - a go.work that lists both the pinned u-root checkout and this repository,
#     because makebb refuses packages from outside the main module;
#   - one small main package per tool under cmd/<name>.
#
# The u-root checkout is pinned, so a build is reproducible. This script never
# touches the acmesh repository's git.
set -eu

PLATFORM=${1:-linux}
PROFILE=${2:-full}
case "$PLATFORM" in
windows)
	export GOOS=windows GOARCH=amd64
	OUT=acmesh.exe
	# ssh and scp come from internal/tools now, for both targets.
	UROOT_EXTRA=""
	;;
linux)
	export GOOS=linux GOARCH=amd64
	OUT=acmesh
	# strace is Linux only. ssh and scp come from internal/tools now.
	UROOT_EXTRA="strace"
	;;
*)
	echo "usage: $0 [windows|linux]" >&2
	exit 2
	;;
esac

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# A profile other than full gets its own file name, so both builds can sit next
# to each other without one overwriting the other.
case "$PROFILE" in
full) ;;
*)
	case "$OUT" in
	*.exe) OUT="${OUT%.exe}-$PROFILE.exe" ;;
	*) OUT="$OUT-$PROFILE" ;;
	esac
	;;
esac
UROOT_COMMIT=869f00f6b224ce02b1db51806159d6cd9c246f76
UROOT_DIR="$ROOT/.build/uroot"

export GOCACHE="$ROOT/.cache"
export GOPATH="$ROOT/.gopath"
export GOTOOLCHAIN=auto
export GOFLAGS=

if [ ! -d "$UROOT_DIR/.git" ]; then
	mkdir -p "$ROOT/.build"
	git clone -q https://github.com/u-root/u-root.git "$UROOT_DIR"
fi
git -C "$UROOT_DIR" fetch -q origin
git -C "$UROOT_DIR" checkout -q "$UROOT_COMMIT"

# makebb is pinned. The generator decides the shape of the whole binary, so an
# unreviewed newer version would change the build without a change here. The
# module path and version are read back from the installed tool.
MAKEBB_MODULE=github.com/u-root/gobusybox/src
MAKEBB_VERSION=v0.6.0
MAKEBB="$GOPATH/bin/makebb"
if [ ! -x "$MAKEBB" ]; then
	MAKEBB="$GOPATH/bin/makebb.exe"
fi
makebb_is_pinned() {
	go version -m "$1" 2>/dev/null |
		awk -v mod="$MAKEBB_MODULE" -v ver="$MAKEBB_VERSION" \
			'$1 == "mod" && $2 == mod && $3 == ver { ok = 1 } END { exit !ok }'
}
install_makebb() {
	# GOOS/GOARCH are set for the target above; the tool itself must be built
	# for this host, otherwise it lands in bin/<goos>_<goarch>/.
	env -u GOOS -u GOARCH GOBIN="$GOPATH/bin" go install "$MAKEBB_MODULE/cmd/makebb@$MAKEBB_VERSION"
	MAKEBB="$GOPATH/bin/makebb"
	if [ ! -x "$MAKEBB" ]; then
		MAKEBB="$GOPATH/bin/makebb.exe"
	fi
	if ! makebb_is_pinned "$MAKEBB"; then
		echo "makebb is not at $MAKEBB_VERSION: the build would use another generator" >&2
		exit 1
	fi
}
# The tool comes first: installing it extracts gobusybox into the module cache,
# and the rename below needs that copy to exist.
if [ ! -x "$MAKEBB" ] || ! makebb_is_pinned "$MAKEBB"; then
	install_makebb
fi

# u-root's dispatcher calls itself "busybox" in the two messages a mistyped
# command produces. The binary is called acmesh, so those words are replaced in
# every copy of gobusybox the build can link: the vendored one inside the u-root
# checkout and the extracted module under GOPATH. A workspace build (GOWORK)
# ignores vendor/, a plain build does not, hence both. If u-root rewords the
# text, the build stops here instead of shipping the old name.
DISPATCHER_FILES="$UROOT_DIR/vendor/github.com/u-root/gobusybox/src/pkg/bb/bbmain/register.go
$(ls -d "$GOPATH"/pkg/mod/github.com/u-root/gobusybox/*/pkg/bb/bbmain/register.go 2>/dev/null || true)"
rename_in_dispatcher() {
	old=$1
	new=$2
	found=0
	for file in $DISPATCHER_FILES; do
		[ -f "$file" ] || continue
		found=1
		if grep -q "$new" "$file"; then
			continue
		fi
		if ! grep -q "$old" "$file"; then
			echo "u-root no longer says \"$old\" in $file: the dispatcher text changed" >&2
			exit 1
		fi
		sed -i "s/$old/$new/" "$file"
	done
	if [ "$found" -eq 0 ]; then
		echo "no gobusybox register.go found: the dispatcher text cannot be renamed" >&2
		exit 1
	fi
}
rename_in_dispatcher "not present in busybox" "not present in acmesh"
rename_in_dispatcher "busybox dispatched" "acmesh dispatched"

# makebb embeds that source with //go:embed, so a tool installed before the
# rename still carries the old word. Rebuild it once now that the source is
# patched; if the word survives anyway, stop rather than ship it.
if tr -d '\000' < "$MAKEBB" | grep -q 'not present in busybox'; then
	install_makebb
	if tr -d '\000' < "$MAKEBB" | grep -q 'not present in busybox'; then
		echo "makebb still embeds u-root's busybox wording: the rename cannot reach the build" >&2
		exit 1
	fi
fi

# Bad command lines must not look like success. Two u-root commands print their
# usage and carry on; the patches and their guards live in patch-uroot.sh.
"$ROOT/scripts/patch-uroot.sh" "$UROOT_DIR"

# The workspace is what lets makebb see our own packages.
WORKFILE="$ROOT/.build/go.work"
cat > "$WORKFILE" <<EOF
go 1.27.0

use (
	$UROOT_DIR
	$ROOT
)
EOF
export GOWORK="$WORKFILE"

# Every name our own tools provide. Where u-root also builds the same name for
# the target, u-root wins: it is already in the list and needs no extra bytes.
OURS_ALL="awk basename head echo false curl cut df diff dirname du env file free hostname gosh iconv jq iconv kill killall md5sum nl patch ps sed sha1sum sha256sum sha512sum shuf sort split stat sync tac timeout truncate uname wget sh policy ssh scp pwd true"

PROFILE_FILE="$ROOT/scripts/profiles/$PROFILE.txt"
if [ ! -f "$PROFILE_FILE" ]; then
	echo "unknown profile $PROFILE; available:" >&2
	ls "$ROOT/scripts/profiles" | sed 's/\.txt$//' >&2
	exit 2
fi
TAGS=""
PROFILE_EXCLUDE=""
while read -r key value; do
	case "$key" in
	''|'#'*) continue ;;
	tags) TAGS="$value" ;;
	exclude) PROFILE_EXCLUDE="$PROFILE_EXCLUDE $value" ;;
	*) echo "profile $PROFILE: unknown line '$key'" >&2; exit 2 ;;
	esac
done < "$PROFILE_FILE"

# One name per line: cmds.txt groups names on a line, and the profile excludes
# compare whole lines. Without this, "exclude ping" never matched "ping netcat".
NAMES=$(grep -v '^#' "$ROOT/scripts/cmds.txt" | tr ' ' '\n' | sed '/^$/d')
for c in $PROFILE_EXCLUDE; do
	NAMES=$(echo "$NAMES" | grep -v "^$c$" || true)
done
PKGS=""
for c in $NAMES; do
	PKGS="$PKGS ./cmds/core/$c"
done
for c in $UROOT_EXTRA; do
	if [ -d "$UROOT_DIR/cmds/core/$c" ]; then
		PKGS="$PKGS ./cmds/core/$c"
	else
		PKGS="$PKGS ./cmds/exp/$c"
	fi
done
# Names back on one line for the membership test below.
NAMES_SPACED=$(echo $NAMES)
UROOT_NAMES=" $NAMES_SPACED $UROOT_EXTRA "
OURS=""
for c in $OURS_ALL; do
	for x in $PROFILE_EXCLUDE; do
		[ "$c" = "$x" ] && continue 2
	done
	case "$UROOT_NAMES" in
	*" $c "*) ;;
	*) OURS="$OURS $c" ;;
	esac
done
echo "own commands in this build:$OURS"
for c in $OURS; do
	PKGS="$PKGS $ROOT/cmd/$c"
done

cd "$UROOT_DIR"
# shellcheck disable=SC2086
# makebb has no -tags flag; the go tool it calls does read GOFLAGS.
if [ -n "$TAGS" ]; then
	export GOFLAGS="-tags=$TAGS"
	echo "build tags: $TAGS"
fi
# A generator-only run is what a non-Go compiler needs: makebb writes the
# multicall source and the build is done by the caller.
if [ -n "${ACMESH_GEN_DIR:-}" ]; then
	"$MAKEBB" -g -gen-dir "$ACMESH_GEN_DIR" -o "$ROOT/$OUT" $PKGS
	echo "generated: $ACMESH_GEN_DIR"
	exit 0
fi
# shellcheck disable=SC2086
"$MAKEBB" -o "$ROOT/$OUT" $PKGS
# The rename above must reach the binary. A vendor/module mismatch would build
# fine and still ship u-root's wording, so the build checks its own output.
if tr -d '\000' < "$ROOT/$OUT" | grep -q 'not present in busybox'; then
	echo "$OUT still says busybox: the dispatcher rename did not reach the build" >&2
	exit 1
fi
ls -l "$ROOT/$OUT"
