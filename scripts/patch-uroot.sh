#!/bin/sh
# Guards for the u-root commands we ship: patches that make a bad command line
# fail the way GNU does instead of succeeding silently.
#
# Every patch is a literal replacement with two guards: if the new text is
# already there the step is skipped (the checkout is patched once per build),
# and if the old text is missing the build stops instead of shipping an
# unpatched command. That is the same rule as the dispatcher rename in
# build-bb.sh.
#
# Why patch at all: a command that prints its usage and then exits 0 tells an
# agent it succeeded. AGENTS.md forbids per-command path checks in Go and
# forbids inventing flags; it does not ask for silent success, and the
# dispatcher wording is already patched the same way.

set -eu

UROOT_DIR=${1:?usage: patch-uroot.sh <u-root checkout>}

patch_file() {
	file=$1
	oldfile=$2
	newfile=$3
	if [ ! -f "$file" ]; then
		echo "patch-uroot: $file is missing" >&2
		exit 1
	fi
	if cmp -s "$file" /dev/null; then
		echo "patch-uroot: $file is empty" >&2
		exit 1
	fi
	python3 - "$file" "$oldfile" "$newfile" <<'PY'
import sys
path, old_path, new_path = sys.argv[1:4]
src = open(path).read()
old = open(old_path).read()
new = open(new_path).read()
if new in src:
    sys.exit(0)
if old not in src:
    sys.stderr.write("patch-uroot: expected text not found in %s\n" % path)
    sys.exit(1)
open(path, "w").write(src.replace(old, new, 1))
PY
}

# The checkout keeps local modifications when it is already on the pinned
# commit, so a build after a patch-text change would patch a patched file.
# Start from the committed text every time.
if [ -d "$UROOT_DIR/.git" ]; then
	git -C "$UROOT_DIR" checkout -- cmds/core/ls/ls.go cmds/core/chmod/chmod.go 2>/dev/null || true
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# 1. ls: an unknown flag printed the usage and then listed the directory with
#    exit 0. "-Z", "-t" and "-1" all landed here, and an agent reads exit 0 as
#    success.
cat > "$TMP/ls.old" <<'EOF'
	f.Parse(unixflag.ArgsToGoArgs(args[1:]))
	return c.list(f.Args())
EOF
cat > "$TMP/ls.new" <<'EOF'
	// akmesh: an unknown flag used to print the usage and then list the
	// directory anyway, with exit 0. Fail like GNU instead.
	if err := f.Parse(unixflag.ArgsToGoArgs(args[1:])); err != nil {
		os.Exit(2)
	}
	return c.list(f.Args())
EOF
patch_file "$UROOT_DIR/cmds/core/ls/ls.go" "$TMP/ls.old" "$TMP/ls.new"

# 2. chmod: the class letters were mandatory, so GNU's "+x" failed to parse;
#    and the failure message never reached the user because main discarded the
#    error. Both are fixed here: the class may be omitted (GNU means "a"), and
#    a usage error prints and exits 2.
cat > "$TMP/chmod-mode.old" <<'EOF'
	reMode := regexp.MustCompile("^([ugoa]+)([-+=])(.*)")
	m := reMode.FindStringSubmatch(modeString)
EOF
cat > "$TMP/chmod-mode.new" <<'EOF'
	reMode := regexp.MustCompile("^([ugoa]*)([-+=])(.*)")
	m := reMode.FindStringSubmatch(modeString)
	// akmesh: GNU allows the class to be omitted; "+x" means "a+x".
	if len(m) >= 2 && m[1] == "" {
		m[1] = "a"
	}
EOF
patch_file "$UROOT_DIR/cmds/core/chmod/chmod.go" "$TMP/chmod-mode.old" "$TMP/chmod-mode.new"

cat > "$TMP/chmod-main.old" <<'EOF'
	if err := command(os.Stderr, *recursive, *reference).run(flag.Args()...); err != nil {
		os.Exit(1)
	}
EOF
cat > "$TMP/chmod-main.new" <<'EOF'
	if err := command(os.Stderr, *recursive, *reference).run(flag.Args()...); err != nil {
		// akmesh: the error used to disappear. GNU chmod exits 1 for a bad
		// mode and for a missing operand, so keep that code.
		fmt.Fprintln(os.Stderr, "chmod:", err)
		os.Exit(1)
	}
EOF
patch_file "$UROOT_DIR/cmds/core/chmod/chmod.go" "$TMP/chmod-main.old" "$TMP/chmod-main.new"

echo "u-root commands patched: ls (exit 2 on a bad flag), chmod (+x, error message)"
