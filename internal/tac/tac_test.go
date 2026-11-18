package tac

import (
	"os"
	"path/filepath"
	"testing"
)

func runHere(t *testing.T, input string) (string, int) {
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
	code := Run(nil, stdin, out, errFile)
	data, _ := os.ReadFile(out.Name())
	return string(data), code
}

// TestReversesLineOrder checks the single thing tac promises.
func TestReversesLineOrder(t *testing.T) {
	got, code := runHere(t, "eins\nzwei\ndrei\n")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got != "drei\nzwei\neins\n" {
		t.Errorf("got %q", got)
	}
}
