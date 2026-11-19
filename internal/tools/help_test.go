package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"akmesh/internal/usage"
)

// runThroughDispatcher calls one command the way the binary does, so that a
// --help answer produced by the dispatcher is part of the test and not only
// the text in the table (acmesh-py8).
func runThroughDispatcher(t *testing.T, name string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer errf.Close()
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()

	code = RunArgs(name, args, stdin, out, errf)
	return readBack(t, out), readBack(t, errf), code
}

func readBack(t *testing.T, f *os.File) string {
	t.Helper()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Clean(f.Name()))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestEveryUsageTextBelongsToACommand catches the typo that makes a text
// unreachable: a name in the table that no command dispatches means --help
// still fails for the command the author meant.
func TestEveryUsageTextBelongsToACommand(t *testing.T) {
	for name := range usage.Texts {
		if _, ok := Commands[name]; !ok {
			t.Errorf("usage text for %q, but no command of that name exists", name)
		}
	}
}

// TestEveryCommandWithTextAnswersHelp is the table test: every command that
// has a text answers --help with the usage line and with the exit status the
// text pins. Before the dispatcher handled this, 47 commands answered with an
// error instead (acmesh-py8).
func TestEveryCommandWithTextAnswersHelp(t *testing.T) {
	for name := range usage.Texts {
		stdout, stderr, code := runThroughDispatcher(t, name, "--help")
		want := usage.ExitCode(name)
		if code != want {
			t.Errorf("%s --help: exit %d, want %d (stderr %q)", name, code, want, stderr)
			continue
		}
		if !strings.HasPrefix(stdout, "Usage: "+name) {
			t.Errorf("%s --help: stdout does not start with %q, got %q", name, "Usage: "+name, firstLine(stdout))
		}
		if stderr != "" {
			t.Errorf("%s --help wrote to stderr: %q", name, stderr)
		}
	}
}

// TestHelpIsAnsweredBeforeTheParser covers the five commands that used to
// execute --help instead of answering it (acmesh-vr1). Each line names what
// the parser did with the word before this fix.
func TestHelpIsAnsweredBeforeTheParser(t *testing.T) {
	cases := []struct {
		name string
		args []string
		was  string // the wrong answer before the fix
	}{
		{"hostname", []string{"--help"}, "hostname: setting the name is not supported"},
		{"sync", []string{"--help"}, "sync: open --help: no such file or directory"},
		{"touch", []string{"--help"}, "touch: unknown option \"--help\""},
		{"base64", []string{"--help"}, "base64: unknown option \"--\""},
		{"wc", []string{"--help"}, "wc: unknown option \"--\""},
	}
	for _, c := range cases {
		stdout, stderr, code := runThroughDispatcher(t, c.name, c.args...)
		if code != 0 {
			t.Errorf("%s %v: exit %d, want 0 (stderr %q)", c.name, c.args, code, stderr)
		}
		if !strings.HasPrefix(stdout, "Usage: "+c.name) {
			t.Errorf("%s %v: stdout %q, want a usage line (before the fix: %q)",
				c.name, c.args, firstLine(stdout), c.was)
		}
	}
}

// TestDashDashStillEndsTheOptions: the fix for the long-option message must not
// turn "--" itself into an error. GNU base64 reads the rest as file names.
func TestDashDashStillEndsTheOptions(t *testing.T) {
	stdout, stderr, code := runThroughDispatcher(t, "base64", "--")
	if code != 0 {
		t.Fatalf("base64 --: exit %d, want 0 (stderr %q)", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("base64 -- with empty stdin printed %q, want nothing", stdout)
	}
}

// TestUnknownLongOptionNamesTheWholeToken: the message must name the token the
// caller wrote. Reporting only the first two letters made it read "unknown
// option \"--\"" (acmesh-vr1).
func TestUnknownLongOptionNamesTheWholeToken(t *testing.T) {
	for _, name := range []string{"base64", "wc"} {
		_, stderr, code := runThroughDispatcher(t, name, "--bogus")
		if code == 0 {
			t.Errorf("%s --bogus: exit 0, want a failure", name)
		}
		if !strings.Contains(stderr, `"--bogus"`) {
			t.Errorf("%s --bogus: stderr %q does not name the whole token", name, stderr)
		}
	}
}

// TestDashHIsHelpOnlyWhereTheOriginalSaysSo pins the measured rule: GNU
// hostname answers -h with usage, everything else keeps -h for its own meaning
// (acmesh-py8, comment of 19.11.2026 plus the container probe).
func TestDashHIsHelpOnlyWhereTheOriginalSaysSo(t *testing.T) {
	stdout, _, code := runThroughDispatcher(t, "hostname", "-h")
	if code != 0 || !strings.HasPrefix(stdout, "Usage: hostname") {
		t.Errorf("hostname -h: exit %d, stdout %q; want usage and 0", code, firstLine(stdout))
	}

	// touch -h is a missing operand in GNU, and it stays that way here.
	_, stderr, code := runThroughDispatcher(t, "touch", "-h")
	if code == 0 || !strings.Contains(stderr, "missing file operand") {
		t.Errorf("touch -h: exit %d, stderr %q; want the missing-operand error", code, stderr)
	}

	// basename and echo print the word itself, which is what GNU does with -h.
	stdout, _, _ = runThroughDispatcher(t, "basename", "-h")
	if stdout != "-h\n" {
		t.Errorf("basename -h: stdout %q, want %q", stdout, "-h\n")
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
