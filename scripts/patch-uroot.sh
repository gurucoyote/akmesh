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

# patch_diff applies a unified diff recorded under docs/design/patches, for
# changes that add a file or touch several places at once. It is idempotent: a
# diff that is already applied is skipped, and one that applies neither way
# stops the build instead of shipping an unpatched command.
patch_diff() {
	file=$1
	if [ ! -f "$file" ]; then
		echo "patch-uroot: $file is missing" >&2
		exit 1
	fi
	# Already applied? A diff that creates a file cannot be probed with
	# --reverse once gofmt has touched the copy, so the mark is a symbol the
	# patch introduces.
	case "$(basename "$file")" in
	uroot-tail-gnu.patch)
		if grep -q "func parseTailArgs" "$UROOT_DIR/cmds/core/tail/tail_args.go" 2>/dev/null; then
			return 0
		fi
		;;
	uroot-messages-gnu.patch)
		if grep -q 'fmt.Fprintf(os.Stderr, "rm: %v' "$UROOT_DIR/cmds/core/rm/rm.go" 2>/dev/null; then
			return 0
		fi
		;;
	uroot-date-gnu.patch)
		if grep -q "func parseDateString" "$UROOT_DIR/cmds/core/date/date_parse.go" 2>/dev/null; then
			return 0
		fi
		;;
	esac
	if git -C "$UROOT_DIR" apply --check "$file" 2>/dev/null; then
		git -C "$UROOT_DIR" apply "$file"
		echo "patch-uroot: $(basename "$file") applied"
		return 0
	fi
	echo "patch-uroot: $(basename "$file") applies neither forward nor backward" >&2
	exit 1
}

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
	git -C "$UROOT_DIR" checkout -- cmds/core/chmod/chmod.go 2>/dev/null || true
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# 1. ls is no longer patched here: it is our own command now
#    (internal/ls, acmesh-wx6), and the unknown-flag exit lives in its Run.

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

echo "u-root commands patched: chmod (+x, error message)"

# patch_all is patch_file for a text that occurs more than once and must be
# fixed everywhere (ip prints a broadcast address in two places). The guards
# are the same: skip when the new text is there, stop when the old is missing.
patch_all() {
	file=$1
	oldfile=$2
	newfile=$3
	if [ ! -f "$file" ]; then
		echo "patch-uroot: $file is missing" >&2
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
open(path, "w").write(src.replace(old, new))
PY
}

# 3. ip: a nil broadcast or source address was printed as "<nil>". iproute2
#    leaves the field out, and a script that parses "brd" or "src" saw a value
#    that is not an address (acmesh-er3).
git -C "$UROOT_DIR" checkout -- cmds/core/ip/link_linux.go cmds/core/ip/route_linux.go 2>/dev/null || true

cat > "$TMP/ip-brd.old" <<'EOF'
				if addr.IP.To4() != nil {
					line += fmt.Sprintf(" brd %s", addr.Broadcast)
				}
EOF
cat > "$TMP/ip-brd.new" <<'EOF'
				// akmesh: a nil broadcast address printed as "<nil>";
				// iproute2 leaves the field out (acmesh-er3).
				if addr.IP.To4() != nil && addr.Broadcast != nil {
					line += fmt.Sprintf(" brd %s", addr.Broadcast)
				}
EOF
patch_all "$UROOT_DIR/cmds/core/ip/link_linux.go" "$TMP/ip-brd.old" "$TMP/ip-brd.new"

# The old text covers the declaration and its use together: the new text must
# be unique in the file, or the "already patched" guard matches something else
# and the patch is skipped without a word.
cat > "$TMP/ip-src.old" <<'EOF'
	src := r.Src
	metric := r.Priority

	var detail string

	if cmd.Opts.Details {
		detail = routeTypeStr(r.Type) + " "
	}

	fmt.Fprintf(cmd.Out, routeFmt, detail, dest, name, proto, scope, src, metric)
EOF
cat > "$TMP/ip-src.new" <<'EOF'
	metric := r.Priority

	var detail string

	if cmd.Opts.Details {
		detail = routeTypeStr(r.Type) + " "
	}

	// akmesh: a route without a preferred source printed "src <nil>";
	// iproute2 leaves the field out (acmesh-er3).
	srcStr := ""
	if r.Src != nil {
		srcStr = " src " + r.Src.String()
	}
	fmt.Fprintf(cmd.Out, "%v%v dev %s proto %s scope %s%s metric %d\n", detail, dest, name, proto, scope, srcStr, metric)
