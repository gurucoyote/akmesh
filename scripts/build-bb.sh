#!/bin/sh
# Build akmesh for one platform: one binary that carries the u-root commands,
# the ssh client and the tools from internal/tools.
#
#   ./scripts/build-bb.sh windows   -> akmesh.exe
#   ./scripts/build-bb.sh linux     -> akmesh
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
# touches the akmesh repository's git.
set -eu

PLATFORM=${1:-linux}
PROFILE=${2:-full}
# The name this build answers to. The sources stay akmesh; the binary, the
# messages and the issuer of a test certificate say whatever is set here:
#   AKMESH_NAME=bssdsh ./scripts/build-bb.sh linux
# It travels two ways: into internal/appname.Name through the linker (-X), and
# into the two dispatcher messages, which come from gobusybox and are renamed
# here because they are strings in a foreign file, not variables.
NAME=${AKMESH_NAME:-akmesh}
case "$PLATFORM" in
windows)
	export GOOS=windows GOARCH=amd64
	OUT="$NAME.exe"
	# ssh and scp come from internal/tools now, for both targets.
	UROOT_EXTRA=""
	;;
linux)
	export GOOS=linux GOARCH=amd64
	OUT="$NAME"
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
# A workspace file somewhere above the build would pull its own module set into
# this build; a stray /tmp/wk/go.work once failed it with a Go version that has
# nothing to do with this repository.
export GOWORK=off

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
# The slimmed goldmark-pdf fork lives as a patch, not as a copy of the module:
# apply it to the module cache before the compiler reads the module (acmesh-y9r).
"$ROOT/scripts/apply-goldmark-pdf-fork.sh"
# Same shape for Go.Sed: sed has to read a regexp the way GNU sed does
# (acmesh-4ne), which the library's Go-flavoured engine does not.
"$ROOT/scripts/apply-go-sed-fork.sh"

# rg runs on gripgrep, pinned. Its GitHub mirror ended at v0.17.0 while the
# forge had v0.17.3, so the version is checked against the build list instead
# of being assumed from "@latest" (acmesh-55u). A different version is a
# decision, not an accident: change it here and in go.mod together.
GRIPGREP_VERSION=v0.17.3
GRIPGREP_HAVE=$(cd "$ROOT" && go list -m -f '{{.Version}}' git.site.quack-lab.dev/dave/gripgrep 2>/dev/null || true)
if [ "$GRIPGREP_HAVE" != "$GRIPGREP_VERSION" ]; then
	echo "gripgrep: build list has '${GRIPGREP_HAVE:-nothing}', pinned is $GRIPGREP_VERSION" >&2
	echo "the module comes from git.site.quack-lab.dev/dave/gripgrep; see acmesh-55u" >&2
	exit 1
fi

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

# The generated multicall source. It stays on disk, so a failed build can be
# read, and so the list checks below have something to look at.
GEN_DIR="${GEN_DIR:-$ROOT/.build/gen}"
LISTPATCH="$ROOT/docs/design/patches/gobusybox-grouped-list.patch"
# prepare_gen makes the generated tree ours: the command list is printed by
# internal/cmdlist (its table), not by gobusybox's template. Both steps are
# checked by a marker, because a generator that rewords its dispatcher must stop
# the build instead of shipping the flat list again.
prepare_gen() {
	gen="$1"
	[ -d "$gen" ] || { echo "no generated source in $gen" >&2; exit 1; }
	sed 's/^package cmdlist$/package main/' "$ROOT/internal/cmdlist/cmdlist.go" > "$gen/cmdlist.go" || exit 1
	if ! grep -q '^func Print(' "$gen/cmdlist.go"; then
		echo "cmdlist.go was not written into $gen" >&2
		exit 1
	fi
	cp "$ROOT/scripts/gen/print.go.txt" "$gen/print.go" || exit 1
	if ! patch -p0 -d "$gen" -i "$LISTPATCH" >/dev/null; then
		echo "the generated dispatcher does not match $LISTPATCH: the command list cannot be grouped" >&2
		exit 1
	fi
	if ! grep -q 'printSupported()' "$gen/main.go"; then
		echo "the list patch left no call in $gen/main.go" >&2
		exit 1
	fi
}

