package docxgen

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

// htmlDocx converts a page the way html2docx does, without the file system.
func htmlDocx(t *testing.T, page, baseDir string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := ConvertWithBase(&buf, []byte(markdownOf(t, page)), baseDir); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	return buf.Bytes()
}

// markdownOf is the step html2docx takes before the writer: the page is read as
// Markdown with GFM tables.
func markdownOf(t *testing.T, page string) string {
	t.Helper()
	return convertPageForTest(page)
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png: %v", err)
	}
	return buf.Bytes()
}

// TestHTMLTableBecomesAWordTable: a table read from a page has to arrive as a
// table, not as lines of pipe-separated text.
func TestHTMLTableBecomesAWordTable(t *testing.T) {
	page := `<table><thead><tr><th>A</th><th>B</th></tr></thead><tbody><tr><td>1</td><td>2</td></tr><tr><td>3</td><td>4</td></tr></tbody></table>`
	parts := unzip(t, htmlDocx(t, page, ""))
	doc := parts["word/document.xml"]
	if n := strings.Count(doc, "<w:tbl>"); n != 1 {
		t.Fatalf("want one table, got %d:\n%s", n, doc)
	}
	for _, want := range []string{">A<", ">1<", ">4<"} {
		if !strings.Contains(doc, want) {
			t.Errorf("table lacks %q", want)
		}
	}
}

// TestHTMLListKeepsItsNesting: an item inside an item must be a deeper level,
// otherwise the list reads as one flat run.
func TestHTMLListKeepsItsNesting(t *testing.T) {
	page := `<ul><li>eins<ul><li>tief</li></ul></li><li>zwei</li></ul>`
	doc := unzip(t, htmlDocx(t, page, ""))["word/document.xml"]
	if !strings.Contains(doc, ">eins<") || !strings.Contains(doc, ">tief<") {
		t.Fatalf("the list text is missing:\n%s", doc)
	}
	if strings.Count(doc, "<w:numPr>") < 3 {
		t.Errorf("want a numbering reference per item:\n%s", doc)
	}
	// A nested item carries an indentation its parent does not.
	if !strings.Contains(doc, `<w:ilvl w:val="1"/>`) {
		t.Errorf("the nested item has no deeper level:\n%s", doc)
	}
}

// TestHTMLPictureIsEmbedded: a picture has to travel inside the package.
func TestHTMLPictureIsEmbedded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/bild.png", pngBytes(t, 12, 6), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	page := `<p><img src="bild.png" alt="Punkt"/></p>`
	parts := unzip(t, htmlDocx(t, page, dir))
	found := false
	for name := range parts {
		if strings.HasPrefix(name, "word/media/") {
			found = true
		}
	}
	if !found {
		var names []string
		for name := range parts {
			names = append(names, name)
		}
		t.Errorf("no picture part: %v", names)
	}
}

// TestPageTitleIsNotAParagraph: the title of a page is the page's name, not the
// first line of the document.
func TestPageTitleIsNotAParagraph(t *testing.T) {
	page := `<html><head><title>Probe</title></head><body><h1>Probe</h1><p>Text.</p></body></html>`
	doc := unzip(t, htmlDocx(t, page, ""))["word/document.xml"]
	if n := strings.Count(doc, ">Probe<"); n != 1 {
		t.Errorf("the title is in the document %d times:\n%s", n, doc)
	}
}

// TestHeadIsDropped is the unit of the same rule.
func TestHeadIsDropped(t *testing.T) {
	got := pageBody(`<html><head><title>T</title><style>p{}</style></head><body><p>a</p></body></html>`)
	if strings.Contains(got, "T") || strings.Contains(got, "style") {
		t.Errorf("the head survived: %q", got)
	}
	if !strings.Contains(got, "<p>a</p>") {
		t.Errorf("the body was lost: %q", got)
	}
}