EOF
patch_file "$UROOT_DIR/cmds/core/ip/route_linux.go" "$TMP/ip-src.old" "$TMP/ip-src.new"

cat > "$TMP/ip-gw.old" <<'EOF'
	fmt.Fprintf(cmd.Out, defaultFmt, detail, gw, name, proto, metric)
EOF
cat > "$TMP/ip-gw.new" <<'EOF'
	// akmesh: a default route without a gateway printed "<nil>".
	if gw == nil {
		fmt.Fprintf(cmd.Out, "%vdefault dev %s proto %s metric %d\n", detail, name, proto, metric)
		return
	}
	fmt.Fprintf(cmd.Out, defaultFmt, detail, gw, name, proto, metric)
EOF
patch_file "$UROOT_DIR/cmds/core/ip/route_linux.go" "$TMP/ip-gw.old" "$TMP/ip-gw.new"

echo "u-root commands patched: ip (no <nil> in addr and route)"

# 4. dhclient: two things a script needs. GNU dhclient takes -4 and -6 to pick
#    the protocol (scripts pass them), and it exits 2 when no lease could be
#    configured; u-root answered 0 in both cases, so "dhclient eth0 || die"
#    never fired (acmesh-er3).
git -C "$UROOT_DIR" checkout -- cmds/core/dhclient/dhclient.go 2>/dev/null || true

