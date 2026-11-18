package corecmd

import (
	"os"
	"path/filepath"
	"testing"
)

func runHere(t *testing.T, cmd string, args []string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errFile, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errFile.Close()
	code := Run(cmd, args, nil, out, errFile)
	o, _ := os.ReadFile(out.Name())
	e, _ := os.ReadFile(errFile.Name())
	return string(o), string(e), code
}

// TestEchoJoinsAndKeepsNewline is the plain case scripts rely on.
func TestEchoJoinsAndKeepsNewline(t *testing.T) {
	got, _, code := runHere(t, "echo", []string{"hallo", "welt"})
	if code != 0 || got != "hallo welt\n" {
		t.Fatalf("got %q, exit %d", got, code)
	}
}

// TestEchoNoNewline pins -n: without it a loop's output runs together.
func TestEchoNoNewline(t *testing.T) {
	got, _, code := runHere(t, "echo", []string{"-n", "x"})
	if code != 0 || got != "x" {
		t.Fatalf("got %q, exit %d", got, code)
	}
}

// TestEchoEscapes pins -e, which is how a harness writes a tab or a newline.
func TestEchoEscapes(t *testing.T) {
	got, _, code := runHere(t, "echo", []string{"-e", `a\tb\n`})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got != "a\tb\n\n" {
		t.Errorf("got %q", got)
	}
}

// TestPwdAndTruth checks the three commands that only report state.
func TestPwdAndTruth(t *testing.T) {
	got, _, code := runHere(t, "pwd", nil)
	if code != 0 || len(got) < 2 || got[len(got)-1] != '\n' {
		t.Fatalf("pwd: got %q, exit %d", got, code)
	}
	if _, _, code := runHere(t, "true", nil); code != 0 {
		t.Errorf("true: exit %d", code)
	}
	if _, _, code := runHere(t, "false", nil); code != 1 {
		t.Errorf("false: exit %d, want 1", code)
	}
}