# u-root's dispatcher calls itself "busybox" in the two messages a mistyped
# command produces. The binary is called akmesh, so those words are replaced in
# every copy of gobusybox the build can link: the vendored one inside the u-root
# checkout and the extracted module under GOPATH. A workspace build (GOWORK)
# ignores vendor/, a plain build does not, hence both. If u-root rewords the
# text, the build stops here instead of shipping the old name.
DISPATCHER_FILES="$UROOT_DIR/vendor/github.com/u-root/gobusybox/src/pkg/bb/bbmain/register.go
$(ls -d "$GOPATH"/pkg/mod/github.com/u-root/gobusybox/*/pkg/bb/bbmain/register.go 2>/dev/null || true)"
# rename_in_dispatcher NEW OLD...: the first argument is what has to stand in
# the file, the rest are the wordings that may stand there now. A checkout
# patched by an earlier build already carries the binary name, so accepting only
# u-root's original text would stop the second build of a renamed project.
rename_in_dispatcher() {
	new=$1
	shift
	found=0
	for file in $DISPATCHER_FILES; do
		[ -f "$file" ] || continue
		found=1
		if grep -q "$new" "$file"; then
			continue
		fi
		done_rename=0
		for old in "$@"; do
			if grep -q "$old" "$file"; then
				sed -i "s/$old/$new/" "$file"
				done_rename=1
				break
			fi
		done
		if [ "$done_rename" -eq 0 ]; then
			echo "u-root says neither \"$new\" nor $* in $file: the dispatcher text changed" >&2
			exit 1
		fi
	done
	if [ "$found" -eq 0 ]; then
		echo "no gobusybox register.go found: the dispatcher text cannot be renamed" >&2
		exit 1
	fi
}
# The word to replace is whatever a previous build left there, not a list of
# names: AKMESH_NAME=bssdsh followed by a plain build has to work in either
# order, and a blacklist of old spellings cannot know every name a build may
# have had.
rewrite_in_dispatcher() {
	new=$1
	pattern=$2
	found=0
	for file in $DISPATCHER_FILES; do
		[ -f "$file" ] || continue
		found=1
		if grep -q "$new" "$file"; then
			continue
		fi
		if ! grep -q "$pattern" "$file"; then
			echo "u-root no longer says \"$pattern\" in $file: the dispatcher text changed" >&2
			exit 1
		fi
		sed -i "s/$pattern [A-Za-z0-9_.-]*/$pattern $NAME/" "$file"
		if ! grep -q "$new" "$file"; then
			echo "could not put \"$new\" into $file" >&2
			exit 1
		fi
	done
	if [ "$found" -eq 0 ]; then
		echo "no gobusybox register.go found: the dispatcher text cannot be renamed" >&2
		exit 1
	fi
}
rewrite_in_dispatcher "not present in $NAME" "not present in"
# The second message reads "<name> dispatched %q", so the name stands before the
# word and needs its own expression; nothing after "dispatched" is the name.
for file in $DISPATCHER_FILES; do
	[ -f "$file" ] || continue
	grep -q "$NAME dispatched" "$file" && continue
	if ! grep -q ' dispatched ' "$file"; then
		echo "u-root no longer says \" dispatched \" in $file: the dispatcher text changed" >&2
		exit 1
	fi
	# Anchored on the quote and on " dispatched %q": the file also declares
	# `var dispatched atomic.Pointer[string]`, and an unanchored pattern turned
	# that declaration into a syntax error.
	sed -i 's/"\([A-Za-z0-9_.-]*\) dispatched %q/"'"$NAME"' dispatched %q/' "$file"
done

# makebb embeds that source with //go:embed, so a tool installed before the
# rename still carries the old word. Rebuild it once now that the source is
# patched; if the word survives anyway, stop rather than ship it.
# The copied source must carry this build's name, not a previous one. Asking for
# the name instead of listing the old wordings keeps a renamed build (AKMESH_NAME)
# working: with a blacklist, makebb kept embedding "akmesh" and the message that
# reached the binary named the wrong tool.
if ! tr -d '\000' < "$MAKEBB" | grep -q "not present in $NAME"; then
	install_makebb
	if ! tr -d '\000' < "$MAKEBB" | grep -q "not present in $NAME"; then
		echo "makebb does not carry \"not present in $NAME\": the rename cannot reach the build" >&2
		exit 1
	fi
