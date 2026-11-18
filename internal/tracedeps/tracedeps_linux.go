//go:build linux && amd64

// Command tracedeps lists the files a program needs while it runs, by tracing
// its syscalls. It answers "what has to go into a distroless image" directly
// instead of reading a trace text (acmesh-n0p).
//
// The tracer is our own, about two hundred lines over x/sys/unix. u-root ships
// one, but importing it would pull the whole module into our dependency graph
// for one command.
package tracedeps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// kind separates the syscalls that use a file from the ones that only look
// whether it exists. A PATH search asks about /usr/bin/cat on the way to the
// real binary; that is not a file the image needs.
type kind int

const (
	use kind = iota
	probe
)

// pathArg says which arguments of a syscall hold a path.
type pathArg struct {
	idx  []int
	kind kind
}

// pathArgs maps the syscall numbers this command knows. The numbers come from
// unix.SYS_*, so there is one table per architecture, not one per kernel.
var pathArgs = map[int]pathArg{
	unix.SYS_OPEN:              {[]int{0}, use},
	unix.SYS_OPENAT:            {[]int{1}, use},
	unix.SYS_EXECVE:            {[]int{0}, use},
	unix.SYS_EXECVEAT:          {[]int{1}, use},
	unix.SYS_READLINK:          {[]int{0}, use},
	unix.SYS_READLINKAT:        {[]int{1}, use},
	unix.SYS_STAT:              {[]int{0}, probe},
	unix.SYS_LSTAT:             {[]int{0}, probe},
	unix.SYS_NEWFSTATAT:        {[]int{1}, probe},
	unix.SYS_STATX:             {[]int{1}, probe},
	unix.SYS_ACCESS:            {[]int{0}, probe},
	unix.SYS_FACCESSAT:         {[]int{1}, probe},
	unix.SYS_FACCESSAT2:        {[]int{1}, probe},
	unix.SYS_CHDIR:             {[]int{0}, probe},
	unix.SYS_MKDIR:             {[]int{0}, probe},
	unix.SYS_MKDIRAT:           {[]int{1}, probe},
	unix.SYS_UNLINK:            {[]int{0}, probe},
	unix.SYS_UNLINKAT:          {[]int{1}, probe},
	unix.SYS_RMDIR:             {[]int{0}, probe},
	unix.SYS_RENAME:            {[]int{0, 1}, probe},
	unix.SYS_RENAMEAT:          {[]int{1, 3}, probe},
	unix.SYS_RENAMEAT2:         {[]int{1, 3}, probe},
	unix.SYS_LINK:              {[]int{0, 1}, probe},
	unix.SYS_LINKAT:            {[]int{1, 3}, probe},
	unix.SYS_SYMLINK:           {[]int{0, 1}, probe},
	unix.SYS_SYMLINKAT:         {[]int{0, 2}, probe},
	unix.SYS_CHMOD:             {[]int{0}, probe},
	unix.SYS_FCHMODAT:          {[]int{1}, probe},
	unix.SYS_CHOWN:             {[]int{0}, probe},
	unix.SYS_LCHOWN:            {[]int{0}, probe},
	unix.SYS_FCHOWNAT:          {[]int{1}, probe},
	unix.SYS_TRUNCATE:          {[]int{0}, probe},
	unix.SYS_UTIMES:            {[]int{0}, probe},
	unix.SYS_STATFS:            {[]int{0}, probe},
	unix.SYS_INOTIFY_ADD_WATCH: {[]int{1}, probe},
}

// skipPrefixes are the paths that say nothing about what has to be copied: the
// kernel's own windows and device nodes.
var skipPrefixes = []string{"/proc", "/sys", "/dev"}

const usage = `usage: tracedeps [-probes] PROG [ARG...]

Runs PROG under ptrace and prints the files it opened or executed, one per
line, sorted and without duplicates. Relative paths are resolved against the
working directory of the process that used them.

  -probes   also print the paths that were only asked about (stat, access),
            which is what a PATH search does on the way to the real binary
`

// Run traces argv and prints the paths it touched. The program's own output
// goes through unchanged, so a script can see both.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	showProbes := false
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "-probes", "--probes":
			showProbes = true
		case "-h", "--help":
			fmt.Fprint(stderr, usage)
			return 0
		default:
			fmt.Fprintf(stderr, "tracedeps: unknown option %s\n%s", args[0], usage)
			return 2
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	paths := map[string]struct{}{}
	found := map[string]bool{}

	// The program itself is the one execve the tracer never sees: the child
	// stops at the exec event, not at a syscall pair (the gap the earlier
	// report on the tracer noted). So it is added from the command line.
	prog := args[0]
	if p, err := exec.LookPath(prog); err == nil {
		prog = p
	}
	if abs, err := filepath.Abs(prog); err == nil {
		paths[abs] = struct{}{}
		found[abs] = true
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &unix.SysProcAttr{Ptrace: true}

	if err := trace(cmd, paths, found); err != nil {
		fmt.Fprintf(stderr, "tracedeps: %v\n", err)
	}

	list := make([]string, 0, len(paths))
	for p := range paths {
		if skip(p) {
			continue
		}
		if !showProbes && !found[p] {
			continue
		}
		list = append(list, p)
	}
	sort.Strings(list)
	for _, p := range list {
		fmt.Fprintln(stdout, p)
	}
	return 0
}

