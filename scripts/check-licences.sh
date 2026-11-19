#!/bin/sh
# Refuses a build whose dependency tree carries a licence we cannot ship.
#
# Why this exists: acmesh is distributed as a binary, and the licence of that
# binary is the sum of its dependencies. Two cases bit us while planning
# md2docx - fumiama/go-docx is AGPL-3.0, and zikani03/goldmark-docx has no
# licence file at all - so the check is part of the build gate and not a note in
# a ticket (Martin, 19.11.2026).
#
# What it does, without network access:
#   1. every module in the build list is looked up in the module cache and its
#      licence file is read;
#   2. a licence that names the GNU Affero General Public License fails the
#      build;
#   3. a module without any licence file fails the build, because silence is not
#      permission;
#   4. a short list of known-bad modules fails the build even if the cache has no
#      licence text for them.
#
# Run from the repository root: ./scripts/check-licences.sh

set -eu
cd "$(dirname "$0")/.."

export GOTOOLCHAIN=auto
export GOCACHE="${GOCACHE:-$PWD/.cache}"
export GOFLAGS="${GOFLAGS:--mod=mod}"
# The module cache is wherever the toolchain puts it; asking beats guessing.
MODCACHE=$(go env GOMODCACHE)

forbidden="github.com/fumiama/go-docx github.com/zikani03/goldmark-docx github.com/unidoc/unioffice"
status=0
checked=0

# Only the modules that are actually compiled into the binary matter: a test
# helper that never reaches the binary cannot make a licensing claim about it.
# "go list -deps" names the packages of the build, their .Module the owners.
mods=$(go list -deps -f '{{with .Module}}{{.Path}}@{{.Version}}{{end}}' ./... 2>/dev/null | sed '/^$/d' | sort -u)
[ -n "$mods" ] || { echo "check-licences: no modules listed"; exit 1; }

for entry in $mods; do
	# go list prints "module version"; split at the first @ or space.
	case "$entry" in
	*@*) mod="${entry%@*}"; ver="${entry##*@}" ;;
	*) continue ;;
	esac
	# The module cache escapes upper case as !x.
	esc=$(printf '%s' "$mod" | sed 's/\([A-Z]\)/!\L\1/g')
	dir="$MODCACHE/$esc@$ver"
	[ -d "$dir" ] || continue
	checked=$((checked + 1))

	for bad in $forbidden; do
		if [ "$mod" = "$bad" ]; then
			echo "check-licences: $mod is on the forbidden list"
			status=1
		fi
	done

	lic=$(find "$dir" -maxdepth 1 -type f \( -iname 'license*' -o -iname 'licence*' -o -iname 'copying*' \) | head -n 1)
	if [ -z "$lic" ]; then
		echo "check-licences: $mod has no licence file (refused: silence is not permission)"
		status=1
		continue
	fi
	if head -n 40 "$lic" | grep -qi "gnu affero"; then
		echo "check-licences: $mod is AGPL, which we cannot ship"
		status=1
	fi
done

if [ "$checked" -eq 0 ]; then
	echo "check-licences: no module cache entries found, nothing was proven"
	exit 1
fi

if [ "$status" -ne 0 ]; then
	echo "check-licences: FAILED"
	exit 1
fi
echo "check-licences: $checked modules checked, all licences usable"