fi

# Bad command lines must not look like success. Two u-root commands print their
# usage and carry on; the patches and their guards live in patch-uroot.sh.
"$ROOT/scripts/patch-uroot.sh" "$UROOT_DIR"

# The workspace is what lets makebb see our own packages. Its Go version has to
# be the one our go.mod asks for: a workspace that names an older version stops
# the build with "module ../.. requires go >= 1.27.1, but go.work lists 1.27.0".
GOVER=$(awk '$1 == "go" { print $2; exit }' "$ROOT/go.mod")
[ -n "$GOVER" ] || GOVER=1.27.1
WORKFILE="$ROOT/.build/go.work"
cat > "$WORKFILE" <<EOF
go $GOVER

use (
	$UROOT_DIR
	$ROOT
)
EOF
export GOWORK="$WORKFILE"

# Every name our own tools provide. Where u-root also builds the same name for
# the target, u-root wins: it is already in the list and needs no extra bytes.
OURS_ALL="id awk basename certinfo ls head find grep chmod cp echo false curl cut df diff dirname du env file free hostname gosh iconv jq iconv kill killall login mkcert md5sum nl patch ps sed rg sha1sum sha256sum sha512sum shuf sort split stat sync tac timeout truncate uname verify wget sh policy ssh scp pwd true xargs tar hexdump base64 wc tr touch tracedeps any2md html2md httpd docx2md rtf2md pdf2md md2txt-l md2pdf md2docx md2epub epub2md pdf2html html2docx html2epub epub2html"

PROFILE_FILE="$ROOT/scripts/profiles/$PROFILE.txt"
if [ ! -f "$PROFILE_FILE" ]; then
	echo "unknown profile $PROFILE; available:" >&2
	ls "$ROOT/scripts/profiles" | sed 's/\.txt$//' >&2
	exit 2
fi
TAGS=""
PROFILE_EXCLUDE=""
PROFILE_KEEP=""
while read -r key value; do
	case "$key" in
	''|'#'*) continue ;;
	tags) TAGS="$value" ;;
	exclude) PROFILE_EXCLUDE="$PROFILE_EXCLUDE $value" ;;
	keep) PROFILE_KEEP="$PROFILE_KEEP $value" ;;
	*) echo "profile $PROFILE: unknown line '$key'" >&2; exit 2 ;;
	esac
done < "$PROFILE_FILE"

# "keep" is the short way to a small binary: everything that is not named is
# excluded, ours and u-root's alike (acmesh-94e.11).
if [ -n "$PROFILE_KEEP" ]; then
	KEPT=" $PROFILE_KEEP "
	if [ -n "$PROFILE_EXCLUDE" ]; then
		echo "profile $PROFILE: keep and exclude together are ambiguous" >&2
		exit 2
	fi
fi

# One name per line: cmds.txt groups names on a line, and the profile excludes
# compare whole lines. Without this, "exclude ping" never matched "ping netcat".
# A line may start with "linux:", which marks u-root commands that only build
# for Linux (mount, ip, mknod): they are left out of the Windows build instead
# of breaking it.
NAMES=$(awk -v goos="$GOOS" '
	/^[[:space:]]*#/ { next }
	{
		line = $0
		only = ""
		if (line ~ /^linux:([[:space:]]|$)/) { only = "linux"; sub(/^linux:[[:space:]]*/, "", line) }
		if (only != "" && goos != only) next
		n = split(line, a, /[[:space:]]+/)
		for (i = 1; i <= n; i++) if (a[i] != "") print a[i]
	}' "$ROOT/scripts/cmds.txt")
for c in $PROFILE_EXCLUDE; do
	NAMES=$(echo "$NAMES" | grep -v "^$c$" || true)
done
if [ -n "$PROFILE_KEEP" ]; then
	KEPTNAMES=""
	for c in $NAMES; do
		case "$KEPT" in *" $c "*) KEPTNAMES="$KEPTNAMES $c" ;; esac
	done
	NAMES=$(echo $KEPTNAMES | tr ' ' '\n' | sed '/^$/d')