// trace runs cmd under ptrace and collects the paths of its syscalls, including
// those of every child it forks. A path counts as found when at least one
// syscall used it; syscalls that only look at it leave it out of found.
func trace(cmd *exec.Cmd, paths map[string]struct{}, found map[string]bool) error {
	// ptrace events belong to the thread that started the child.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := cmd.Start(); err != nil {
		return err
	}
	first := cmd.Process.Pid

	var ws unix.WaitStatus
	if _, err := unix.Wait4(first, &ws, 0, nil); err != nil {
		return err
	}
	const opts = unix.PTRACE_O_TRACESYSGOOD | unix.PTRACE_O_TRACEFORK |
		unix.PTRACE_O_TRACEVFORK | unix.PTRACE_O_TRACECLONE
	if err := unix.PtraceSetOptions(first, opts); err != nil {
		return err
	}
	// The child waits in its first stop until it is continued.
	if err := unix.PtraceSyscall(first, 0); err != nil {
		return err
	}

	dirs := map[int]string{}

	for {
		pid, err := unix.Wait4(-1, &ws, unix.WALL, nil)
		if err == unix.ECHILD {
			return nil
		}
		if err != nil {
			return err
		}
		if ws.Exited() || ws.Signaled() {
			delete(dirs, pid)
			continue
		}
		if !ws.Stopped() {
			continue
		}

		sig := ws.StopSignal()
		cause := ws.TrapCause()
		switch {
		case sig == unix.SIGTRAP|0x80:
			// A syscall stop. The arguments are read on enter and on exit:
			// the exit stop repeats them in the same registers, and reading
			// both keeps this right when a stop is missing, which is what
			// fork and exec do. Duplicates land in the same map entry.
			collect(pid, dirs, paths, found)
		case sig == unix.SIGTRAP && (cause == unix.PTRACE_EVENT_FORK ||
			cause == unix.PTRACE_EVENT_VFORK ||
			cause == unix.PTRACE_EVENT_CLONE):
			// fork, vfork and clone report an event instead of an exit stop.
			// The new child is stopped and shows up in the next wait, so
			// there is nothing to do here beyond letting both run.
		}
		if err := unix.PtraceSyscall(pid, 0); err != nil && err != unix.ESRCH {
			return err
		}
	}
}

// collect reads the arguments of the syscall the process is stopped in and
// keeps the ones that name a file.
func collect(pid int, dirs map[int]string, paths map[string]struct{}, found map[string]bool) {
	var regs unix.PtraceRegs
	if err := unix.PtraceGetRegs(pid, &regs); err != nil {
		return
	}
	arg, ok := pathArgs[int(regs.Orig_rax)]
	if !ok {
		return
	}
	regs4 := [4]uint64{regs.Rdi, regs.Rsi, regs.Rdx, regs.R10}
	for _, i := range arg.idx {
		addr := uintptr(regs4[i])
		if addr == 0 {
			continue
		}
		name := readString(pid, addr)
		if name == "" {
			continue
		}
		p := resolve(pid, name, dirs)
		paths[p] = struct{}{}
		if arg.kind == use {
			found[p] = true
		}
	}
}

// readString reads a NUL-terminated string out of the traced process with
// PTRACE_PEEKDATA. Reading /proc/<pid>/mem returned EIO on pages of a child
// that had just been forked, which is exactly where the interesting paths are
// (acmesh-n0p).
func readString(pid int, addr uintptr) string {
	var buf []byte
	word := make([]byte, 8)
	for len(buf) < unix.PathMax {
		n, err := unix.PtracePeekData(pid, addr+uintptr(len(buf)), word)
		if err != nil || n == 0 {
			break
		}
		for _, b := range word[:n] {
			if b == 0 {
				if plausible(buf) {
					return string(buf)
				}
				return ""
			}
			buf = append(buf, b)
		}
	}
	if plausible(buf) {
		return string(buf)
	}
	return ""
}

// plausible says whether the bytes look like a name a program would pass, and
// not like a register that happened to hold an address.
func plausible(name []byte) bool {
	if len(name) == 0 || len(name) > unix.PathMax {
		return false
	}
	for _, b := range name {
		if b < 0x20 || b == 0x7f {
			return false
		}
	}
	return true
}

// resolve turns a relative path into an absolute one, using the working
// directory the traced process had at that moment.
func resolve(pid int, name string, dirs map[int]string) string {
	if strings.HasPrefix(name, "/") {
		return filepath.Clean(name)
	}
	cwd, ok := dirs[pid]
	if !ok {
		cwd, _ = os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
		dirs[pid] = cwd
	}
	if cwd == "" {
		return filepath.Clean(name)
	}
	return filepath.Clean(cwd + "/" + name)
}

// skip filters the kernel windows: they name a number, not a file to copy.
func skip(p string) bool {
	if p == "" || !strings.HasPrefix(p, "/") {
		return true
	}
	for _, pre := range skipPrefixes {
		if p == pre || strings.HasPrefix(p, pre+"/") {
			return true
		}
	}
	return false
}
