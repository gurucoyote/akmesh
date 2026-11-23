package supervise

import (
	"bytes"
	"net"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// collect is a Sink that records what it was given.
type collect struct {
	mu    sync.Mutex
	lines []string
	prios []int
}

func (c *collect) TrySend(tag string, priority int, line string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, line)
	c.prios = append(c.prios, priority)
	return true
}

func (c *collect) all() ([]string, []int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.lines...), append([]int{}, c.prios...)
}

func sh(script string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", script)
}

// TestDupPassesAndForwards is the whole ticket in one test: three lines on
// stderr, two on stdout, all five visible in the pass-through, all five in the
// sink, with info for stdout and err for stderr.
func TestDupPassesAndForwards(t *testing.T) {
	var out, errb bytes.Buffer
	sink := &collect{}
	cmd := sh("echo eins; echo zwei auf stderr >&2; echo drei; echo vier auf stderr >&2; echo fuenf auf stderr >&2")
	s, err := Start(cmd, Config{Mode: ModeDup, Tag: "kind", Sink: sink, Out: &out, Err: &errb})
	if err != nil {
		t.Fatal(err)
	}
	code, dropped := s.Wait()
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if dropped != 0 {
		t.Fatalf("%d lines dropped", dropped)
	}
	if got := strings.Count(out.String(), "\n"); got != 2 {
		t.Errorf("stdout lines = %d, want 2: %q", got, out.String())
	}
	if got := strings.Count(errb.String(), "\n"); got != 3 {
		t.Errorf("stderr lines = %d, want 3: %q", got, errb.String())
	}
	lines, prios := sink.all()
	if len(lines) != 5 {
		t.Fatalf("sink got %d lines, want 5: %v", len(lines), lines)
	}
	info, errs := 0, 0
	for _, p := range prios {
		switch p {
		case PriorityStdout:
			info++
		case PriorityStderr:
			errs++
		}
	}
	if info != 2 || errs != 3 {
		t.Fatalf("priorities info=%d err=%d, want 2 and 3 (%v)", info, errs, prios)
	}
}

// TestForwardOnlyKeepsTheStreamsQuiet is the mode for a program that must not see
// its own output repeated.
func TestForwardOnlyKeepsTheStreamsQuiet(t *testing.T) {
	var out, errb bytes.Buffer
	sink := &collect{}
	s, err := Start(sh("echo still; echo laut >&2"), Config{Mode: ModeForward, Sink: sink, Out: &out, Err: &errb})
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := s.Wait(); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out.Len() != 0 || errb.Len() != 0 {
		t.Fatalf("forward kept output: out=%q err=%q", out.String(), errb.String())
	}
	if lines, _ := sink.all(); len(lines) != 2 {
		t.Fatalf("sink got %d lines", len(lines))
	}
}

// TestPartialLastLineIsDelivered: the last line of a dying child often has no
// newline, and it is the interesting one.
func TestPartialLastLineIsDelivered(t *testing.T) {
	var out bytes.Buffer
	sink := &collect{}
	s, err := Start(sh(`printf 'ohne abschluss'`), Config{Mode: ModeDup, Sink: sink, Out: &out, Err: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if !strings.Contains(out.String(), "ohne abschluss") {
		t.Fatalf("pass-through lost the partial line: %q", out.String())
	}
	if lines, _ := sink.all(); len(lines) != 1 || lines[0] != "ohne abschluss" {
		t.Fatalf("sink = %v", lines)
	}
}

// TestLongLineIsCutAndMarked keeps one stack trace line from becoming a megabyte
// in the log.
func TestLongLineIsCutAndMarked(t *testing.T) {
	var out bytes.Buffer
	sink := &collect{}
	s, err := Start(sh(`printf '%5000s\n' x`), Config{Mode: ModeDup, Sink: sink, MaxLine: 100, Out: &out, Err: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	s.Wait()
	lines, _ := sink.all()
	if len(lines) != 1 {
		t.Fatalf("sink = %v", lines)
	}
	if len(lines[0]) > 100+len(" (gekuerzt)") || !strings.HasSuffix(lines[0], "(gekuerzt)") {
		t.Fatalf("line not cut and marked: %d bytes %q", len(lines[0]), lines[0][:min(len(lines[0]), 40)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestExitCodeIsPassedThrough: the supervisor must not invent a status.
func TestExitCodeIsPassedThrough(t *testing.T) {
	s, err := Start(sh("exit 7"), Config{Mode: ModeDup, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := s.Wait(); code != 7 {
		t.Fatalf("exit %d, want 7", code)
	}
}

// blockSink never takes a line, like a syslogd that stopped reading.
type blockSink struct{}

func (blockSink) TrySend(string, int, string) bool { return false }

// TestFullSinkDoesNotStallTheChild is the rule that matters: the pipe must never
// kill or freeze the child.
func TestFullSinkDoesNotStallTheChild(t *testing.T) {
	done := make(chan struct{})
	var code int
	var dropped int64
	go func() {
		s, err := Start(sh("i=0; while [ $i -lt 200 ]; do echo zeile $i; i=$((i+1)); done"),
			Config{Mode: ModeForward, Sink: blockSink{}})
		if err != nil {
			t.Error(err)
			close(done)
			return
		}
		code, dropped = s.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("child stalled with a sink that takes nothing")
	}
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if dropped != 200 {
		t.Fatalf("dropped %d lines, want 200", dropped)
	}
}

// TestModeFromEnv: a typo is an error, not a silent default.
func TestModeFromEnv(t *testing.T) {
	cases := map[string]Mode{"": ModeDup, "dup": ModeDup, "forward": ModeForward, "pass": ModePass, "PASS": ModePass}
	for v, want := range cases {
		t.Setenv(EnvName, v)
		got, err := ModeFromEnv()
		if err != nil {
			t.Errorf("%q: %v", v, err)
			continue
		}
		if got != want {
			t.Errorf("%q = %v, want %v", v, got, want)
		}
	}
	t.Setenv(EnvName, "quatsch")
	if _, err := ModeFromEnv(); err == nil {
		t.Fatal("unknown mode accepted")
	}
}

// TestDeadSyslogDoesNotStallTheChild measures the limit from the ticket: a
// socket nobody reads fills the queue, lines are dropped and counted, and the
// child still finishes.
func TestDeadSyslogDoesNotStallTheChild(t *testing.T) {
	dir := t.TempDir()
	sock := dir + "/log"
	addr, err := net.ResolveUnixAddr("unixgram", sock)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // never read from it

	sink := NewSyslogSink(sock, "kind", 4)
	if err := sink.Start(); err != nil {
		t.Fatal(err)
	}
	defer sink.Close()

	done := make(chan struct{})
	var code int
	var dropped int64
	go func() {
		s, err := Start(sh("i=0; while [ $i -lt 500 ]; do echo zeile $i mit etwas text; i=$((i+1)); done"),
			Config{Mode: ModeForward, Tag: "kind", Sink: sink})
		if err != nil {
			t.Error(err)
			close(done)
			return
		}
		code, dropped = s.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("child stalled because syslog stopped reading")
	}
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if dropped == 0 {
		t.Fatalf("nothing dropped, the limit was not reached (sent %d)", sink.Sent())
	}
	t.Logf("500 lines, %d dropped, %d sent", dropped, sink.Sent())
}
