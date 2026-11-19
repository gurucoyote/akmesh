package epubmd

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"acmesh/internal/epubgen"
)

// buildEPUB writes a minimal EPUB with the parts in the given spine order. The
// file names are deliberately not in reading order: a reader that sorts by name
// gets the book backwards, and that is the property this fixture exists for.
func buildEPUB(t *testing.T, spine []string, chapters map[string]string) []byte {
	return buildEPUBLinear(t, spine, chapters, nil)
}

// buildEPUBLinear additionally marks some spine entries as linear="no", which is
// how a book keeps an appendix out of the reading flow.
func buildEPUBLinear(t *testing.T, spine []string, chapters map[string]string, nonLinear []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	write := func(name, body string, store bool) {
		var (
			w   io.Writer
			err error
		)
		if store {
			w, err = zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		} else {
			w, err = zw.Create(name)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	}

	write("mimetype", "application/epub+zip", true)
	write("META-INF/container.xml",
		`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">`+
			`<rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`, false)

	var items, refs strings.Builder
	nonLinearSet := map[string]bool{}
	for _, n := range nonLinear {
		nonLinearSet[n] = true
	}
	for i, name := range spine {
		// nav.xhtml is declared once, with its properties, before the
		// chapters; in the spine it appears only when the fixture asks for it.
		if name == "nav.xhtml" {
			refs.WriteString(`<itemref idref="nav"/>`)
			continue
		}
		items.WriteString(`<item id="c` + itoa(i) + `" href="` + name + `" media-type="application/xhtml+xml"/>`)
		if nonLinearSet[name] {
			refs.WriteString(`<itemref idref="c` + itoa(i) + `" linear="no"/>`)
		} else {
			refs.WriteString(`<itemref idref="c` + itoa(i) + `"/>`)
		}
	}
	write("OEBPS/package.opf",
		`<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">`+
			`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Probe</dc:title><dc:language>de</dc:language>`+
			`<dc:identifier id="id">urn:uuid:probe</dc:identifier></metadata>`+
			`<manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`+
			items.String()+`</manifest><spine>`+refs.String()+`</spine></package>`, false)

	write("OEBPS/nav.xhtml",
		`<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Inhalt</title></head>`+
			`<body><nav epub:type="toc" xmlns:epub="http://www.idpf.org/2007/ops"><ol><li><a href="c0.xhtml">Kapitel</a></li></ol></nav></body></html>`, false)

	for name, body := range chapters {
		if name == "nav.xhtml" {
			continue
		}
		write("OEBPS/"+name,
			`<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>x</title></head><body>`+body+`</body></html>`, false)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func itoa(i int) string {
	return string(rune('0' + i))
}

// TestChaptersInSpineOrder: the reading order is the package's spine, never the
// file names.
func TestChaptersInSpineOrder(t *testing.T) {
	data := buildEPUB(t,
		[]string{"chapter9.xhtml", "chapter2.xhtml"},
		map[string]string{
			"chapter9.xhtml": `<h1>Zuerst</h1><p>Der erste Text.</p>`,
			"chapter2.xhtml": `<h1>Danach</h1><p>Der zweite Text.</p>`,
		})
	got, err := Convert(data)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	a := strings.Index(got, "Zuerst")
	b := strings.Index(got, "Danach")
	if a < 0 || b < 0 {
		t.Fatalf("Kapitel fehlen:\n%s", got)
	}
	if a > b {
		t.Errorf("Reihenfolge ist nach Dateiname sortiert statt nach spine:\n%s", got)
	}
}

// TestNavIsNotAChapter: nav.xhtml carries the table of contents, and printing it
// as a chapter puts the contents into the book twice. It stands in the spine
// here, because a test that leaves it out cannot fail.
func TestNavIsNotAChapter(t *testing.T) {
	data := buildEPUB(t,
		[]string{"nav.xhtml", "c0.xhtml"},
		map[string]string{"c0.xhtml": `<h1>Eins</h1><p>Text.</p>`})
	got, err := Convert(data)
	if err != nil {
		t.Fatal(err)
	}
	// The nav has a positive control of its own: its text exists in the file,
	// so "Inhalt" appearing would mean it was read as a chapter.
	if strings.Contains(got, "Inhalt") {
		t.Errorf("nav.xhtml wurde als Kapitel gelesen:\n%s", got)
	}
	if !strings.Contains(got, "Eins") {
		t.Errorf("das echte Kapitel fehlt, der Test waere auch bei leerer Ausgabe gruen:\n%s", got)
	}
}

// TestHeadingAndParagraphBecomeMarkdown: the chapter body arrives as Markdown,
// not as HTML with tags in it.
func TestHeadingAndParagraphBecomeMarkdown(t *testing.T) {
	data := buildEPUB(t,
		[]string{"c0.xhtml"},
		map[string]string{"c0.xhtml": `<h1>Eins</h1><p>Ein <strong>fetter</strong> Satz.</p>`})
	got, err := Convert(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "# Eins") {
		t.Errorf("Ueberschrift fehlt:\n%s", got)
	}
	if !strings.Contains(got, "**fetter**") {
		t.Errorf("fett fehlt:\n%s", got)
	}
	if strings.Contains(got, "<p>") || strings.Contains(got, "<h1>") {
		t.Errorf("HTML ist stehengeblieben:\n%s", got)
	}
	// Head and stylesheet are not text: a book read from the top would start
	// with the XML declaration and the CSS rules of the reader's own style.
	for _, junk := range []string{"<?xml", "DOCTYPE", "font-family"} {
		if strings.Contains(got, junk) {
			t.Errorf("Kopf oder Stylesheet ist im Text gelandet (%q):\n%s", junk, got)
		}
	}
}

// TestListAndTable: the two shapes that collapse silently into running text.
func TestListAndTable(t *testing.T) {
	data := buildEPUB(t,
		[]string{"c0.xhtml"},
		map[string]string{"c0.xhtml": `<h1>Eins</h1><ul><li>eins</li><li>zwei</li></ul>` +
			`<table><tr><th>Name</th><th>Ort</th></tr><tr><td>Anna</td><td>Bremen</td></tr></table>`})
	got, err := Convert(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "- eins") {
		t.Errorf("Liste fehlt:\n%s", got)
	}
	if !strings.Contains(got, "| Name | Ort |") {
		t.Errorf("Tabelle fehlt:\n%s", got)
	}
}

// TestNotAnEpub: a zip that is not an EPUB has to be refused, not read as an
// empty book.
func TestNotAnEpub(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("liesmich.txt")
	w.Write([]byte("nichts"))
	zw.Close()
	if _, err := Convert(buf.Bytes()); err == nil {
		t.Error("ein Zip ohne container.xml wurde als Buch angenommen")
	}
}

// TestRoundtripWithWriter is the claim that matters for the pair: what md2epub
// writes has to come back through epub2md.
func TestRoundtripWithWriter(t *testing.T) {
	var buf bytes.Buffer
	md := "# Eins\n\nEin Absatz.\n\n- punkt\n\n| Name | Ort |\n| --- | --- |\n| Anna | Bremen |\n"
	if err := epubgen.Convert(&buf, []byte(md), epubgen.Options{Title: "Probe"}); err != nil {
		t.Fatal(err)
	}
	got, err := Convert(buf.Bytes())
	if err != nil {
		t.Fatalf("der eigene Schreiber erzeugt ein Buch, das der Leser nicht liest: %v", err)
	}
	for _, want := range []string{"# Eins", "Ein Absatz.", "- punkt", "| Name | Ort |", "| Anna | Bremen |"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q fehlt nach dem Rundlauf:\n%s", want, got)
		}
	}
}
