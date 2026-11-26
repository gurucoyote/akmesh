package anymd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gurucoyote/akmesh/internal/docxgen"
)

// runIn calls Run or RunAs with streams backed by temp files, so that the test
// exercises the real signature the multicall binary uses.
func runIn(t *testing.T, stdin []byte, args []string, call func([]string, *os.File, *os.File, *os.File) int) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in")
	out := filepath.Join(dir, "out")
	errf := filepath.Join(dir, "err")
	if err := os.WriteFile(in, stdin, 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer fi.Close()
	fo, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	fe, err := os.Create(errf)
	if err != nil {
		t.Fatal(err)
	}
	code := call(args, fi, fo, fe)
	fo.Close()
	fe.Close()
	stdout, _ := os.ReadFile(out)
	stderr, _ := os.ReadFile(errf)
	return string(stdout), string(stderr), code
}

func docxOf(t *testing.T, markdown string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := docxgen.Convert(&buf, []byte(markdown)); err != nil {
		t.Fatalf("build docx: %v", err)
	}
	return buf.Bytes()
}

// TestRunReadsHTMLFromStdin is the promise of the name: pipe a document in, get
// Markdown out, without saying which format it is.
func TestRunReadsHTMLFromStdin(t *testing.T) {
	out, errs, code := runIn(t, []byte("<!DOCTYPE html><html><body><h1>Titel</h1></body></html>"),
		nil, Run)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
	if !strings.Contains(out, "# Titel") {
		t.Fatalf("stdout %q, want a level-1 heading", out)
	}
}

// TestRunReadsDocx: a zip is detected by its entries, not by a file name, and
// the Word reader produces the Markdown. Building the document with our own
// writer keeps the test free of a binary fixture.
func TestRunReadsDocx(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "angebot")
	if err := os.WriteFile(name, docxOf(t, "# Titel\n\nAbsatz.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errs, code := runIn(t, nil, []string{name}, Run)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
	if !strings.Contains(out, "Titel") || !strings.Contains(out, "Absatz") {
		t.Fatalf("stdout %q, want the text of the document", out)
	}
}

// TestRunOutputWritesFile: -o puts the Markdown into the file, as docx2md always
// did, so that the pictures can land beside it.
func TestRunOutputWritesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "angebot.docx")
	if err := os.WriteFile(src, docxOf(t, "# Titel\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "angebot.md")
	out, errs, code := runIn(t, nil, []string{"-o", target, src}, Run)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
	if out != "" {
		t.Fatalf("stdout %q, want nothing with -o", out)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("output file: %v", err)
	}
	if !strings.Contains(string(data), "Titel") {
		t.Fatalf("output file %q", data)
	}
}

// TestRunRefusesOutputForSeveralFiles: one -o and two inputs would leave one
// result where two belong. That was silent in docx2md; here it is an error.
func TestRunRefusesOutputForSeveralFiles(t *testing.T) {
	_, errs, code := runIn(t, nil, []string{"-o", "x.md", "a.html", "b.html"}, Run)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(errs, "-o") {
		t.Fatalf("stderr %q, want it to name -o", errs)
	}
}

// TestRunNamesFormatsOnUnknownBytes: the message has to say what any2md reads,
// otherwise the agent's next move is a guess.
func TestRunNamesFormatsOnUnknownBytes(t *testing.T) {
	out, errs, code := runIn(t, []byte("just text\n"), nil, Run)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if out != "" {
		t.Fatalf("stdout %q, want nothing", out)
	}
	for _, f := range Formats {
		if !strings.Contains(errs, string(f)) {
			t.Errorf("stderr %q does not name %q", errs, f)
		}
	}
}

// TestRunContinuesAfterAMissingFile: with two files where one is gone, the other
// is still converted and the exit code reports the failure.
func TestRunContinuesAfterAMissingFile(t *testing.T) {
	dir := t.TempDir()
	html := filepath.Join(dir, "seite.html")
	if err := os.WriteFile(html, []byte("<h2>Zweite</h2>"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errs, code := runIn(t, nil, []string{filepath.Join(dir, "fehlt.html"), html}, Run)
	if code != 1 {
		t.Fatalf("exit %d, want 1, stderr %q", code, errs)
	}
	if !strings.Contains(out, "Zweite") {
		t.Fatalf("stdout %q, want the second file converted", out)
	}
}

// TestRunAsPinsTheFormat is what makes pdf2md an alias instead of a second
// implementation: with PDF pinned, an HTML file is refused with the alias's own
// name in the message, and the message says what the caller asked for.
func TestRunAsPinsTheFormat(t *testing.T) {
	call := func(args []string, in, out, errf *os.File) int {
		return RunAs("pdf2md", PDF, args, in, out, errf)
	}
	out, errs, code := runIn(t, []byte("<h1>Titel</h1>"), nil, call)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if out != "" {
		t.Fatalf("stdout %q, want nothing", out)
	}
	if !strings.HasPrefix(errs, "pdf2md: ") || !strings.Contains(errs, "pdf") {
		t.Fatalf("stderr %q, want it to start with pdf2md and name pdf", errs)
	}
}

// TestRunAsAcceptsItsOwnFormat: pinning must not refuse a document that is what
// it says. A docx read as docx has to go through unchanged.
func TestRunAsAcceptsItsOwnFormat(t *testing.T) {
	call := func(args []string, in, out, errf *os.File) int {
		return RunAs("docx2md", DOCX, args, in, out, errf)
	}
	out, errs, code := runIn(t, docxOf(t, "# Titel\n"), nil, call)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
	if !strings.Contains(out, "Titel") {
		t.Fatalf("stdout %q", out)
	}
}
