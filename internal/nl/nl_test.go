package nl

import (
	"os"
	"path/filepath"
	"testing"
)

func runHere(t *testing.T, args []string, input string) (string, int) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in")
	if err := os.WriteFile(in, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	stdin, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
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
	code := Run(args, stdin, out, errFile)
	data, _ := os.ReadFile(out.Name())
	return string(data), code
}

// TestNumbersOnlyNonEmptyLines: blank lines get the padding but no number, which
// is what GNU nl does (checked with od against the container: six spaces, no
// digits, then the newline).
func TestNumbersOnlyNonEmptyLines(t *testing.T) {
	got, code := runHere(t, nil, "eins\n\nzwei\n")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got != "     1\teins\n      \n     2\tzwei\n" {
		t.Errorf("got %q", got)
	}
}
