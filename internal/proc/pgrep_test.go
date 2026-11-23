package proc

import (
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestPgrepMatching pins the differences between the flags, because they are
// the whole point: a substring that matches too much is worse than no answer.
func TestPgrepMatching(t *testing.T) {
	procs := []procInfo{
		{PID: 10, PPID: 1, User: "root", Comm: "sleep", CmdLine: "sleep 30", Started: time.Unix(100, 0)},
		{PID: 11, PPID: 1, User: "martin", Comm: "mysleep", CmdLine: "mysleep 30", Started: time.Unix(200, 0)},
		{PID: 12, PPID: 10, User: "root", Comm: "python3", CmdLine: "python3 worker.py --queue", Started: time.Unix(300, 0)},
		{PID: 13, PPID: 1, User: "Administrator", Comm: "akmesh.exe", CmdLine: "akmesh.exe ps", Started: time.Unix(400, 0)},
	}
	cases := []struct {
		name string
		o    pgrepOpts
		want []int
	}{
		{"name is a substring by default", pgrepOpts{pattern: "sleep"}, []int{10, 11}},
		{"-x demands the whole name", pgrepOpts{pattern: "sleep", exact: true}, []int{10}},
		{"-f matches the command line", pgrepOpts{pattern: "worker.py", full: true}, []int{12}},
		{"-f still honours -x", pgrepOpts{pattern: "python3 worker.py --queue", full: true, exact: true}, []int{12}},
		{"-u filters by user", pgrepOpts{pattern: "*", user: "martin", full: true}, []int{11}},
		{"-P filters by parent", pgrepOpts{pattern: "*", ppid: 10, full: true}, []int{12}},
		{"-n takes the youngest match", pgrepOpts{pattern: "sleep", newest: true}, []int{11}},
		{"-o takes the oldest match", pgrepOpts{pattern: "sleep", oldest: true}, []int{10}},
		// Windows names the image with .exe; an agent types the plain name.
		{"-x sees through .exe", pgrepOpts{pattern: "akmesh", exact: true}, []int{13}},
	}
	for _, c := range cases {
		opts := c.o
		if opts.pattern == "*" {
			// A lone star as an expression matches anything.
			opts.pattern = ".*"
		}
		got := pgrepSelect(procs, opts)
		var pids []int
		for _, p := range got {
			pids = append(pids, p.PID)
		}
		if len(pids) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, pids, c.want)
			continue
		}
		for i := range pids {
			if pids[i] != c.want[i] {
				t.Errorf("%s: got %v, want %v", c.name, pids, c.want)
				break
			}
		}
	}
}

// TestPgrepArgumentErrors: a missing pattern is a usage error, not an empty
// process list that looks like "nothing running".
func TestPgrepArgumentErrors(t *testing.T) {
	if _, _, err := parsePgrepArgs(nil, false); err == nil {
		t.Fatal("no pattern and no filter accepted")
	}
	// With a filter the pattern may be left out: "pgrep -P 1" is a list request.
	if _, _, err := parsePgrepArgs([]string{"-P", "1"}, false); err != nil {
		t.Fatalf("-P without a pattern refused: %v", err)
	}
	if _, _, err := parsePgrepArgs([]string{"-u"}, false); err == nil {
		t.Fatal("-u without a user accepted")
	}
	if _, _, err := parsePgrepArgs([]string{"-P", "keine-zahl"}, false); err == nil {
		t.Fatal("-P with a non-number accepted")
	}
	if _, _, err := parsePgrepArgs([]string{"-Z", "x"}, false); err == nil {
		t.Fatal("unknown option accepted")
	}
	// pkill reads -TERM and -9, pgrep must refuse them: it has no signal.
	if _, _, err := parsePgrepArgs([]string{"-TERM", "x"}, false); err == nil {
		t.Fatal("pgrep accepted a signal")
	}
	if _, sig, err := parsePgrepArgs([]string{"-TERM", "x"}, true); err != nil || sig != "TERM" {
		t.Fatalf("pkill did not read -TERM: sig=%q err=%v", sig, err)
	}
}

// TestResolveUser: "pgrep -u 0" has to mean the same as "pgrep -u root".
func TestResolveUser(t *testing.T) {
	if got := resolveUser("0"); got != "root" {
		t.Fatalf("uid 0 resolved to %q, want root", got)
	}
	if got := resolveUser("root"); got != "root" {
		t.Fatalf("name changed: %q", got)
	}
	if got := resolveUser("4242424"); got != "4242424" {
		t.Fatalf("unknown uid %q was changed to %q", "4242424", got)
	}
}

