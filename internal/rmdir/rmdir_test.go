package rmdir

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run("rmdir", args, &out, &errb)
	return code, out.String(), errb.String()
}

// TestEmptyAndNonEmpty: GNU wording, exit 1, and the ignore switch.
func TestEmptyAndNonEmpty(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	full := filepath.Join(dir, "full")
	os.Mkdir(empty, 0o755)
	os.Mkdir(full, 0o755)
	os.WriteFile(filepath.Join(full, "x"), nil, 0o644)

	if code, _, errText := run(t, empty); code != 0 || errText != "" {
		t.Fatalf("empty dir: %d %q", code, errText)
	}
	if _, err := os.Stat(empty); !os.IsNotExist(err) {
		t.Fatal("empty dir survived")
	}
	code, _, errText := run(t, full)
	if code != 1 || !strings.Contains(errText, "rmdir: failed to remove '"+full+"': Directory not empty") {
		t.Fatalf("non-empty: %d %q", code, errText)
	}
	if code, _, errText := run(t, "--ignore-fail-on-non-empty", full); code != 0 || errText != "" {
		t.Fatalf("ignore switch: %d %q", code, errText)
	}
}

// TestParents: -p removes the parents that become empty and names them
// "directory" in an error, GNU's second wording. The walk stops at the first
// parent with content, which is also what keeps the test inside its tree:
// measured, GNU keeps walking up to "/" and reports that as busy.
func TestParents(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c")
	os.MkdirAll(deep, 0o755)
	// Something in dir keeps the walk from leaving the test's own tree.
	os.WriteFile(filepath.Join(dir, "keep"), nil, 0o644)
	code, _, errText := run(t, "-p", deep)
	if code != 1 || !strings.Contains(errText, "rmdir: failed to remove directory '"+dir+"': Directory not empty") {
		t.Fatalf("walk: %d %q", code, errText)
	}
	for _, gone := range []string{"c", "b", "a"} {
		if _, err := os.Stat(filepath.Join(dir, "a")); !os.IsNotExist(err) && gone == "a" {
			t.Fatal("parent a survived")
		}
	}

	// A parent that still has something else is reported, and the walk stops.
	deep = filepath.Join(dir, "p", "q")
	os.MkdirAll(deep, 0o755)
	os.WriteFile(filepath.Join(dir, "p", "other"), nil, 0o644)
	code, _, errText = run(t, "-p", deep)
	if code != 1 || !strings.Contains(errText, "rmdir: failed to remove directory '"+filepath.Join(dir, "p")+"': Directory not empty") {
		t.Fatalf("non-empty parent: %d %q", code, errText)
	}
}

// TestMessages: missing operand, missing directory, verbose.
func TestMessages(t *testing.T) {
	if code, _, errText := run(t); code != 1 || !strings.Contains(errText, "rmdir: missing operand\nTry 'rmdir --help' for more information.\n") {
		t.Fatalf("missing operand: %d %q", code, errText)
	}
	dir := t.TempDir()
	missing := filepath.Join(dir, "nix")
	if code, _, errText := run(t, missing); code != 1 || !strings.Contains(errText, "rmdir: failed to remove '"+missing+"': No such file or directory") {
		t.Fatalf("missing dir: %d %q", code, errText)
	}
	gone := filepath.Join(dir, "g")
	os.Mkdir(gone, 0o755)
	code, out, _ := run(t, "-v", gone)
	if code != 0 || !strings.Contains(out, "rmdir: removing directory, '"+gone+"'") {
		t.Fatalf("verbose: %d %q", code, out)
	}
	if code, _, errText := run(t, "--badopt"); code != 1 || !strings.Contains(errText, "unrecognized option '--badopt'") {
		t.Fatalf("bad option: %d %q", code, errText)
	}
}
