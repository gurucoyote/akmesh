package clear

import (
	"os"
	"path/filepath"
	"testing"
)

// run in a temp file and return what the command wrote, plus the exit code.
func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out")
	errPath := filepath.Join(dir, "err")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	errFile, err := os.Create(errPath)
	if err != nil {
		t.Fatal(err)
	}
	code := Run(args, os.Stdin, out, errFile)
	out.Close()
	errFile.Close()
	outData, _ := os.ReadFile(outPath)
	errData, _ := os.ReadFile(errPath)
	return string(outData), string(errData), code
}

func TestClearWritesScreenAndScrollback(t *testing.T) {
	out, _, code := run(t)
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	want := "\x1b[H\x1b[2J\x1b[3J\x1b[H"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestClearDashXKeepsScrollback(t *testing.T) {
	out, _, code := run(t, "-x")
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	want := "\x1b[H\x1b[2J\x1b[H"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestClearRejectsUnknownOption(t *testing.T) {
	out, errOut, code := run(t, "-q")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if out != "" {
		t.Fatalf("wrote %q although the option was refused", out)
	}
	if errOut == "" {
		t.Fatal("no message for an unknown option")
	}
}