fi
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
	if [ -n "$PROFILE_KEEP" ]; then
		case "$KEPT" in *" $c "*) ;; *) continue ;; esac
	fi
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
	export GOFLAGS=
# A workspace file somewhere above the build would pull its own module set into
# this build; a stray /tmp/wk/go.work once failed it with a Go version that has
# nothing to do with this repository.
export GOWORK=off"-tags=$TAGS"
	echo "build tags: $TAGS"
fi
# A generator-only run is what a non-Go compiler needs: makebb writes the
# multicall source and the build is done by the caller.
if [ -n "${AKMESH_GEN_DIR:-}" ]; then
	"$MAKEBB" -g -gen-dir "$AKMESH_GEN_DIR" -o "$ROOT/$OUT" $PKGS
	prepare_gen "$AKMESH_GEN_DIR"
	echo "generated: $AKMESH_GEN_DIR"
	exit 0
fi
# makebb generates the multicall main.go, then we build it ourselves: that is
# how the command list becomes ours. The flags below are the ones makebb would
# have used (-a, inlining off, stripped, trimpath) with the same toolchain, so
# the size stays comparable; the size budget further down would notice a
# difference of a megabyte.
rm -rf "$GEN_DIR"
"$MAKEBB" -g -gen-dir "$GEN_DIR" -o "$ROOT/$OUT" $PKGS
prepare_gen "$GEN_DIR"
(
	cd "$GEN_DIR" &&
		GOWORK=off GOTOOLCHAIN=auto \
			go build -o "$ROOT/$OUT" -mod=vendor -a -gcflags=all=-l \
			-ldflags "-s -w -buildid= -X akmesh/internal/appname.Name=$NAME" -trimpath .
)
# The rename above must reach the binary. A vendor/module mismatch would build
# fine and still ship u-root's wording, so the build checks its own output.
if ! tr -d '\000' < "$ROOT/$OUT" | grep -q "not present in $NAME"; then
	echo "$OUT does not say \"not present in $NAME\": the dispatcher rename did not reach the build" >&2
	exit 1
fi
# And the list has to be ours. The marker is a group title only internal/cmdlist
# has, so a build that ignored the patch cannot pass as a grouped list.
if ! tr -d '\000' < "$ROOT/$OUT" | grep -q 'Files and text:'; then
	echo "$OUT has no grouped command list: the list patch did not reach the build" >&2
	exit 1
fi
ls -l "$ROOT/$OUT"

# Size budget (acmesh-94e.15): a warning above 12 MB, a hard stop above 16 MB,
# and a growth of more than 1 MB against the previous build of the same target
# named in the build report. The last size is kept per target, not per profile,
# because the profile name already says which binary this is.
SIZE=$(stat -c %s "$ROOT/$OUT" 2>/dev/null || stat -f %z "$ROOT/$OUT")
STATE="$ROOT/.build/last-size-$GOOS-$PROFILE"
if [ -f "$STATE" ]; then
	LAST=$(cat "$STATE")
	GROWTH=$((SIZE - LAST))
	if [ "$GROWTH" -gt 1048576 ]; then
		echo "size: $OUT grew by $GROWTH bytes since the last build of this target ($LAST -> $SIZE)" >&2
	fi
fi
echo "$SIZE" > "$STATE"
# The warning line was 12 MB, then 13 MB for httpd. It is 25 MB since
# 19.11.2026 (Martin): the document writers need goldmark with its renderers,
# measured in acmesh-54s.1 as goldmark+GFM 1.1 MB, a chroma-free goldmark-pdf
# fork 6.4 to 7.5 MB, go-epub 2.5 MB and a docx writer 0.4 MB. With all of them
# the binary lands near 23 MB, so 13 MB would cry wolf on every build. The hard
# stop moves with it to 30 MB: a limit of 25 on a binary that is expected to
# reach 23 leaves no room to notice a regression.
if [ "$SIZE" -gt 30000000 ]; then
	echo "size: $OUT is $SIZE bytes, over the hard limit of 30000000 (acmesh-94e.15)" >&2
	exit 1
fi
if [ "$SIZE" -gt 25000000 ]; then
	echo "size: $OUT is $SIZE bytes, over the warning line of 25000000 (acmesh-94e.15, acmesh-m5j, acmesh-54s.1)" >&2
fi
