package docxgen

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"akmesh/internal/docxmd"
)

// unzip reads a .docx back into its parts.
func unzip(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		out[f.Name] = string(b)
	}
	return out
}

// relIDs lists the r:id values a body uses, in order.
func relIDs(body string) []string {
	var out []string
	for {
		i := strings.Index(body, `r:id="`)
		if i < 0 {
			return out
		}
		body = body[i+len(`r:id="`):]
		j := strings.IndexByte(body, '"')
		if j < 0 {
			return out
		}
		out = append(out, body[:j])
		body = body[j:]
	}
}

// roundTrip is the whole point of this package: what we write must be readable
// by our own reader, because a .docx that only its author can open is worthless.
func roundTrip(t *testing.T, markdown string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Convert(&buf, []byte(markdown)); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	out, err := docxmd.Convert(buf.Bytes())
	if err != nil {
		t.Fatalf("the reader rejects what the writer produced: %v", err)
	}
	return out
}

// TestHeadingsAndParagraphs: the style a heading carries is how Word knows it is
// one, so the reader has to find a heading where the markdown had one.
func TestHeadingsAndParagraphs(t *testing.T) {
	got := roundTrip(t, "# Titel\n\nEin Absatz.\n\n## Zweite Ebene\n")
	if !strings.Contains(got, "# Titel") {
		t.Errorf("Heading 1 fehlt:\n%q", got)
	}
	if !strings.Contains(got, "Ein Absatz.") {
		t.Errorf("Absatz fehlt:\n%q", got)
	}
	if !strings.Contains(got, "## Zweite Ebene") {
		t.Errorf("Heading 2 fehlt:\n%q", got)
	}
}

// TestInlineFormatting: bold and italic are run properties, not characters.
func TestInlineFormatting(t *testing.T) {
	got := roundTrip(t, "Ein **fetter** und ein *kursiver* Teil.\n")
	if !strings.Contains(got, "**fetter**") {
		t.Errorf("fett fehlt:\n%q", got)
	}
	if !strings.Contains(got, "*kursiver*") {
		t.Errorf("kursiv fehlt:\n%q", got)
	}
}

// TestBulletAndNumberedList: a list in Word is a paragraph with a numbering
// reference, and the numbering part has to define that reference - one without
// the other shows no bullet at all.
func TestBulletAndNumberedList(t *testing.T) {
	got := roundTrip(t, "- eins\n- zwei\n\n1. erster\n2. zweiter\n")
	if !strings.Contains(got, "- eins") || !strings.Contains(got, "- zwei") {
		t.Errorf("Aufzaehlung fehlt:\n%q", got)
	}
	if !strings.Contains(got, "1. erster") || !strings.Contains(got, "2. zweiter") {
		t.Errorf("nummerierte Liste fehlt:\n%q", got)
	}
}

// TestTable: cells are table cells, not paragraphs with pipes.
func TestTable(t *testing.T) {
	got := roundTrip(t, "| Name | Ort |\n| --- | --- |\n| Anna | Bremen |\n")
	if !strings.Contains(got, "| Name | Ort |") {
		t.Errorf("Kopfzeile fehlt:\n%q", got)
	}
	if !strings.Contains(got, "| Anna | Bremen |") {
		t.Errorf("Datenzeile fehlt:\n%q", got)
	}
}

// TestCodeBlock keeps the code as its own paragraphs with a monospaced style; a
// code block that arrives as prose has lost its meaning.
func TestCodeBlock(t *testing.T) {
	got := roundTrip(t, "```\nfmt.Println(\"hi\")\n```\n")
	if !strings.Contains(got, "fmt.Println") {
		t.Errorf("Codeblock fehlt:\n%q", got)
	}
}

// TestUmlautsAndEscaping: the XML escaping must be right in both directions, and
// an ampersand in the text must not break the part.
func TestUmlautsAndEscaping(t *testing.T) {
	// In Markdown a bare <spitz> is an HTML tag, so the text has to be escaped
	// with a backslash - otherwise goldmark is right to drop it, not us.
	got := roundTrip(t, "Grüße & Umlaute: äöüß, \\<spitz\\> und \"Anführungszeichen\".\n")
	// The reader writes Markdown, so it escapes < and > with a backslash.
	// Dropping those escapes is the fair comparison: what matters is that the
	// characters survived the XML, not that the reader left them bare.
	plain := strings.ReplaceAll(got, "\\", "")
	for _, want := range []string{"Grüße", "äöüß", "&", "<spitz>"} {
		if !strings.Contains(plain, want) {
			t.Errorf("%q fehlt nach dem Rundlauf:\n%q", want, plain)
		}
	}
}

// TestEmptyDocument: an empty markdown file is still a valid document.
func TestEmptyDocument(t *testing.T) {
	var buf bytes.Buffer
	if err := Convert(&buf, nil); err != nil {
		t.Fatalf("Convert of an empty document: %v", err)
	}
	if _, err := docxmd.Convert(buf.Bytes()); err != nil {
		t.Fatalf("empty document is not readable: %v", err)
	}
}

// TestBlockquote is a paragraph with an indent; Word has no quote object.
func TestBlockquote(t *testing.T) {
	got := roundTrip(t, "> Ein Zitat.\n")
	if !strings.Contains(got, "Ein Zitat.") {
		t.Errorf("Zitat fehlt:\n%q", got)
	}
}

// TestLinksAreClickable is the acceptance of Martin's request of 2026-11-19: a
// Markdown link has to arrive as a link in Word, not as text with the address
// dropped. The relationship part is what makes the click work, and the reader
// has no business seeing the address as text next to the link.
func TestLinksAreClickable(t *testing.T) {
	markdown := "Ein [Verweis](https://example.com/x?a=1&b=2) und ein [Sprung](#titel).\n"
	var buf bytes.Buffer
	if err := Convert(&buf, []byte(markdown)); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	parts := unzip(t, buf.Bytes())

	doc := parts["word/document.xml"]
	if n := strings.Count(doc, "<w:hyperlink "); n != 1 {
		t.Errorf("hyperlinks in document.xml: %d, want 1 (the fragment stays text)", n)
	}
	// Without the r: namespace the r:id is an unknown attribute and the file is
	// refused outright; measured with LibreOffice.
	if !strings.Contains(doc, `xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`) {
		t.Errorf("document.xml lacks the r: namespace declaration")
	}
	rels := parts["word/_rels/document.xml.rels"]
	if !strings.Contains(rels, `Target="https://example.com/x?a=1&amp;b=2" TargetMode="External"`) {
		t.Errorf("the link's address is not in the relationship part:\n%s", rels)
	}
	if strings.Contains(rels, `Target="#titel"`) {
		t.Errorf("the fragment link became a relationship:\n%s", rels)
	}
	// Every r:id the body uses must be defined, or the link does nothing.
	for _, id := range relIDs(doc) {
		if !strings.Contains(rels, `Id="`+id+`"`) {
			t.Errorf("%s used in document.xml but not defined", id)
		}
	}
	if !strings.Contains(parts["word/styles.xml"], `w:styleId="Hyperlink"`) {
		t.Errorf("styles.xml defines no Hyperlink style")
	}
	if got := roundTrip(t, markdown); !strings.Contains(got, "Verweis") {
		t.Errorf("the reader lost the link text: %q", got)
	}
}
