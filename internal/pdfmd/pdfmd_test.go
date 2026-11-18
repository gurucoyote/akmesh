package pdfmd

import (
	"bytes"
	"compress/zlib"
	"strings"
	"testing"
)

// pdf builds a document from object bodies. Object 1 is the catalog, object 2
// the page tree, and the caller adds the page and the content. The reader scans
// for objects rather than following the cross-reference table, so the fixture
// needs no offsets - which is also what happens in a file whose xref is stale.
func pdf(objects ...string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	for i, body := range objects {
		b.WriteString(itoa(i+1) + " 0 obj\n" + body + "\nendobj\n")
	}
	b.WriteString("trailer\n<< /Root 1 0 R >>\n%%EOF\n")
	return b.Bytes()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var s string
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// pageWith builds a one-page document whose content stream is content. Object 4
// is always a font, because that is what the text operators reference.
func pageWith(fontDict string, content string) []byte {
	res := "<< /Font << /F1 4 0 R >> >>"
	return pdf(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Resources "+res+" /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"+fontDict,
		"<< /Length "+itoa(len(content))+" >>\nstream\n"+content+"\nendstream",
	)
}

// TestSimpleText: the operators a word processor writes, in the order it writes
// them: begin text, set font, put a line, next line, put a line, end.
func TestSimpleText(t *testing.T) {
	content := "BT /F1 12 Tf 72 700 Td (Hallo Welt) Tj 0 -14 Td (Zweite Zeile) Tj ET"
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(got, "Hallo Welt") || !strings.Contains(got, "Zweite Zeile") {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(got, "Hallo Welt\n") {
		t.Errorf("the second Td is a new line, got %q", got)
	}
}

// TestTJArray: TJ carries the pieces of one line with kerning numbers between
// them; the numbers are not text.
func TestTJArray(t *testing.T) {
	content := "BT /F1 12 Tf 72 700 Td [(Ker) -20 (ning) -300 (trennt)] TJ ET"
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Kerning") {
		t.Errorf("got %q", got)
	}
	if strings.Contains(got, "-20") || strings.Contains(got, "-300") {
		t.Errorf("kerning numbers leaked into the text: %q", got)
	}
}

// TestEscapesInStrings: a string in a PDF uses \\ ( ) with backslashes, and the
// text must come out as the characters they stand for.
func TestEscapesInStrings(t *testing.T) {
	content := `BT /F1 12 Tf 72 700 Td (Klammer \(so\) und \\ Schraeg) Tj ET`
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Klammer (so) und \\ Schraeg") {
		t.Errorf("got %q", got)
	}
}

// TestWinAnsiBytes: a single-byte font with WinAnsi means 0xE4 is ä.
func TestWinAnsiBytes(t *testing.T) {
	content := "BT /F1 12 Tf 72 700 Td (Gr\\366\\337e) Tj ET"
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Größe") {
		t.Errorf("got %q", got)
	}
}

// TestToUnicodeMap: a font that is not Latin-1 carries a map, and without it
// the bytes are meaningless. This is what a PDF from Word needs.
func TestToUnicodeMap(t *testing.T) {
	font := ` /ToUnicode 6 0 R`
	cmap := `<< /Length 190 >>
stream
/CIDInit /ProcSet findresource begin
begincmap
/CMapName /Adobe-Identity-UCS def
1 begincodespacerange
<00> <FF>
endcodespacerange
2 beginbfchar
<41> <00E4>
<42> <00F6>
endbfchar
endcmap
endstream`
	doc := pdf(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Custom /Encoding /WinAnsiEncoding"+font+" >>",
		"<< /Length 40 >>\nstream\nBT /F1 12 Tf 72 700 Td (AB) Tj ET\nendstream",
		cmap,
	)
	got, err := Convert(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "äö") {
		t.Errorf("got %q", got)
	}
}

// TestCompressedStream: real documents are Flate compressed, and a reader that
// only sees compressed bytes finds no text at all.
func TestCompressedStream(t *testing.T) {
	content := "BT /F1 12 Tf 72 700 Td (Komprimiert) Tj ET"
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write([]byte(content))
	zw.Close()
	body := []byte("<< /Length " + itoa(buf.Len()) + " /Filter /FlateDecode >>\nstream\n")
	body = append(body, buf.Bytes()...)
	body = append(body, []byte("\nendstream")...)

	doc := pdf(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		string(body),
	)
	got, err := Convert(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Komprimiert") {
		t.Errorf("got %q", got)
	}
}

// TestTwoPagesKeepTheirOrder: page two comes after page one, not in object
// order, because the page tree says so.
func TestTwoPagesKeepTheirOrder(t *testing.T) {
	doc := pdf(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << >> /Contents 4 0 R >>",
		"<< /Length 30 >>\nstream\nBT 72 700 Td (Erste Seite) Tj ET\nendstream",
		"<< /Type /Page /Parent 2 0 R /Resources << >> /Contents 7 0 R >>",
		"<< /Type /Page /Parent 2 0 R /Resources << >> /Contents 7 0 R >>",
		"<< /Length 30 >>\nstream\nBT 72 700 Td (Zweite Seite) Tj ET\nendstream",
	)
	got, err := Convert(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(got, "Erste Seite") > strings.Index(got, "Zweite Seite") {
		t.Errorf("pages out of order: %q", got)
	}
}

// TestNotAPdfIsAnError: an empty extraction must not look like an empty file.
func TestNotAPdfIsAnError(t *testing.T) {
	if _, err := Convert([]byte("das ist kein pdf")); err == nil {
		t.Errorf("no error for a file that is not a PDF")
	}
}

// TestEmptyPageIsEmptyText: a page with no text operator (an image-only page)
// is not an error, but it also must not invent text.
func TestEmptyPageIsEmptyText(t *testing.T) {
	doc := pdf(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << >> /Contents 4 0 R >>",
		"<< /Length 22 >>\nstream\n0 0 100 100 re f\nendstream",
	)
	got, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if strings.TrimSpace(got) != "" {
		t.Errorf("invented text: %q", got)
	}
}

// TestHeadingFromFontSize: a PDF carries no heading, only a size. A line set
// much larger than the body text is a heading, and the level follows the
// distance from the body size (this is what pdf-inspector does with its size
// tiers).
func TestHeadingFromFontSize(t *testing.T) {
	content := "BT /F1 24 Tf 72 760 Td (Kapitel Eins) Tj ET " +
		"BT /F1 10 Tf 72 720 Td (Der Text beginnt hier und laeuft weiter.) Tj ET " +
		"BT /F1 10 Tf 72 700 Td (Und noch eine Zeile Text.) Tj ET"
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "# Kapitel Eins\n") {
		t.Errorf("no heading from the size: %q", got)
	}
	if strings.Contains(got, "# Der Text") {
		t.Errorf("body text became a heading: %q", got)
	}
}

// TestNoHeadingWithoutContrast: a document set in one size has no headings, or
// every line would become one.
func TestNoHeadingWithoutContrast(t *testing.T) {
	content := "BT /F1 10 Tf 72 760 Td (Erste Zeile.) Tj ET " +
		"BT /F1 10 Tf 72 740 Td (Zweite Zeile.) Tj ET"
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "#") {
		t.Errorf("headings without a size difference: %q", got)
	}
}
