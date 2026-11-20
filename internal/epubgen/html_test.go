package epubgen

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestHTMLBookKeepsStructure: the reason for HTML as a second source is that a
// table with a head row, a nested list and inline code come through unchanged,
// where a trip through Markdown may flatten them.
func TestHTMLBookKeepsStructure(t *testing.T) {
	page := `<!DOCTYPE html><html lang="de"><head><title>Seite</title></head><body>
<h1>Kapitel eins</h1>
<p>Text mit <code>inline</code> Code.</p>
<table><thead><tr><th>A</th><th>B</th></tr></thead><tbody><tr><td>1</td><td>2</td></tr></tbody></table>
<h2>Unterkapitel</h2>
<ul><li>eins<ul><li>tief</li></ul></li></ul>
</body></html>`
	var buf bytes.Buffer
	if err := ConvertHTML(&buf, []byte(page), Options{Title: "Seite"}); err != nil {
		t.Fatalf("ConvertHTML: %v", err)
	}
	parts := bookParts(t, buf.Bytes())
	ch := parts["OEBPS/chapter1.xhtml"]
	for _, want := range []string{"<h1>Kapitel eins</h1>", "<code>inline</code>", "<thead>", "<th>A</th>", "<li>tief</li>"} {
		if !strings.Contains(ch, want) {
			t.Errorf("chapter lacks %q:\n%s", want, ch)
		}
	}
	// The page's own head belongs to the page, not to the book: its title must
	// not turn up as the title of a chapter.
	if strings.Contains(ch, "<title>Seite</title>") {
		t.Errorf("the page title leaked into the chapter:\n%s", ch)
	}
	// Chapters are cut at the top-level headings, as in md2epub; a level-two
	// heading stays inside its chapter with its level intact.
	if !strings.Contains(ch, "<h2>Unterkapitel</h2>") {
		t.Errorf("the level-two heading lost its level:\n%s", ch)
	}
}

// TestHTMLBookCarriesItsFontsAndPictures: the book from HTML is a book like any
// other - fonts, licence page and pictures travel with it.
func TestHTMLBookCarriesItsFontsAndPictures(t *testing.T) {
	dir := t.TempDir()
	png := testPNG(t, 20, 10)
	if err := os.WriteFile(dir+"/bild.png", png, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	page := `<html><body><h1>Titel</h1><p><img src="bild.png" alt="Punkt"/></p></body></html>`
	var buf bytes.Buffer
	if err := ConvertHTML(&buf, []byte(page), Options{Title: "Titel", BaseDir: dir}); err != nil {
		t.Fatalf("ConvertHTML: %v", err)
	}
	parts := bookParts(t, buf.Bytes())
	if parts["OEBPS/images/bild1.png"] == "" {
		t.Errorf("the picture is missing: %v", keysOf(parts))
	}
	if !strings.Contains(parts["OEBPS/chapter1.xhtml"], `src="images/bild1.png"`) {
		t.Errorf("the chapter does not point at the copy")
	}
	if parts["OEBPS/colophon.xhtml"] == "" {
		t.Errorf("no licence page")
	}
	fonts := 0
	for name := range parts {
		if strings.HasPrefix(name, "OEBPS/fonts/") {
			fonts++
		}
	}
	if fonts == 0 {
		t.Errorf("no fonts in the book")
	}
}
