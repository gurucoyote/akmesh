package crond

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gurucoyote/akmesh/internal/supervise"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// parser accepts 5 fields, 6 fields (seconds first) and the @descriptors.
func parser() cron.Parser {
	return cron.NewParser(
		cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
}

// daemon is one run of the scheduler over a set of files.
type daemon struct {
	// Files are the crontab files given with -c (or the default), Dir is the
	// directory of crontabs. Both are kept apart because a reload must look at
	// the directory again - a file added after the start is the usual reason
	// for a reload.
	Files  []string
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
	// Now is replaceable for tests.
	Now func() time.Time
	// Self is the binary that runs the jobs; jobs go through its own sh so the
	// same crontab works on Linux and Windows without /bin/sh existing.
	Self string
	// Mode and Sink override the supervisor settings (tests, and a caller that
	// wants its own log file).
	Mode supervise.Mode
	Sink supervise.Sink
	// Shell builds the argument list for one job; tests replace it.
	Shell func(command string) []string

	mu      sync.Mutex
	running map[string]bool
	pids    map[int]bool
}

// shellWords are the arguments that turn <self> into a shell reading -c.
func (d *daemon) shellWords(command string) []string {
	if d.Shell != nil {
		return d.Shell(command)
	}
	return []string{d.Self, "sh", "-c", command}
}

// start schedules every job and runs the @reboot ones once.
func (d *daemon) start() (*cron.Cron, int, error) {
	if d.running == nil {
		d.running = map[string]bool{}
		d.pids = map[int]bool{}
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Self == "" {
		self, err := os.Executable()
		if err != nil {
			return nil, 0, err
		}
		d.Self = self
	}
	files := append([]string{}, d.Files...)
	if d.Dir != "" {
		files = append(files, CronDir(d.Dir)...)
	}
	// A file given with -c may also sit in the cron.d directory; reading it
	// twice would run every job twice.
	seen := map[string]bool{}
	unique := files[:0]
	for _, f := range files {
		clean := filepath.Clean(f)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		unique = append(unique, f)
	}
	files = unique
	tab, err := ReadFiles(files)
	if err != nil {
		return nil, 0, err
	}
	sched := cron.New(cron.WithParser(parser()), cron.WithLogger(cron.DiscardLogger))
	bad := 0
	for _, j := range tab.Jobs {
		job := j
		if job.Spec == "@reboot" {
			go d.runJob(job)
			continue
		}
		if _, err := sched.AddFunc(job.Spec, func() { d.runJob(job) }); err != nil {
			// A broken line is named, not skipped silently: this is the failure
			// an operator needs to see in the log, with the line number.
			fmt.Fprintf(d.Stderr, "crond: %s line %d: %v\n", job.Source, job.Line, err)
			bad++
		}
	}
	sched.Start()
	return sched, len(tab.Jobs), nil
}

// ensureMaps makes a daemon usable that was built without start(), which tests
// do to run a single job.
func (d *daemon) ensureMaps() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running == nil {
		d.running = map[string]bool{}
	}
	if d.pids == nil {
		d.pids = map[int]bool{}
	}
	if d.Now == nil {
		d.Now = time.Now
	}
}

// runJob executes one command with the crontab environment and reports its exit.
func (d *daemon) runJob(j Job) {
	d.ensureMaps()
	key := j.Source + ":" + strconv.Itoa(j.Line)
	d.mu.Lock()
	if d.running[key] {
		d.mu.Unlock()
		fmt.Fprintf(d.Stdout, "crond: %s still running, skipping %s\n", key, j.Command)
		return
	}
	d.running[key] = true
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.running, key)
		d.mu.Unlock()
	}()

	start := d.Now()
	fmt.Fprintf(d.Stdout, "crond: %s start %s\n", start.Format("2006-01-02 15:04:05"), j.Command)
	words := d.shellWords(j.Command)
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Env = append(os.Environ(), j.Env...)
	cmd.SysProcAttr = jobProcAttr()
	sess, err := supervise.Start(cmd, supervise.Config{
		Mode: d.mode(), Sink: d.sink(), Tag: jobTag(j.Command), Out: d.Stdout, Err: d.Stderr,
	})
	if err != nil {
		fmt.Fprintf(d.Stderr, "crond: cannot start %q: %v\n", j.Command, err)
		return
	}
	d.mu.Lock()
	d.pids[cmd.Process.Pid] = true
	d.mu.Unlock()
	code, dropped := sess.Wait()
	d.mu.Lock()
	delete(d.pids, cmd.Process.Pid)
	d.mu.Unlock()
	if dropped > 0 {
		fmt.Fprintf(d.Stderr, "crond: %d line(s) of %q not forwarded to syslog\n", dropped, j.Command)
	}
	fmt.Fprintf(d.Stdout, "crond: %s end %s (exit %d)\n", d.Now().Format("2006-01-02 15:04:05"), j.Command, code)
}