// helperProcess is the child the integration tests look for. It does nothing
// but sleep with a marker on its command line.
func helperProcess(t *testing.T) {
	if os.Getenv("AKMESH_PGREP_HELPER") != "1" {
		return
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func startHelper(t *testing.T) *exec.Cmd {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("own executable: %v", err)
	}
	marker := "akmesh-pgrep-marker-" + strconv.Itoa(os.Getpid())
	cmd := exec.Command(self, "-test.run=TestPgrepHelperProcess", marker)
	cmd.Env = append(os.Environ(), "AKMESH_PGREP_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	// The process has to be visible in /proc before the search can find it.
	for i := 0; i < 100; i++ {
		if p, err := os.FindProcess(cmd.Process.Pid); err == nil && p != nil {
			if _, err := os.ReadFile("/proc/" + strconv.Itoa(cmd.Process.Pid) + "/cmdline"); err == nil {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cmd
}

func TestPgrepHelperProcess(t *testing.T) { helperProcess(t) }

// TestPgrepFindsARealProcess: the measured case, "pgrep -f <thing>".
func TestPgrepFindsARealProcess(t *testing.T) {
	cmd := startHelper(t)
	out, err := captureRun(t, runPgrep, []string{"-f", "akmesh-pgrep-marker-" + strconv.Itoa(os.Getpid())})
	if err != nil {
		t.Fatalf("pgrep: %v", err)
	}
	got := strings.TrimSpace(out)
	if got != strconv.Itoa(cmd.Process.Pid) {
		t.Fatalf("pgrep -f gave %q, want %d", got, cmd.Process.Pid)
	}
	// A pattern that matches nothing is exit 1, the "not running" answer.
	if code := runPgrepQuiet([]string{"-f", "akmesh-nichts-da-4711"}); code != 1 {
		t.Fatalf("no match gave exit %d, want 1", code)
	}
}

// TestPkillEndsARealProcess: pkill must reach the same process pgrep finds.
func TestPkillEndsARealProcess(t *testing.T) {
	cmd := startHelper(t)
	pid := cmd.Process.Pid
	marker := "akmesh-pgrep-marker-" + strconv.Itoa(os.Getpid())
	code := runPkillQuiet([]string{"-f", marker})
	if code != 0 {
		t.Fatalf("pkill exit %d, want 0", code)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("pid %d still running after pkill", pid)
	}
	if code := runPgrepQuiet([]string{"-f", marker}); code != 1 {
		t.Fatalf("pgrep still finds the helper after pkill (exit %d)", code)
	}
}

// TestPkillNeverKillsItself: "pkill -f akmesh" from inside akmesh must not take
// the calling process with it. Checked on a list, not by really signalling
// everything that matches - the first version of this test did that and killed
// the test runner's whole process group.
func TestPkillNeverKillsItself(t *testing.T) {
	self := procInfo{PID: 4711, Comm: "akmesh", CmdLine: "akmesh sh -c pkill -f akmesh"}
	other := procInfo{PID: 4712, Comm: "akmesh", CmdLine: "akmesh ps"}
	got := pkillTargets([]procInfo{self, other}, pgrepOpts{pattern: "akmesh", full: true}, self.PID)
	if len(got) != 1 || got[0].PID != other.PID {
		t.Fatalf("targets %v, want only %d", got, other.PID)
	}
	// Nothing but itself matching means "no process found", exit 1.
	got = pkillTargets([]procInfo{self}, pgrepOpts{pattern: "akmesh", full: true}, self.PID)
	if len(got) != 0 {
		t.Fatalf("targets %v, want none", got)
	}
}

// captureRun runs one command and returns its standard output.
func captureRun(t *testing.T, fn func([]string, *os.File, *os.File, *os.File) int, args []string) (string, error) {
	t.Helper()
	outF, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer outF.Close()
	errF, err := os.CreateTemp(t.TempDir(), "err")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer errF.Close()
	if code := fn(args, nil, outF, errF); code != 0 {
		data, _ := os.ReadFile(errF.Name())
		return "", &exitError{code: code, msg: string(data)}
	}
	data, err := os.ReadFile(outF.Name())
	return string(data), err
}

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string {
	return "exit " + strconv.Itoa(e.code) + ": " + strings.TrimSpace(e.msg)
}

// runPgrepQuiet returns only the exit code, with output discarded.
func runPgrepQuiet(args []string) int {
	return runPgrepQuietWith(args, runPgrep)
}

func runPkillQuiet(args []string) int {
	return runPgrepQuietWith(args, runPkill)
}

func runPgrepQuietWith(args []string, fn func([]string, *os.File, *os.File, *os.File) int) int {
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return 2
	}
	defer devnull.Close()
	return fn(args, nil, devnull, devnull)
}

var _ = io.Discard
