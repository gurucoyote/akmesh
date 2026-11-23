//go:build !windows

// Package akinit is init as PID 1 in a container: it starts the services from
// /etc/inittab, reaps every orphan, forwards SIGTERM to its children and ends
// when the last service does (acmesh-b97).
//
// Why our own and not u-root's init: that one runs a command list once and then
// waits for orphans at exit. As PID 1 it never reaps during the run (finished
// children stay as Z lines), it ignores SIGTERM - the kernel gives PID 1 the
// default action "ignore" - so `docker stop` runs into its 10 second timeout,
// and it has no way to restart a service. Those three are exactly what a
// container needs.
//
// What this does NOT do: capture the children's output (that is acmesh-17s), no
// getty handling (acmesh-bu1), no runlevels, no syslog, no mount of /proc.
package akinit

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// action is the inittab action, the subset BusyBox understands and a container
// uses.
type action string

const (
	// sysinit runs before anything else and is waited for, in file order.
	sysinit action = "sysinit"
	// wait runs after sysinit and is waited for.
	wait action = "wait"
	// once runs after sysinit, not waited for.
	once action = "once"
	// respawn is restarted whenever it ends.
	respawn action = "respawn"
	// shutdown runs on SIGTERM after the other services were stopped.
	shutdown action = "shutdown"
)

// entry is one line of /etc/inittab.
type entry struct {
	ID      string // first field, kept for messages only
	Action  action
	Command string
	Line    int
}