// mode and sink keep the supervisor settings in one place, read once.
func (d *daemon) mode() supervise.Mode {
	if d.Mode != 0 {
		return d.Mode
	}
	m, err := supervise.ModeFromEnv()
	if err != nil {
		fmt.Fprintf(d.Stderr, "crond: %v; using dup\n", err)
	}
	return m
}

func (d *daemon) sink() supervise.Sink {
	if d.Sink != nil {
		return d.Sink
	}
	// Always created: the connection is made on the first line, so a syslogd
	// that starts later is still reached.
	s := supervise.NewSyslogSink("/dev/log", "crond", 1024)
	_ = s.Start()
	return s
}

// jobTag is the tag a job's output carries: the program it runs.
func jobTag(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "job"
	}
	return filepath.Base(fields[0])
}

// stopJobs asks the running jobs to end and kills them when they do not.
func (d *daemon) stopJobs(grace time.Duration) {
	d.ensureMaps()
	d.mu.Lock()
	pids := make([]int, 0, len(d.pids))
	for pid := range d.pids {
		pids = append(pids, pid)
	}
	d.mu.Unlock()
	for _, pid := range pids {
		signalJob(pid, false)
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		n := len(d.pids)
		d.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	d.mu.Lock()
	left := make([]int, 0, len(d.pids))
	for pid := range d.pids {
		left = append(left, pid)
	}
	d.mu.Unlock()
	for _, pid := range left {
		signalJob(pid, true)
	}
}

// Run is the command. It reads the files, schedules, and waits for SIGTERM
// (stop) or SIGHUP (reload).
//
//	crond [-c FILE]... [-D DIR] [-n]
func Run(name string, args []string, stdout, stderr io.Writer) int {
	var files []string
	dir := "/etc/cron.d"
	explicit := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			fmt.Fprintf(stdout, "usage: %s [-c FILE]... [-D DIR] [-n]\n", name)
			fmt.Fprintln(stdout, "  -c FILE   crontab to read (repeatable; default /etc/crontab)")
			fmt.Fprintln(stdout, "  -D DIR    directory of crontabs (default /etc/cron.d)")
			fmt.Fprintln(stdout, "  -n        foreground; this program never daemonises, the flag is accepted for symmetry")
			fmt.Fprintln(stdout, "  5 fields, or 6 with a leading seconds field; @hourly/@daily/@weekly/@monthly/@yearly/@reboot")
			fmt.Fprintln(stdout, "  SIGHUP reloads the files, SIGTERM stops the jobs and exits")
			return 0
		case a == "-c":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "%s: -c needs a file\n", name)
				return 2
			}
			i++
			files = append(files, args[i])
			explicit = true
		case strings.HasPrefix(a, "-c"):
			files = append(files, strings.TrimPrefix(a, "-c"))
			explicit = true
		case a == "-D":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "%s: -D needs a directory\n", name)
				return 2
			}
			i++
			dir = args[i]
		case strings.HasPrefix(a, "-D"):
			dir = strings.TrimPrefix(a, "-D")
		case a == "-n" || a == "-f":
			// foreground is the only mode; accepted so a BusyBox call works
		default:
			fmt.Fprintf(stderr, "%s: unknown option %q\n", name, a)
			return 2
		}
	}
	if !explicit {
		files = []string{"/etc/crontab"}
	}
	d := &daemon{Files: files, Dir: dir, Stdout: stdout, Stderr: stderr, Now: time.Now}
	sched, jobs, err := d.start()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	fmt.Fprintf(stdout, "crond: %d job(s) from %s and %s\n", jobs, strings.Join(files, ","), dir)

	reload, stop := signalChannels()
	defer stopSignals()
	for {
		select {
		case <-reload:
			sched.Stop()
			fmt.Fprintf(stdout, "crond: reloading\n")
			sched, jobs, err = d.start()
			if err != nil {
				fmt.Fprintf(stderr, "%s: reload failed, keeping the old schedule: %v\n", name, err)
				continue
			}
			fmt.Fprintf(stdout, "crond: %d job(s) after reload\n", jobs)
		case <-stop:
			sched.Stop()
			d.stopJobs(2 * time.Second)
			fmt.Fprintf(stdout, "crond: stopping\n")
			return 0
		}
	}
}
