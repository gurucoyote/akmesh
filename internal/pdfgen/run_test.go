package pdfgen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConvertWritesPDF holds the one thing a PDF reader can check without
// trusting the renderer: the result is a PDF, with a header, a trailer and a
// page object.
func TestConvertWritesPDF(t *testing.T) {
	var buf bytes.Buffer
	if err := Convert(&buf, []byte("# Titel\n\nEin Absatz.\n")); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	out := buf.Bytes()
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("no PDF header, got %q", out[:min(len(out), 12)])
	}
	if !bytes.Contains(out, []byte("%%EOF")) {
		t.Fatalf("no PDF trailer")
	}
	if !bytes.Contains(out, []byte("/Type /Page")) && !bytes.Contains(out, []byte("/Type/Page")) {
		t.Fatalf("no page object in the PDF")
	}
}

// TestConvertKeepsUnicode tracks the reason we forked at all: a font that cannot
// carry Umlaute would silently produce a PDF with empty glyphs.
func TestConvertKeepsUnicode(t *testing.T) {
	var buf bytes.Buffer
	if err := Convert(&buf, []byte("Grüße, äöüß.\n")); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	// A PDF written with a subset font still names the glyphs it used; a run
	// without any embedded font data is a sign that the font failed.
	if !bytes.Contains(buf.Bytes(), []byte("FontFile")) {
		t.Fatalf("no embedded font in the PDF, Umlaute would be missing")
	}
}

// TestConvertEmptyDocument says what an empty input does: it is not an error, an
// agent that pipes nothing gets a valid, empty PDF rather than a crash.
func TestConvertEmptyDocument(t *testing.T) {
	var buf bytes.Buffer
	if err := Convert(&buf, nil); err != nil {
		t.Fatalf("Convert of an empty document: %v", err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatalf("empty document did not produce a PDF")
	}
}

// TestRunWritesFileAndKeepsStdoutClean is the contract an agent relies on: with
// -o nothing goes to standard output, so a following command in a pipe is not
// fed binary garbage.
func TestRunWritesFileAndKeepsStdoutClean(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.md")
	if err := os.WriteFile(src, []byte("# Titel\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out.pdf")

	var stdout, stderr bytes.Buffer
	code := runIn(t, []string{"-o", dst, src}, "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout is not empty with -o: %q", stdout.String())
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("output file: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("output file is not a PDF")
	}
}

// TestRunReadsStdin covers the pipe case.
func TestRunReadsStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runIn(t, nil, "# Aus der Pipe\n", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if !bytes.HasPrefix(stdout.Bytes(), []byte("%PDF-")) {
		t.Fatalf("stdin was not turned into a PDF")
	}
}

// TestRunRefusesBadInput keeps the exit codes apart: a missing file or a wrong
// option is not success, and an unknown option is a usage error (2), not a
// runtime error (1).
func TestRunRefusesBadInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runIn(t, []string{"--frobnicate"}, "", &stdout, &stderr); code != 2 {
		t.Fatalf("unknown option: exit %d, want 2", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runIn(t, []string{"/nonexistent/in.md"}, "", &stdout, &stderr); code != 1 {
		t.Fatalf("missing file: exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "md2pdf:") {
		t.Fatalf("missing file produced no message: %q", stderr.String())
	}
}

// runIn calls Run with a temporary stdin, so the test does not depend on the
// real one.
func runIn(t *testing.T, args []string, stdin string, stdout, stderr *bytes.Buffer) int {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(stdin); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	so, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer so.Close()
	se, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer se.Close()
	code := Run(args, f, so, se)
	if stdout != nil {
		b, _ := os.ReadFile(so.Name())
		stdout.Write(b)
	}
	if stderr != nil {
		b, _ := os.ReadFile(se.Name())
		stderr.Write(b)
	}
	return code
}