// ParseInittab reads the BusyBox format:
//
//	id:runlevels:action:command
//
// Runlevels are accepted and ignored: a container has one. Comments start with
// "#", empty lines and lines without a command are skipped, and an unknown
// action is an error that names the line - a service that is silently not
// started is the kind of failure this project treats as a defect.
func ParseInittab(r io.Reader) ([]entry, error) {
	var out []entry
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.SplitN(text, ":", 4)
		if len(fields) != 4 {
			return nil, fmt.Errorf("line %d: expected id:runlevels:action:command, got %q", line, text)
		}
		act := action(strings.TrimSpace(fields[2]))
		command := strings.TrimSpace(fields[3])
		switch act {
		case sysinit, wait, once, respawn, shutdown:
		default:
			return nil, fmt.Errorf("line %d: unknown action %q (known: sysinit, wait, once, respawn, shutdown)", line, act)
		}
		if command == "" {
			return nil, fmt.Errorf("line %d: no command", line)
		}
		out = append(out, entry{ID: strings.TrimSpace(fields[0]), Action: act, Command: command, Line: line})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// child is one running service.
type child struct {
	pid   int
	ent   entry
	cmd   *exec.Cmd
	start time.Time
}

// Runner is the init. The syscalls and the clock are fields so that the tests can
// drive respawn, reaping and shutdown without being PID 1.
type Runner struct {
	Entries []entry
	Stdout  io.Writer
	Stderr  io.Writer
	// Grace is how long a SIGTERM may take before SIGKILL. The container
	// runtime's stop timeout is 10s by default; 3s leaves room for it.
	Grace time.Duration
	// RespawnDelay is the pause before a service is started again; it grows
	// when a service keeps dying at once, up to RespawnMax.
	RespawnDelay time.Duration
	RespawnMax   time.Duration

	now   func() time.Time
	spawn func(entry) (*exec.Cmd, error)
	kill  func(pid int, sig syscall.Signal) error
	sleep func(time.Duration)

	mu        sync.Mutex
	children  map[int]*child
	fastFails map[string]int
	stopping  bool
}

// New returns a runner with the real syscalls and a 3 second grace.
func New(entries []entry, stdout, stderr io.Writer) *Runner {
	return &Runner{
		Entries:      entries,
		Stdout:       stdout,
		Stderr:       stderr,
		Grace:        3 * time.Second,
		RespawnDelay: 100 * time.Millisecond,
		RespawnMax:   5 * time.Second,
		now:          time.Now,
		spawn:        defaultSpawn(stdout, stderr),
		kill:         syscall.Kill,
		sleep:        time.Sleep,
		children:     map[int]*child{},
		fastFails:    map[string]int{},
	}
}

// defaultSpawn starts a command through /bin/sh, the way inittab lines are
// meant (one line, own arguments, no shell parsing surprises).
func defaultSpawn(stdout, stderr io.Writer) func(entry) (*exec.Cmd, error) {
	return func(e entry) (*exec.Cmd, error) {
		cmd := exec.Command("/bin/sh", "-c", e.Command)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		// Own process group: a shutdown signal reaches the whole pipeline, and
		// a shell that ignores SIGTERM does not keep the group alive alone.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		return cmd, nil
	}
}

// Run is the loop: sysinit, then the long-running services, then reap until
// nothing is left or a signal ends it.
func (r *Runner) Run() int {
	if os.Getpid() != 1 {
		fmt.Fprintf(r.Stderr, "init: not PID 1 (pid %d); signals of children still arrive here, but orphans are adopted by whoever is PID 1\n", os.Getpid())
	}

	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGCHLD, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer signal.Stop(sigs)

	// sysinit and wait entries run in file order and are waited for; a failure
	// is reported but does not stop the others (BusyBox behaves the same).
	for _, e := range r.Entries {
		if e.Action != sysinit && e.Action != wait {
			continue
		}
		if code := r.runAndWait(e); code != 0 {
			fmt.Fprintf(r.Stderr, "init: %s (%s) ended with exit %d\n", e.Command, e.Action, code)
		}
	}

	// once and respawn: started, not waited for.
	for _, e := range r.Entries {
		if e.Action != once && e.Action != respawn {
			continue
		}
		if err := r.start(e); err != nil {
			fmt.Fprintf(r.Stderr, "init: cannot start %q: %v\n", e.Command, err)
		}
	}

	last := 0
	for {
		r.mu.Lock()
		running := len(r.children)
		wantRespawn := r.hasAction(respawn)
		r.mu.Unlock()
		if running == 0 && !wantRespawn {
			return last
		}

		select {
		case sig := <-sigs:
			switch sig {
			case syscall.SIGCHLD:
				if code, ok := r.reapAll(); ok {
					last = code
				}
			default:
				// PID 1 ignores SIGTERM by default; forwarding it is the
				// whole point of this program in a container.
				return r.shutdown()
			}
		default:
			// No signal at hand: sleep a little so the loop does not spin,
			// then reap whatever ended and start missing respawn services -
			// a service whose start failed once must not end the loop, and it
			// must not spin either.
			r.sleep(20 * time.Millisecond)
			if code, ok := r.reapAll(); ok {
				last = code
			}
			r.ensureRespawn()
		}
	}
}

// shutdown forwards SIGTERM to every child, waits at most Grace and then kills.
// The shutdown entries run afterwards, before init returns.
func (r *Runner) shutdown() int {
	r.mu.Lock()
	r.stopping = true
	pids := make([]int, 0, len(r.children))
	for pid := range r.children {
		pids = append(pids, pid)
	}
	r.mu.Unlock()
	sort.Ints(pids)

	for _, pid := range pids {
		if err := r.kill(-pid, syscall.SIGTERM); err != nil {
			_ = r.kill(pid, syscall.SIGTERM)
		}
	}
	deadline := r.now().Add(r.Grace)
	for r.now().Before(deadline) {
		if r.countChildren() == 0 {
			break
		}
		r.sleep(20 * time.Millisecond)
		r.reapAll()
	}
	r.mu.Lock()
	left := make([]int, 0, len(r.children))
	for pid := range r.children {
		left = append(left, pid)
	}
	r.mu.Unlock()
	for _, pid := range left {
		_ = r.kill(-pid, syscall.SIGKILL)
		_ = r.kill(pid, syscall.SIGKILL)
	}
	// Give the kernel a moment to hand the deaths back.
	for i := 0; i < 50 && r.countChildren() > 0; i++ {
		r.sleep(20 * time.Millisecond)
		r.reapAll()
	}
	for _, e := range r.Entries {
		if e.Action == shutdown {
			if code := r.runAndWait(e); code != 0 {
				fmt.Fprintf(r.Stderr, "init: %s (shutdown) ended with exit %d\n", e.Command, code)
			}
		}
	}
	return 0
}

// start launches one service and remembers it.
func (r *Runner) start(e entry) error {
	cmd, err := r.spawn(e)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	r.mu.Lock()
	r.children[cmd.Process.Pid] = &child{pid: cmd.Process.Pid, ent: e, cmd: cmd, start: r.now()}
	r.mu.Unlock()
	return nil
}

// runAndWait starts a service and waits for that one pid, at most until it ends.
func (r *Runner) runAndWait(e entry) int {
	cmd, err := r.spawn(e)
	if err != nil {
		fmt.Fprintf(r.Stderr, "init: cannot start %q: %v\n", e.Command, err)
		return 1
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(r.Stderr, "init: cannot start %q: %v\n", e.Command, err)
		return 1
	}
	pid := cmd.Process.Pid
	for {
		var status syscall.WaitStatus
		wpid, err := syscall.Wait4(pid, &status, 0, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			fmt.Fprintf(r.Stderr, "init: wait for %q: %v\n", e.Command, err)
			return 1
		}
		if wpid == pid {
			return exitCode(status)
		}
	}
}

// reapAll collects every finished child without blocking and restarts the
// respawn ones. It reports the last exit code it saw.
func (r *Runner) reapAll() (int, bool) {
	last, seen := 0, false
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if err != nil || pid <= 0 {
			return last, seen
		}
		last, seen = exitCode(status), true
		r.mu.Lock()
		c, known := r.children[pid]
		delete(r.children, pid)
		stopping := r.stopping
		r.mu.Unlock()
		if !known {
			// An adopted orphan: reaping it is the duty, nothing to restart.
			continue
		}
		if !stopping && c.ent.Action == respawn {
			r.noteExit(c)
		}
		if !stopping {
			fmt.Fprintf(r.Stderr, "init: %s exited with %d\n", c.ent.Command, exitCode(status))
		}
	}
}

