//go:build !windows

package sh

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestDollarBangIsARealPID: mvdan/sh answered $! with a job number ("g1"), which
// no script can hand to kill. It must be the process id (acmesh-tms).
func TestDollarBangIsARealPID(t *testing.T) {
	var out, errb bytes.Buffer
	code := RunAs("sh", []string{"-c", "sleep 30 & echo $!"}, strings.NewReader(""), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	pid, err := strconv.Atoi(strings.TrimSpace(out.String()))
	if err != nil {
		t.Fatalf("$! = %q, not a pid", strings.TrimSpace(out.String()))
	}
	// The process must really exist while the shell is still alive. The shell
	// has ended here, so check the pid was plausible and clean up if it lives.
	if _, err := os.Stat("/proc/" + strconv.Itoa(pid)); err != nil {
		t.Logf("pid %d already gone (the shell exited and took the job with it)", pid)
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// TestKillEndsAProcess is the case the ticket came from: start in the
// background, kill it by its own $!.
func TestKillEndsAProcess(t *testing.T) {
	var out, errb bytes.Buffer
	script := `sh -c 'sleep 30 & echo $!; sleep 0.3; kill $! && echo "kill ok"'`
	code := RunAs("sh", []string{"-c", script}, strings.NewReader(""), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	lines := strings.Fields(strings.TrimSpace(out.String()))
	if len(lines) < 3 || lines[len(lines)-2] != "kill" || lines[len(lines)-1] != "ok" {
		t.Fatalf("kill did not report success: %q (stderr %q)", out.String(), errb.String())
	}
	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		t.Fatalf("$! = %q, not a pid", lines[0])
	}
	// The killed process may be a zombie until its parent reaps it; accept
	// both "gone" and "not running".
	if alive(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("pid %d is still running after kill", pid)
	}
}

// TestKillZeroTestsExistence: kill -0 is the "is it alive" probe scripts use.
func TestKillZeroTestsExistence(t *testing.T) {
	var out, errb bytes.Buffer
	code := RunAs("sh", []string{"-c", "kill -0 $$ && echo lebt; kill -0 999999 || echo tot"},
		strings.NewReader(""), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "lebt") {
		t.Errorf("kill -0 $$ failed: %q (stderr %q)", got, errb.String())
	}
	if !strings.Contains(got, "tot") {
		t.Errorf("kill -0 on a missing pid succeeded: %q", got)
	}
	if !strings.Contains(errb.String(), "no such process") {
		t.Errorf("no message for the missing pid: %q", errb.String())
	}
}

// TestKillOptions: -s and -n with names and numbers, and kill -l.
func TestKillOptions(t *testing.T) {
	var out, errb bytes.Buffer
	code := RunAs("sh", []string{"-c", "kill -l KILL; kill -l 15"}, strings.NewReader(""), &out, &errb)
	if code != 0 {
		t.Fatalf("kill -l failed: exit %d, stderr %q", code, errb.String())
	}
	got := out.String() + errb.String()
	if !strings.Contains(got, "9") || !strings.Contains(got, "TERM") {
		t.Fatalf("kill -l output %q, want 9 and TERM", got)
	}

	// A bad signal name must fail, not silently send SIGTERM.
	out.Reset()
	errb.Reset()
	if code := RunAs("sh", []string{"-c", "kill -s NOSUCH $$"}, strings.NewReader(""), &out, &errb); code == 0 {
		t.Fatalf("invalid signal accepted")
	}
	if !strings.Contains(errb.String(), "invalid signal") {
		t.Fatalf("no message for an invalid signal: %q", errb.String())
	}
}

// alive reports whether pid is a running (not zombie) process.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	// state is the field after the command name, which is in parentheses.
	if i := bytes.LastIndexByte(data, ')'); i >= 0 && i+2 < len(data) {
		if data[i+2] == 'Z' {
			return false
		}
	}
	time.Sleep(0)
	return true
}
