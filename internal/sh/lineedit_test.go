package sh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHistoryRoundTrip covers what the file has to do: keep the lines in order,
// skip an immediate repetition, and never grow past the limit.
func TestHistoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "history")
	t.Setenv("AKMESH_HISTFILE", name)

	appendHistory("echo eins")
	appendHistory("echo eins") // immediate repetition: not stored twice
	appendHistory("   ")       // blank: not stored at all
	appendHistory("echo zwei")

	got := loadHistory()
	want := []string{"echo eins", "echo zwei"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("history is %q, want %q", got, want)
	}

	// A file with more lines than the limit keeps the newest ones.
	var many []string
	for i := 0; i < historyLimit+50; i++ {
		many = append(many, "cmd"+string(rune('a'+i%26)))
	}
	if err := os.WriteFile(name, []byte(strings.Join(many, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n := len(loadHistory()); n != historyLimit {
		t.Fatalf("loaded %d lines, want the limit %d", n, historyLimit)
	}
}

// TestHistoryWithoutFileIsQuiet: no HOME and no AKMESH_HISTFILE means history
// stays in memory; nothing may fail or write anywhere.
func TestHistoryWithoutFileIsQuiet(t *testing.T) {
	t.Setenv("AKMESH_HISTFILE", "")
	t.Setenv("HOME", "")
	appendHistory("echo eins")
	if got := loadHistory(); len(got) != 0 {
		t.Fatalf("history is %q, want none", got)
	}
}

// TestNoEditorOnAPipe: a pipe is not a terminal, so the plain reader stays in
// charge and the non-interactive behaviour does not change.
func TestNoEditorOnAPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if _, ok := useEditor(r); ok {
		t.Fatal("a pipe must not get the line editor")
	}
	if _, ok := useEditor(strings.NewReader("echo hi\n")); ok {
		t.Fatal("a strings.Reader must not get the line editor")
	}
}
