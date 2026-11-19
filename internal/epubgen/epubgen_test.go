package epubgen

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

// openConvert is the one constructor the command uses, so the tests go through
// it instead of assembling parts the product never builds.
func openConvert(t *testing.T, markdown string, opts Options) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	if err := Convert(&buf, []byte(markdown), opts); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("the result is not a zip: %v", err)
	}
	return r
}

// TestMimetypeComesFirstAndStored: an EPUB has to start with the mimetype
// entry, and it must be stored, not compressed. A reader that sees anything
// else refuses the file before it looks inside.
func TestMimetypeComesFirstAndStored(t *testing.T) {
	r := openConvert(t, "# Eins\n\nText.\n", Options{Title: "Probe"})
	if len(r.File) == 0 {
		t.Fatal("empty zip")
	}
	first := r.File[0]
	if first.Name != "mimetype" {
		t.Fatalf("first entry is %q, want mimetype", first.Name)
	}
	if first.Method != zip.Store {
		t.Errorf("mimetype is compressed (method %d), it has to be stored", first.Method)
	}
	rc, err := first.Open()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(rc)
	rc.Close()
	if got := string(body); got != "application/epub+zip" {
		t.Errorf("mimetype is %q", got)
	}
}

// TestRequiredPartsExist: without container.xml, the package document and a nav
// document the file is a zip, not an EPUB.
func TestRequiredPartsExist(t *testing.T) {
	r := openConvert(t, "# Eins\n\nText.\n", Options{Title: "Probe"})
	names := map[string]bool{}
	for _, f := range r.File {
		names[f.Name] = true
	}
	for _, want := range []string{"META-INF/container.xml", "OEBPS/package.opf", "OEBPS/nav.xhtml"} {
		if !names[want] {
			t.Errorf("%s fehlt", want)
		}
	}
}

// TestOneChapterPerHeading: a top-level heading starts a chapter, or the book is
// one endless scroll.
func TestOneChapterPerHeading(t *testing.T) {
	r := openConvert(t, "# Eins\n\nErster Text.\n\n# Zwei\n\nZweiter Text.\n", Options{Title: "Probe"})
	var chapters []string
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "OEBPS/chapter") && strings.HasSuffix(f.Name, ".xhtml") {
			chapters = append(chapters, f.Name)
		}
	}
	if len(chapters) != 2 {
		t.Fatalf("%d Kapitel, want 2: %v", len(chapters), chapters)
	}
}

// TestTextWithoutHeadingIsOneChapter: a document without a single heading must
// still become a readable book.
func TestTextWithoutHeadingIsOneChapter(t *testing.T) {
	r := openConvert(t, "Nur ein Absatz.\n\nUnd noch einer.\n", Options{Title: "Probe"})
	n := 0
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "OEBPS/chapter") && strings.HasSuffix(f.Name, ".xhtml") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d Kapitel, want 1", n)
	}
}

// TestUmlautsSurvive: the chapter is written as UTF-8 XML; a mojibake book is
// unreadable even if every part is present.
func TestUmlautsSurvive(t *testing.T) {
	r := openConvert(t, "# Grüße\n\näöüß und Straße.\n", Options{Title: "Probe"})
	found := false
	for _, f := range r.File {
		if !strings.HasSuffix(f.Name, ".xhtml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(rc)
		rc.Close()
		if strings.Contains(string(body), "äöüß") {
			found = true
		}
	}
	if !found {
		t.Error("die Umlaute sind in keinem Kapitel lesbar")
	}
}

// TestTitleInMetadata: the book title belongs in the package document, escaped.
func TestTitleInMetadata(t *testing.T) {
	r := openConvert(t, "# Eins\n", Options{Title: "Tom & Jerry"})
	for _, f := range r.File {
		if f.Name != "OEBPS/package.opf" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(rc)
		rc.Close()
		if !strings.Contains(string(body), "Tom &amp; Jerry") {
			t.Errorf("Titel nicht escaped im package.opf:\n%s", body)
		}
	}
}

// TestNestingKeepsHeadingLevel: a second-level heading is not a chapter, but it
// must not lose its level either.
func TestNestingKeepsHeadingLevel(t *testing.T) {
	r := openConvert(t, "# Eins\n\n## Unterabschnitt\n\nText.\n", Options{Title: "Probe"})
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, "OEBPS/chapter") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(rc)
		rc.Close()
		if !strings.Contains(string(body), "<h2") {
			t.Errorf("Unterueberschrift hat ihre Ebene verloren:\n%s", body)
		}
	}
}
