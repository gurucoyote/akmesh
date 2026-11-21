package docxmd

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// makeDocx builds the smallest docx that Word would accept: the content types,
// the document part and, when links is not empty, the relationship part. The
// tests use it instead of a checked-in binary, so the fixture and the claim sit
// next to each other.
func makeDocx(t *testing.T, document, rels string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`)
	add("word/document.xml", `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
 xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<w:body>`+document+`</w:body></w:document>`)
	if rels != "" {
		add("word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+rels+`</Relationships>`)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// mustConvert fails the test on an error, so that a claim about the Markdown is
// not hidden behind an error that arrives as an empty string.
func mustConvert(t *testing.T, data []byte) string {
	t.Helper()
	md, err := Convert(data)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	return md
}

// para wraps a run list in a paragraph, with an optional style name.
func para(style, runs string) string {
	ppr := ""
	if style != "" {
		ppr = `<w:pPr><w:pStyle w:val="` + style + `"/></w:pPr>`
	}
	return `<w:p>` + ppr + runs + `</w:p>`
}

// text is a plain run.
func text(s string) string { return `<w:r><w:t xml:space="preserve">` + s + `</w:t></w:r>` }

// TestHeadingsComeFromTheStyle: Word does not mark a heading with a character,
// it marks it with a paragraph style, and an agent reading the markdown needs
// the level back.
func TestHeadingsComeFromTheStyle(t *testing.T) {
	body := para("Heading1", text("Erste Ebene")) +
		para("Heading2", text("Zweite Ebene")) +
		para("Heading3", text("Dritte Ebene")) +
		para("", text("Fliesstext"))
	got := mustConvert(t, makeDocx(t, body, ""))
	want := "# Erste Ebene\n\n## Zweite Ebene\n\n### Dritte Ebene\n\nFliesstext\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestTitleStyleIsAHeading: "Title" is the style Word gives a document title,
// and it belongs at the top of the file, not as plain text.
func TestTitleStyleIsAHeading(t *testing.T) {
	got := mustConvert(t, makeDocx(t, para("Title", text("Mein Bericht")), ""))
	if got != "# Mein Bericht\n" {
		t.Errorf("got %q", got)
	}
}

// TestBoldAndItalicRuns: the formatting lives on the run, and a paragraph can
// mix them with plain text.
func TestBoldAndItalicRuns(t *testing.T) {
	body := para("", text("vor ")+
		`<w:r><w:rPr><w:b/></w:rPr><w:t>fett</w:t></w:r>`+
		text(" und ")+
		`<w:r><w:rPr><w:i/></w:rPr><w:t>kursiv</w:t></w:r>`)
	got := mustConvert(t, makeDocx(t, body, ""))
	if got != "vor **fett** und *kursiv*\n" {
		t.Errorf("got %q", got)
	}
}

// TestBulletList: a paragraph with numPr is a list item, and the bullet shape
// is in the numbering part in a real file; here the level alone has to be
// enough to produce an item.
func TestBulletList(t *testing.T) {
	item := func(s string) string {
		return `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr>` + text(s) + `</w:p>`
	}
	got := mustConvert(t, makeDocx(t, item("eins")+item("zwei"), ""))
	want := "- eins\n- zwei\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// TestNumberedList: numFmt "decimal" in the numbering part makes an ordered
// list, and the numbers have to be there or the document reads as one list.
func TestNumberedList(t *testing.T) {
	body := `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="2"/></w:numPr></w:pPr>` + text("erster") + `</w:p>` +
		`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="2"/></w:numPr></w:pPr>` + text("zweiter") + `</w:p>`
	numbering := `<?xml version="1.0" encoding="UTF-8"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/></w:lvl></w:abstractNum>
<w:abstractNum w:abstractNumId="1"><w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/></w:lvl></w:abstractNum>
<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
<w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>
</w:numbering>`
	got := mustConvert(t, makeDocxWithNumbering(t, body, numbering))
	want := "1. erster\n2. zweiter\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// TestTableBecomesPipeTable: a docx table is where an agent loses the most when
// it only gets text, so the rows have to survive as a GFM table.
func TestTableBecomesPipeTable(t *testing.T) {
	cell := func(s string) string { return `<w:tc><w:p>` + text(s) + `</w:p></w:tc>` }
	row := func(cells ...string) string { return `<w:tr>` + strings.Join(cells, "") + `</w:tr>` }
	body := `<w:tbl>` +
		row(cell("Name"), cell("Wert")) +
		row(cell("a"), cell("1")) +
		row(cell("b"), cell("2")) +
		`</w:tbl>`
	got := mustConvert(t, makeDocx(t, body, ""))
	want := "| Name | Wert |\n| --- | --- |\n| a | 1 |\n| b | 2 |\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestHyperlinkUsesTheRelationship: the address is not in the text, it is in the
// relationship part, and a converter that drops it loses the only link there is.
func TestHyperlinkUsesTheRelationship(t *testing.T) {
	body := `<w:p>` + text("siehe ") +
		`<w:hyperlink r:id="rId4">` + text("die Seite") + `</w:hyperlink></w:p>`
	rels := `<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="http://x.tld/seite" TargetMode="External"/>`
	got := mustConvert(t, makeDocx(t, body, rels))
	want := "siehe [die Seite](http://x.tld/seite)\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// TestParagraphsAreSeparated: two paragraphs are two paragraphs, not one long
// sentence.
func TestParagraphsAreSeparated(t *testing.T) {
	got := mustConvert(t, makeDocx(t, para("", text("eins"))+para("", text("zwei")), ""))
	if got != "eins\n\nzwei\n" {
		t.Errorf("got %q", got)
	}
}

// TestMarkdownCharactersAreEscaped: a document that talks about markup must not
// come back as markup.
func TestMarkdownCharactersAreEscaped(t *testing.T) {
	got := mustConvert(t, makeDocx(t, para("", text("5*3 und [x]")), ""))
	if !strings.Contains(got, `5\*3`) || !strings.Contains(got, `\[x\]`) {
		t.Errorf("got %q", got)
	}
}

// TestFirstLineKeepsItsRules: a document whose first paragraph starts with "1."
// must not come back as a list item, and one that starts with "#" must not
// become a heading.
func TestFirstLineKeepsItsRules(t *testing.T) {
	got := mustConvert(t, makeDocx(t, para("", text("1. Punkt"))+para("", text("# kein Titel")), ""))
	want := "1\\. Punkt\n\n\\# kein Titel\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// TestMarkersAlreadyProtectALine: a bold "1. Folge" is not a list item, because
// the markers come first; escaping the number inside the markers only makes the
// text harder to read.
func TestMarkersAlreadyProtectALine(t *testing.T) {
	bold := `<w:r><w:rPr><w:b/></w:rPr><w:t>1. Folge</w:t></w:r>`
	got := mustConvert(t, makeDocx(t, para("", bold), ""))
	if got != "**1. Folge**\n" {
		t.Errorf("got %q", got)
	}
}

// TestNotADocxIsAnError: the message has to say what was wrong, and the caller
// has to see a non-zero exit code, or an agent reads an empty conversion as an
// empty document. The exit code itself is any2md's business now; what this
// package owes is the reason.
func TestNotADocxIsAnError(t *testing.T) {
	if _, err := Convert([]byte("das ist kein zip")); err == nil {
		t.Fatal("no error for a file that is not a docx")
	} else if !strings.Contains(err.Error(), "zip") {
		t.Errorf("error does not name the reason: %q", err)
	}
}

// TestDocxWithoutDocumentPartIsAnError: a zip that is not a Word file must not
// come back as an empty markdown file either.
func TestDocxWithoutDocumentPartIsAnError(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("readme.txt")
	w.Write([]byte("hallo"))
	zw.Close()

	_, err := Convert(buf.Bytes())
	if err == nil {
		t.Fatal("no error for a zip without word/document.xml")
	}
	if !strings.Contains(err.Error(), "document.xml") {
		t.Errorf("error does not name the missing part: %q", err)
	}
}

// makeDocxWithNumbering is makeDocx plus the numbering part, which is where the
// bullet or decimal shape of a list is decided.
func makeDocxWithNumbering(t *testing.T, document, numbering string) []byte {
	t.Helper()
	base := makeDocx(t, document, "")
	zr, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if _, err := b.ReadFrom(rc); err != nil {
			t.Fatal(err)
		}
		rc.Close()
		w.Write(b.Bytes())
	}
	w, err := zw.Create("word/numbering.xml")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(numbering))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestLineBreakBetweenRuns: Word writes a line break as a w:br that is a
// sibling of the runs, not inside one. Without it, a list of show notes turns
// into one long sentence.
func TestLineBreakBetweenRuns(t *testing.T) {
	body := `<w:p>` + text("eins") + `<w:br/>` + text("zwei") + `</w:p>`
	got := mustConvert(t, makeDocx(t, body, ""))
	if got != "eins  \nzwei\n" {
		t.Errorf("got %q", got)
	}
}

// TestAdjacentBoldRunsMerge: Word splits a bold sentence into several runs
// whenever the spelling or the language changes, and closing and reopening the
// markers gives four asterisks, which is no emphasis at all.
func TestAdjacentBoldRunsMerge(t *testing.T) {
	bold := func(s string) string {
		return `<w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">` + s + `</w:t></w:r>`
	}
	body := `<w:p>` + bold("fett ") + bold("auch fett") + `</w:p>`
	got := mustConvert(t, makeDocx(t, body, ""))
	if got != "**fett auch fett**\n" {
		t.Errorf("got %q", got)
	}
}

// TestNumbersInTheMiddleAreNotEscaped: "11. Juni" is a date, not a list item.
// The escaping exists for the start of a line, and applying it everywhere makes
// an ordinary German sentence unreadable.
func TestNumbersInTheMiddleAreNotEscaped(t *testing.T) {
	// The date sits in its own run, which is how Word splits a paragraph; the
	// escape must not fire just because the run starts there.
	got := mustConvert(t, makeDocx(t, para("", text("Stand: ")+text("11. Juni")), ""))
	if got != "Stand: 11. Juni\n" {
		t.Errorf("got %q", got)
	}
}
