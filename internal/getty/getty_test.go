//go:build !windows

package getty

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExpandIssue(t *testing.T) {
	at := time.Date(2026, 11, 23, 13, 45, 0, 0, time.UTC)
	got := expandIssue("Willkommen auf \\n\nTerminal \\l, \\t am \\d\n", "kiste", "tty1", at)
	want := "Willkommen auf kiste\nTerminal tty1, 13:45:00 am Nov 23 2026\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// An unknown escape stays visible instead of disappearing.
	if got := expandIssue(`\q`, "h", "t", at); got != `\q` {
		t.Fatalf("unknown escape = %q", got)
	}
	if got := expandIssue(`\\`, "h", "t", at); got != `\` {
		t.Fatalf("backslash = %q", got)
	}
}

// TestGettyAsksAndStartsSessions: one session per run (init respawns getty), an
// empty line is asked again, and the name reaches login.
func TestGettyAsksAndStartsSessions(t *testing.T) {
	dir := t.TempDir()
	issue := dir + "/issue"
	if err := os.WriteFile(issue, []byte("Willkommen auf \\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldIssue, oldHost, oldLogin := issueFile, hostname, runLogin
	defer func() { issueFile, hostname, runLogin = oldIssue, oldHost, oldLogin }()
	issueFile = issue
	hostname = func() (string, error) { return "kiste", nil }

	var names []string
	runLogin = func(name string, in io.Reader, out, err io.Writer) int {
		names = append(names, name)
		return 0
	}
	var out, errb bytes.Buffer
	code := Run(nil, strings.NewReader("\ntester\n"), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if len(names) != 1 || names[0] != "tester" {
		t.Fatalf("logins = %v, want [tester]", names)
	}
	text := out.String()
	if !strings.Contains(text, "Willkommen auf kiste") {
		t.Errorf("issue not printed: %q", text)
	}
	if strings.Count(text, "kiste login: ") != 2 {
		t.Errorf("prompt count = %d, want 2 (one repeated for the empty line): %q",
			strings.Count(text, "kiste login: "), text)
	}
	if !strings.Contains(text, "logout (exit 0)") {
		t.Errorf("no logout line: %q", text)
	}
}

// TestGettyEndsWhenTheTerminalCloses: EOF is the only reason to return, and then
// with 0 so init does not see a failed service.
func TestGettyEndsWhenTheTerminalCloses(t *testing.T) {
	oldIssue, oldHost, oldLogin := issueFile, hostname, runLogin
	defer func() { issueFile, hostname, runLogin = oldIssue, oldHost, oldLogin }()
	issueFile = "/nonexistent/issue" // a missing issue is not an error
	hostname = func() (string, error) { return "kiste", nil }
	runLogin = func(string, io.Reader, io.Writer, io.Writer) int { return 0 }
	var out, errb bytes.Buffer
	if code := Run(nil, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, errb.String())
	}
}

// TestMissingTTYIsAnError: a tty that cannot be opened must fail, not fall back to
// stdin - a getty on the wrong terminal is worse than none.
func TestMissingTTYIsAnError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"-t", "/dev/definitely-not-here"}, strings.NewReader(""), &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "getty:") {
		t.Fatalf("no message: %q", errb.String())
	}
}