// noteExit records whether a respawn service died immediately, which makes
// ensureRespawn wait longer before starting it again - a fork bomb is worse than
// a dead service. The start itself happens there, not in the reaper.
func (r *Runner) noteExit(c *child) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.now().Sub(c.start) < time.Second {
		r.fastFails[c.ent.Command]++
		return
	}
	delete(r.fastFails, c.ent.Command)
}

// ensureRespawn starts every respawn service that has no live child. The
// wait loop sleeps between attempts, so a service that cannot start at all is
// retried slowly and reported, not turned into a busy loop.
func (r *Runner) ensureRespawn() {
	r.mu.Lock()
	alive := map[string]bool{}
	for _, c := range r.children {
		alive[c.ent.Command] = true
	}
	r.mu.Unlock()
	for _, e := range r.Entries {
		if e.Action != respawn || alive[e.Command] {
			continue
		}
		r.mu.Lock()
		n := r.fastFails[e.Command]
		r.mu.Unlock()
		if n > 0 {
			delay := r.RespawnDelay
			for i := 1; i < n && delay < r.RespawnMax; i++ {
				delay *= 2
			}
			if delay > r.RespawnMax {
				delay = r.RespawnMax
			}
			if delay > 100*time.Millisecond {
				// Only the growing part is waited here; the plain first retry
				// happens at once, which keeps the image probe fast.
				r.sleep(delay)
			}
		}
		if err := r.start(e); err != nil {
			fmt.Fprintf(r.Stderr, "init: cannot start %q: %v\n", e.Command, err)
			r.mu.Lock()
			r.fastFails[e.Command]++
			r.mu.Unlock()
		}
	}
}

func (r *Runner) hasAction(a action) bool {
	for _, e := range r.Entries {
		if e.Action == a {
			return true
		}
	}
	return false
}

func (r *Runner) countChildren() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.children)
}

func exitCode(status syscall.WaitStatus) int {
	if status.Exited() {
		return status.ExitStatus()
	}
	if status.Signaled() {
		return 128 + int(status.Signal())
	}
	return 0
}

// Run is the command entry point: parse the file, run, and return the code.
//
//	init [-c /etc/inittab] [command...]
//
// Without a file it runs the commands from the argument list once and returns
// their status - that is the honest answer for a container that was started with
// "init /bin/sh" and has no configuration.
func Run(name string, args []string, stdout, stderr io.Writer) int {
	path := "/etc/inittab"
	var command []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			fmt.Fprintf(stdout, "usage: %s [-c INITTAB] [command...]\n", name)
			fmt.Fprintln(stdout, "  -c FILE   init table (default /etc/inittab)")
			fmt.Fprintln(stdout, "  actions: sysinit, wait, once, respawn, shutdown")
			fmt.Fprintln(stdout, "  without a table the given command runs once and its status is returned")
			return 0
		case a == "-c":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "%s: -c needs a file\n", name)
				return 2
			}
			i++
			path = args[i]
		case strings.HasPrefix(a, "-c"):
			path = strings.TrimPrefix(a, "-c")
		default:
			command = append(command, args[i:]...)
			i = len(args)
		}
	}

	f, err := os.Open(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return 1
		}
		if len(command) == 0 {
			fmt.Fprintf(stderr, "%s: no %s and no command given\n", name, path)
			return 2
		}
		entries := []entry{{ID: "cmd", Action: once, Command: strings.Join(command, " "), Line: 0}}
		return New(entries, stdout, stderr).Run()
	}
	defer f.Close()
	entries, err := ParseInittab(f)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s: %v\n", name, path, err)
		return 1
	}
	if len(entries) == 0 {
		fmt.Fprintf(stderr, "%s: %s: no service\n", name, path)
		return 1
	}
	return New(entries, stdout, stderr).Run()
}

// String makes the entries readable in a test failure.
func (e entry) String() string {
	return strconv.Itoa(e.Line) + ":" + e.ID + ":" + string(e.Action) + ":" + e.Command
}
