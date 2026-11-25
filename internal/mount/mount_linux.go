// Package mount implements the mount command.
//
// It exists for one reason u-root's mount cannot serve: WSL. When a WSL
// distribution starts, /init runs
//
//	/bin/mount --internal-only -t drvfs C: /mnt/c -o <options>
//
// (measured 25.11.2026: dmesg says "/bin/mount failed with status 0x200" when
// our mount refuses the filesystem type). The drvfs filesystem is not a kernel
// filesystem: WSL handles it in /init itself, which offers the entrypoint
// "mount.drvfs" - picked by argv[0] (see wsl.dev/technical-documentation/drvfs
// and src/linux/init/drvfs.cpp, MountDrvfsEntry). So this command forwards the
// drvfs case to that entrypoint and keeps everything else as a plain mount(2).
//
// What it deliberately does not do: -o loop (u-root's mount had it via
// pkg/mount/loop), mtab bookkeeping, and mounting from /etc/fstab without
// arguments. The WSL case needs none of them, and a smaller surface is the only
// reason to replace a working command at all.
package mount

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

// drvfsInit is where WSL puts its own /init; it is a file the distribution does
// not ship, WSL mounts it. Overridable so a test can point at a stub.
var drvfsInit = "/init"

// drvfsArgs builds the argv for the mount.drvfs entrypoint. WSL's
// MountDrvfsEntry reads Argv[1] as source, Argv[2] as target and Argv[4] as the
// option string, so the shape is "mount.drvfs SOURCE TARGET -o OPTIONS".
func drvfsArgs(source, target, opts string) []string {
	args := []string{"mount.drvfs", source, target}
	if opts != "" {
		args = append(args, "-o", opts)
	}
	return args
}

// runDrvfs hands the mount to /init. The exit code is WSL's, so a failure there
// stays a failure here instead of being reported as success.
func runDrvfs(source, target, opts string, stderr *os.File) int {
	argv := drvfsArgs(source, target, opts)
	if _, err := os.Stat(drvfsInit); err != nil {
		fmt.Fprintf(stderr, "mount: %s is missing; drvfs is a WSL filesystem and needs WSL's /init\n", drvfsInit)
		return 1
	}
	cmd := exec.Command(drvfsInit)
	cmd.Args = argv
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "mount: %v\n", err)
		return 1
	}
	return 0
}

// mountFlags are the options that map onto a mount(2) flag; everything else is
// passed through as a filesystem-specific option, which is what util-linux does.
var mountFlags = map[string]struct {
	flag  uintptr
	clear bool
}{
	"ro":         {unix.MS_RDONLY, false},
	"rw":         {unix.MS_RDONLY, true},
	"nosuid":     {unix.MS_NOSUID, false},
	"suid":       {unix.MS_NOSUID, true},
	"nodev":      {unix.MS_NODEV, false},
	"dev":        {unix.MS_NODEV, true},
	"noexec":     {unix.MS_NOEXEC, false},
	"exec":       {unix.MS_NOEXEC, true},
	"sync":       {unix.MS_SYNCHRONOUS, false},
	"async":      {unix.MS_SYNCHRONOUS, true},
	"noatime":    {unix.MS_NOATIME, false},
	"atime":      {unix.MS_NOATIME, true},
	"nodiratime": {unix.MS_NODIRATIME, false},
	"diratime":   {unix.MS_NODIRATIME, true},
	"relatime":   {unix.MS_RELATIME, false},
	"norelatime": {unix.MS_RELATIME, true},
	"dirsync":    {unix.MS_DIRSYNC, false},
	"remount":    {unix.MS_REMOUNT, false},
	"bind":       {unix.MS_BIND, false},
	"rbind":      {unix.MS_BIND | unix.MS_REC, false},
	"mand":       {unix.MS_MANDLOCK, false},
	"nomand":     {unix.MS_MANDLOCK, true},
	"silent":     {unix.MS_SILENT, false},
	"loud":       {unix.MS_SILENT, true},
}

// splitOptions turns a comma list into a flag word and the remaining data
// string, the way mount(8) does.
func splitOptions(list []string) (uintptr, string) {
	var flags uintptr
	var data []string
	for _, opt := range list {
		for o := range strings.SplitSeq(opt, ",") {
			if o == "" {
				continue
			}
			if f, ok := mountFlags[o]; ok {
				if f.clear {
					flags &^= f.flag
				} else {
					flags |= f.flag
				}
				continue
			}
			data = append(data, o)
		}
	}
	return flags, strings.Join(data, ",")
}

// Run is the command: mount [-r] [-t TYPE] [-o OPTS] SOURCE TARGET.
//
// Without SOURCE and TARGET it prints the mounted filesystems, which is what
// "mount" alone does everywhere else.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var (
		readOnly bool
		fsType   string
		options  []string
		pos      []string
	)
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, "usage: mount [-r] [-t TYPE] [-o OPTS] SOURCE TARGET")
			fmt.Fprintln(stdout, "       mount                                  list filesystems")
			fmt.Fprintln(stdout, "  -t drvfs is handed to WSL (mount.drvfs in /init); everything else is mount(2)")
			return 0
		// Flags that change nothing here but that WSL and util-linux pass:
		// --internal-only marks the mount WSL starts itself, -i says "do not
		// consult fstab", -n "do not write mtab", -v "be verbose", -f "fake",
		// -s "sloppy", -w "read-write". Without this, "-i" is taken for the
		// source and the mount lands on the wrong paths (measured 25.11.2026:
		// /init calls back into /bin/mount with "-i -t 9p ...").
		case a == "--internal-only" || a == "-i" || a == "--internal-only=true" ||
			a == "-n" || a == "-v" || a == "-f" || a == "-s" || a == "-w" ||
			a == "-l" || a == "-a":
		case a == "-r" || a == "--read-only":
			readOnly = true
		case a == "-t" && i+1 < len(args):
			i++
			fsType = args[i]
		case strings.HasPrefix(a, "-t") && len(a) > 2:
			fsType = a[2:]
		case a == "-o" && i+1 < len(args):
			i++
			options = append(options, args[i])
		case strings.HasPrefix(a, "-o") && len(a) > 2:
			options = append(options, a[2:])
		case a == "--":
			pos = append(pos, args[i+1:]...)
			i = len(args)
		default:
			pos = append(pos, a)
		}
	}

	if len(pos) == 0 {
		for _, p := range []string{"/proc/self/mounts", "/proc/mounts"} {
			if b, err := os.ReadFile(p); err == nil {
				_, _ = stdout.Write(b)
				return 0
			}
		}
		fmt.Fprintln(stderr, "mount: cannot read /proc/mounts")
		return 1
	}
	if len(pos) < 2 {
		fmt.Fprintln(stderr, "usage: mount [-r] [-t TYPE] [-o OPTS] SOURCE TARGET")
		return 1
	}
	source, target := pos[0], pos[1]
	optString := strings.Join(options, ",")

	// The WSL case, and the reason this command exists.
	if fsType == "drvfs" {
		return runDrvfs(source, target, optString, stderr)
	}

	flags, data := splitOptions(options)
	if readOnly {
		flags |= unix.MS_RDONLY
	}
	if err := unix.Mount(source, target, fsType, flags, data); err != nil {
		// The type is named in the message: when a kernel refuses it, that is
		// the word the next person searches for (see the package comment).
		if fsType != "" {
			fmt.Fprintf(stderr, "mount: %s: %v\n", fsType, err)
		}
		fmt.Fprintf(stderr, "mount: mounting %s on %s failed: %v\n", source, target, err)
		return 1
	}
	return 0
}