cat > "$TMP/dh-imports.old" <<'EOF'
import (
	"context"
	"flag"
	"log"
	"net"
	"time"
EOF
cat > "$TMP/dh-imports.new" <<'EOF'
import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"time"
EOF
patch_file "$UROOT_DIR/cmds/core/dhclient/dhclient.go" "$TMP/dh-imports.old" "$TMP/dh-imports.new"

cat > "$TMP/dh-flags.old" <<'EOF'
	ipv4     = flag.Bool("ipv4", true, "use IPV4")
	ipv6     = flag.Bool("ipv6", true, "use IPV6")
EOF
cat > "$TMP/dh-flags.new" <<'EOF'
	ipv4     = flag.Bool("ipv4", true, "use IPV4")
	ipv6     = flag.Bool("ipv6", true, "use IPV6")
	// akmesh: iproute2 spells the protocol choice -4 and -6 (acmesh-er3).
	flag4 = flag.Bool("4", false, "Use IPv4 only (as iproute2's -4)")
	flag6 = flag.Bool("6", false, "Use IPv6 only (as iproute2's -6)")
EOF
patch_file "$UROOT_DIR/cmds/core/dhclient/dhclient.go" "$TMP/dh-flags.old" "$TMP/dh-flags.new"

cat > "$TMP/dh-main.old" <<'EOF'
func main() {
	flag.Parse()
	if len(flag.Args()) > 1 {
EOF
cat > "$TMP/dh-main.new" <<'EOF'
func main() {
	flag.Parse()
	// akmesh: -4 and -6 are the spellings scripts use (acmesh-er3). Without
	// them a plain call also waits for DHCPv6 and looks like a hang.
	if *flag6 {
		*ipv4 = false
	}
	if *flag4 {
		*ipv6 = false
	}
	if len(flag.Args()) > 1 {
EOF
patch_file "$UROOT_DIR/cmds/core/dhclient/dhclient.go" "$TMP/dh-main.old" "$TMP/dh-main.new"

cat > "$TMP/dh-count.old" <<'EOF'
	for result := range r {
EOF
cat > "$TMP/dh-count.new" <<'EOF'
	configured := 0
	for result := range r {
EOF
patch_file "$UROOT_DIR/cmds/core/dhclient/dhclient.go" "$TMP/dh-count.old" "$TMP/dh-count.new"

cat > "$TMP/dh-ok.old" <<'EOF'
		} else {
			log.Printf("Configured %s with %s", result.Interface.Attrs().Name, result.Lease)
		}
	}
	log.Printf("Finished trying to configure all interfaces.")
EOF
cat > "$TMP/dh-ok.new" <<'EOF'
		} else {
			log.Printf("Configured %s with %s", result.Interface.Attrs().Name, result.Lease)
			configured++
		}
	}
	log.Printf("Finished trying to configure all interfaces.")
	if configured == 0 {
		// akmesh: GNU dhclient exits 2 when no lease could be configured; the
		// old exit 0 told a caller it had worked (acmesh-er3).
		os.Exit(2)
	}
EOF
patch_file "$UROOT_DIR/cmds/core/dhclient/dhclient.go" "$TMP/dh-ok.old" "$TMP/dh-ok.new"

echo "u-root commands patched: dhclient (-4/-6, exit 2 without a lease)"

# 5. mknod: u-root creates device nodes with 0660, BusyBox with 0666 (minus the
#    umask). A script that creates /dev/null and then reads it as a non-root user
#    works under BusyBox and fails here (acmesh-u38).
git -C "$UROOT_DIR" checkout -- cmds/core/mknod/mknod_linux.go 2>/dev/null || true

cat > "$TMP/mknod-perm.old" <<'EOF'
const defaultPerms = 0o660
EOF
cat > "$TMP/mknod-perm.new" <<'EOF'
// akmesh: BusyBox uses 0666 (less the umask), u-root had 0660, so a device node
// was not readable for a non-root user (acmesh-u38).
const defaultPerms = 0o666
EOF
patch_file "$UROOT_DIR/cmds/core/mknod/mknod_linux.go" "$TMP/mknod-perm.old" "$TMP/mknod-perm.new"

echo "u-root commands patched: mknod (0666)"

# 6. losetup: BusyBox prints the first free loop device for -f; u-root has no
#    such flag and exited 2 (acmesh-u38).
git -C "$UROOT_DIR" checkout -- cmds/core/losetup/losetup_linux.go 2>/dev/null || true

cat > "$TMP/losetup-var.old" <<'EOF'
var detach = flag.Bool("d", false, "Detach the device")
EOF
cat > "$TMP/losetup-var.new" <<'EOF'
var (
	detach = flag.Bool("d", false, "Detach the device")
	// akmesh: BusyBox's -f prints the first free loop device (acmesh-u38).
	findFree = flag.Bool("f", false, "Print the first free loop device")
)
EOF
patch_file "$UROOT_DIR/cmds/core/losetup/losetup_linux.go" "$TMP/losetup-var.old" "$TMP/losetup-var.new"

cat > "$TMP/losetup-main.old" <<'EOF'
	flag.Parse()
	args := flag.Args()
	if *detach {
EOF
cat > "$TMP/losetup-main.new" <<'EOF'
	flag.Parse()
	args := flag.Args()
	if *findFree {
		device, err := loop.FindDevice()
		if err != nil {
			log.Fatalf("losetup: no free loop device: %v", err)
		}
		fmt.Println(device)
		os.Exit(0)
	}
	if *detach {
EOF
patch_file "$UROOT_DIR/cmds/core/losetup/losetup_linux.go" "$TMP/losetup-main.old" "$TMP/losetup-main.new"

cat > "$TMP/losetup-import.old" <<'EOF'
import (
	"flag"
	"log"
	"os"
EOF
cat > "$TMP/losetup-import.new" <<'EOF'
import (
	"flag"
	"fmt"
	"log"
	"os"
EOF
patch_file "$UROOT_DIR/cmds/core/losetup/losetup_linux.go" "$TMP/losetup-import.old" "$TMP/losetup-import.new"

echo "u-root commands patched: losetup (-f)"

# tail: -N, -c, -q, -v und die "+N"-Formen, so wie GNU sie liest (acmesh-uzd).
# absolut, weil git apply den Pfad relativ zur Repo-Wurzel aufloest
PATCH_ROOT=$(cd "$(dirname "$0")/.." && pwd)
patch_diff "$PATCH_ROOT/docs/design/patches/uroot-tail-gnu.patch"
patch_diff "$PATCH_ROOT/docs/design/patches/uroot-messages-gnu.patch"
# date: -d, -I, -R, %s, %N und die relativen Formen (acmesh-xli).
patch_diff "$PATCH_ROOT/docs/design/patches/uroot-date-gnu.patch"
