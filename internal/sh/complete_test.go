package sh

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestEditor builds an editor whose out is a buffer, so the candidate list
// can be read back, with a fixed directory and PATH.
func newTestEditor(t *testing.T, dir, path string) (*editor, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	e := newEditor(os.Stdin, &buf)
	e.dir = func() string { return dir }
	e.getenv = func(name string) string {
		switch name {
		case "PATH":
			return path
		case "HOME":
			return dir
		}
		return ""
	}
	return e, &buf
}

func TestCompleteCommandFromPath(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"projects-tool", "prog", "other"} {
		if err := os.WriteFile(filepath.Join(bin, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	e, _ := newTestEditor(t, bin, bin)
	buf, pos := e.completeLine([]rune("pro"), 3, "p$ ")
	// Both names begin with "pro", so the shared part is filled in and nothing
	// else happens.
	if got := string(buf); got != "pro" {
		t.Fatalf("shared prefix: got %q, want %q", got, "pro")
	}
	if pos != 3 {
		t.Fatalf("pos: got %d, want 3", pos)
	}
}

func TestCompleteUniqueCommandAddsSpace(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gosh"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	e, _ := newTestEditor(t, bin, bin)
	buf, pos := e.completeLine([]rune("go"), 2, "p$ ")
	if got, want := string(buf), "gosh "; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if pos != len(buf) {
		t.Fatalf("pos: got %d, want %d", pos, len(buf))
	}
}

func TestCompleteDirectoriesGetSlash(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	e, _ := newTestEditor(t, dir, "")
	buf, pos := e.completeLine([]rune("cd proj"), 7, "p$ ")
	if got, want := string(buf), "cd projects/"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if pos != len(buf) {
		t.Fatalf("pos: got %d, want %d", pos, len(buf))
	}
}

func TestCompleteListsAmbiguousPaths(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"projects", "privat"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	e, out := newTestEditor(t, dir, "")
	buf, pos := e.completeLine([]rune("cd pr"), 5, "p$ ")
	if got := string(buf); got != "cd pr" {
		t.Fatalf("line must stay unchanged, got %q", got)
	}
	if pos != 5 {
		t.Fatalf("pos: got %d, want 5", pos)
	}
	printed := out.String()
	for _, want := range []string{"privat", "projects"} {
		if !strings.Contains(printed, want) {
			t.Fatalf("list %q is missing %q", printed, want)
		}
	}
}

func TestCompleteIgnoresHiddenFilesUnlessAsked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".profile"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e, _ := newTestEditor(t, dir, "")
	buf, pos := e.completeLine([]rune("cat "), 4, "p$ ")
	if got := string(buf); got != "cat " {
		t.Fatalf("hidden file completed without a dot: %q", got)
	}
	if pos != 4 {
		t.Fatalf("pos: got %d, want 4", pos)
	}
}
