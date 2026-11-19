package pdfgen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// needTools skips a test when the PDF readers it checks against are missing: the
// properties below are about what a reader finds, which the PDF bytes do not say.
func needTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"pdftotext", "pdfinfo"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
}

// writePDF renders the document and returns the path of the result and its page
// count, so a test walks the pages the renderer really produced.
func writePDF(t *testing.T, source string) (string, int) {
	t.Helper()
	needTools(t)
	var buf bytes.Buffer
	if err := Convert(&buf, []byte(source)); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	path := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("pdfinfo", path).Output()
	if err != nil {
		t.Fatalf("pdfinfo: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Pages:")))
			if err != nil {
				t.Fatal(err)
			}
			return path, n
		}
	}
	t.Fatalf("no page count in pdfinfo output")
	return "", 0
}

// pageText reads one page of a written PDF.
func pageText(t *testing.T, path string, page int) string {
	t.Helper()
	out, err := exec.Command("pdftotext", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), path, "-").Output()
	if err != nil {
		t.Fatalf("pdftotext page %d: %v", page, err)
	}
	return string(out)
}

// longDoc is a document of many pages: enough paragraphs to overflow any sensible
// page size, so the tests do not depend on where exactly a break falls.
func longDoc() string {
	var b strings.Builder
	b.WriteString("# Titel über Öl\n\n")
	for i := 0; i < 200; i++ {
		b.WriteString("Zeile mit etwas Text, damit die Seite voll wird und die Umbrüche geprüft werden können.\n")
	}
	return b.String()
}

// TestEveryPageNumbered is the property the footer exists for: on every page the
// reader finds that page's own number and the real total.
func TestEveryPageNumbered(t *testing.T) {
	path, pages := writePDF(t, longDoc())
	if pages < 3 {
		t.Fatalf("document has %d pages, the test needs at least three", pages)
	}
	for page := 1; page <= pages; page++ {
		want := "Seite " + strconv.Itoa(page) + " von " + strconv.Itoa(pages)
		if text := pageText(t, path, page); !strings.Contains(text, want) {
			t.Fatalf("page %d of %d misses %q", page, pages, want)
		}
	}
}

// TestFooterShowsTheTotal guards the {nb} alias: the total is unknown while the
// last page is being written, so an unresolved alias would print literally.
func TestFooterShowsTheTotal(t *testing.T) {
	path, pages := writePDF(t, longDoc())
	text := pageText(t, path, 1)
	if strings.Contains(text, "{nb}") {
		t.Fatalf("the page count alias was not resolved: %q", text)
	}
	if !strings.Contains(text, "von "+strconv.Itoa(pages)) {
		t.Fatalf("footer does not name the total of %d pages: %q", pages, text)
	}
}

// TestHeaderOnlyFromSecondPage: the first page opens with the title anyway, so a
// running header there would print it twice.
func TestHeaderOnlyFromSecondPage(t *testing.T) {
	path, pages := writePDF(t, longDoc())
	if pages < 2 {
		t.Skipf("document has only %d page", pages)
	}
	const title = "Titel über Öl"
	if first := pageText(t, path, 1); strings.Count(first, title) != 1 {
		t.Fatalf("title appears %d times on page 1, want once", strings.Count(first, title))
	}
	if second := pageText(t, path, 2); strings.Count(second, title) != 1 {
		t.Fatalf("title appears %d times on page 2, want once in the header", strings.Count(second, title))
	}
}

// TestAmpersandIsNotEscaped: a PDF is not HTML, so an ampersand must reach the
// reader as one character instead of the entity HTML would need.
func TestAmpersandIsNotEscaped(t *testing.T) {
	path, _ := writePDF(t, "# Müller & Söhne\n\nEin Absatz.\n")
	text := pageText(t, path, 1)
	if strings.Contains(text, "&amp;") {
		t.Fatalf("ampersand was written as an HTML entity: %q", text)
	}
	if !strings.Contains(text, "Müller & Söhne") {
		t.Fatalf("title is damaged: %q", text)
	}
}
