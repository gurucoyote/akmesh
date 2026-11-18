package wc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runWc(t *testing.T, stdin string, args ...string) (string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), code
}

// TestCharsSwitch: -m counts characters, GNU's spelling. Upstream only knew -r.
func TestCharsSwitch(t *testing.T) {
	out, code := runWc(t, "abc\n")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.TrimSpace(out) != "1 1 4" {
		t.Fatalf("default = %q, want line, word and byte counts", out)
	}
	out, _ = runWc(t, "héllo\n", "-m")
	if strings.TrimSpace(out) != "6" {
		t.Fatalf("-m = %q, want 6 characters", out)
	}
	out, _ = runWc(t, "héllo\n", "-c")
	if strings.TrimSpace(out) != "7" {
		t.Fatalf("-c = %q, want 7 bytes", out)
	}
}

// TestCombinedFlags: -lw is read as -l -w.
func TestCombinedFlags(t *testing.T) {
	out, _ := runWc(t, "a b\nc\n", "-lw")
	if strings.TrimSpace(out) != "2 3" {
		t.Fatalf("-lw = %q", out)
	}
}

// TestFiles0From: the names come NUL-separated from a file, as find -print0
// writes them.
func TestFiles0From(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, "one.txt")
	two := filepath.Join(dir, "two.txt")
	list := filepath.Join(dir, "list")
	if err := os.WriteFile(one, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte("b\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(list, []byte(one+"\x00"+two+"\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := runWc(t, "", "--files0-from="+list)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "total") || !strings.Contains(out, "3 3 6 total") {
		t.Fatalf("--files0-from = %q", out)
	}
}
