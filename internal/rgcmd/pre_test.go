package rgcmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gurucoyote/akmesh/internal/docxgen"
)

// docxBytes builds a real docx with our own writer, so the fixture and the
// claim sit in the same test and no binary is checked in.
func docxBytes(t *testing.T, markdown string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := docxgen.Convert(&buf, []byte(markdown)); err != nil {
		t.Fatalf("docxgen: %v", err)
	}
	return buf.Bytes()
}

// preFixture is a directory with one docx and one text file that talk about
// the same word. The broken PDF lives in its own test, so the success cases
// are not turned into exit 2 by it.
func preFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "vertrag.docx"), docxBytes(t, "# Vertrag\n\nDer Vertrag endet am 31.12.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("Der Vertrag als Text.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestPreFindsTextInsideDocx is the acceptance case: a sentence that exists
// only inside the docx is found through --pre, and the hit names the docx.
func TestPreFindsTextInsideDocx(t *testing.T) {
	dir := preFixture(t)
	out, stderr, code := runOurs(t, dir, []string{"--pre", "any2md", "endet am 31.12", "."})
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(out, "vertrag.docx:") {
		t.Fatalf("hit does not name the docx:\n%s", out)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

// TestWithoutPreDocxIsInvisible proves the preprocessor is the cause of the
// hit above, not a coincidence of the raw bytes.
func TestWithoutPreDocxIsInvisible(t *testing.T) {
	dir := preFixture(t)
	out, _, code := runOurs(t, dir, []string{"endet am 31.12", "."})
	if code != 1 || out != "" {
		t.Fatalf("without --pre: exit %d output %q, want 1 and no output", code, out)
	}
}

// TestPreBrokenDocumentIsAnError: a document that cannot be converted is
// reported and the run ends with 2, never with a silent "no match".
func TestPreBrokenDocumentIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kaputt.pdf"), []byte("%PDF-1.4\nkaputt ohne xref\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runOurs(t, dir, []string{"--pre", "any2md", "nichts", "."})
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr, "kaputt.pdf") {
		t.Fatalf("stderr does not name the broken document: %q", stderr)
	}
}

// TestPreGlobSelectsFiles: with --pre-glob '*.txt' the text file goes through
// the converter (and fails, because it is no document), while the docx stays
// raw (and is therefore not found). One run proves both directions.
func TestPreGlobSelectsFiles(t *testing.T) {
	dir := preFixture(t)
	out, stderr, code := runOurs(t, dir, []string{"--pre", "any2md", "--pre-glob", "*.txt", "Vertrag", "."})
	if code != 2 {
		t.Fatalf("exit %d, want 2 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stderr, "note.txt") || !strings.Contains(stderr, "unknown document format") {
		t.Fatalf("stderr does not show note.txt going through the converter: %q", stderr)
	}
	if strings.Contains(out, "vertrag.docx") {
		t.Fatalf("docx was searched through the preprocessor although --pre-glob said *.txt:\n%s", out)
	}
}

// TestPreExternalCommand covers the other form: a command with the path as its
// last argument, its output searched as the document.
func TestPreExternalCommand(t *testing.T) {
	cat, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("no cat on PATH: the external preprocessor form did not run")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "roh.dat"), []byte("nadel im heuhaufen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runOurs(t, dir, []string{"--pre", cat, "--pre-glob", "*.dat", "nadel", "."})
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(out, "roh.dat:nadel im heuhaufen") {
		t.Fatalf("external preprocessor output was not searched:\n%s", out)
	}
}
