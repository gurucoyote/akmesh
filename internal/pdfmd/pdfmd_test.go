package pdfmd

import (
	"bytes"
	"compress/zlib"
	"fmt"
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

// TestTableFromColumns: three cells per row at fixed x positions are a table.
// A PDF has no table either - only cells that line up, which is what a reader
// can see and what pdf-inspector uses.
func TestTableFromColumns(t *testing.T) {
	content := "BT /F1 10 Tf 72 700 Td (Name) Tj 100 0 Td (Ort) Tj 100 0 Td (Zeit) Tj ET " +
		"BT /F1 10 Tf 72 680 Td (Anna) Tj 100 0 Td (Bremen) Tj 100 0 Td (10 Uhr) Tj ET"
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatal(err)
	}
	want := "| Name | Ort | Zeit |\n| --- | --- | --- |\n| Anna | Bremen | 10 Uhr |\n"
	if !strings.Contains(got, want) {
		t.Errorf("keine Tabelle aus den Spalten:\n%q", got)
	}
}

// TestProseStaysProse: a page without aligned cells must not become a table,
// or every paragraph of a book would.
func TestProseStaysProse(t *testing.T) {
	content := "BT /F1 10 Tf 72 700 Td (Ein Satz, der einfach nur Text ist.) Tj ET " +
		"BT /F1 10 Tf 72 680 Td (Noch ein Satz ohne Spalten.) Tj ET"
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "|") {
		t.Errorf("Fliesstext wurde zur Tabelle:\n%q", got)
	}
}

// TestTableFromRectangles: many calendars and invoices have no text columns at
// all - each cell carries a filled rectangle. Text and cells are both drawn
// through the current transformation matrix, so a reader that ignores it can
// not put them together.
func TestTableFromRectangles(t *testing.T) {
	var b strings.Builder
	// Two columns, two rows of 100x20 cells, with a 0.5 scale transform.
	b.WriteString(".5 0 0 .5 0 0 cm ")
	cells := [][2]string{{"A1", "B1"}, {"A2", "B2"}}
	for row := 0; row < 2; row++ {
		for col := 0; col < 2; col++ {
			x := 100 + col*100
			y := 600 - row*40
			b.WriteString(fmt.Sprintf("%d %d 100 20 re f ", x, y))
			b.WriteString(fmt.Sprintf("BT /F1 10 Tf %d %d Td (%s) Tj ET ", x+5, y+6, cells[row][col]))
		}
	}
	got, err := Convert(pageWith("", b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "| A1 | B1 |") || !strings.Contains(got, "| A2 | B2 |") {
		t.Errorf("keine Tabelle aus den Rechtecken:\n%q", got)
	}
}

// TestTextFromFormXObject: a form field, a stamp or a header of a form is often
// a form XObject - its own content stream with its own resources. A reader that
// only follows the page content loses that text, and in a form that is exactly
// the part that says what the field is.
func TestTextFromFormXObject(t *testing.T) {
	form := "BT /F1 11 Tf 5 6 Td (Kursgebuehr: 630) Tj ET"
	doc := pdf(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> /XObject << /X1 6 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Length 22 >>\nstream\nq 1 0 0 1 70 700 cm /X1 Do Q\nendstream",
		"<< /Type /XObject /Subtype /Form /BBox [0 0 200 20] /Resources << /Font << /F1 4 0 R >> >> /Length "+itoa(len(form))+" >>\nstream\n"+form+"\nendstream",
	)
	got, err := Convert(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Kursgebuehr: 630") {
		t.Errorf("Text aus dem Form-XObject fehlt:\n%q", got)
	}
}

// TestLabelValuePairs: a form sets a label, then its value a good distance to
// the right. The labels line up, the values do not, because a value is as wide as
// it happens to be. With the old 3pt alignment on every column such a pair stayed
// prose, which is the debt this test closes: the label column is what makes it a
// table, the value column may wander.
func TestLabelValuePairs(t *testing.T) {
	// A form as it is actually written: one text object per cell, the label at
	// x=72, the value somewhere to the right - and not at the same x in every
	// row, because the label width differs. Both have to survive: the baseline
	// keeps the cells on one row, and the label column is what makes it a table.
	rows := [][2]string{
		{"Kursgebuehr:", "630 EUR"},
		{"Anmeldung:", "bis 1. Maerz"},
		{"Ort:", "Bremen, Halle 4"},
	}
	var b strings.Builder
	for i, r := range rows {
		y := 700 - i*20
		b.WriteString("BT /F1 10 Tf 72 " + itoa(y) + " Td (" + r[0] + ") Tj ET ")
		b.WriteString("BT /F1 10 Tf " + itoa(170+i*7) + " " + itoa(y) + " Td (" + r[1] + ") Tj ET ")
	}
	got, err := Convert(pageWith("", b.String()))
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Kursgebuehr:", "Anmeldung:", "Ort:"} {
		if !strings.Contains(got, "| "+label) {
			t.Errorf("Label %q steht nicht als Tabellenzelle:\n%q", label, got)
		}
	}
	if strings.Count(got, "|") < 6 {
		t.Errorf("keine Tabelle aus den Label-Wert-Paaren:\n%q", got)
	}
	// The label and its value must share a row, not a line each.
	if strings.Contains(got, "Kursgebuehr:\n") {
		t.Errorf("Label und Wert stehen in getrennten Zeilen:\n%q", got)
	}
}

// TestColonProseStaysProse is the guard: two lines that merely end in a colon
// are not a table. Without this rule every two-line note would become one.
func TestColonProseStaysProse(t *testing.T) {
	content := "BT /F1 10 Tf 72 700 Td (Hinweis:) Tj ET " +
		"BT /F1 10 Tf 72 680 Td (Bitte pünktlich erscheinen und den Ausweis mitbringen.) Tj ET " +
		"BT /F1 10 Tf 72 660 Td (Fragen an das Büro.) Tj ET"
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "|") {
		t.Errorf("Fliesstext mit Doppelpunkt wurde zur Tabelle:\n%q", got)
	}
}

// TestLabelValueInOneCell is the case that was left as debt: the writer puts
// "Komplexität: 6" into a single run, with no cell gap at all, so the label and
// its value are one cell. Two or more such lines in a row are a form, and a form
// reads as a table - one line alone is still prose, or every note with a colon
// would become one.
func TestLabelValueInOneCell(t *testing.T) {
	content := "BT /F1 10 Tf 72 700 Td (Komplexitaet: 6) Tj ET " +
		"BT /F1 10 Tf 72 680 Td (Stimmung: unheimlich) Tj ET " +
		"BT /F1 10 Tf 72 660 Td (Ort: Pumpkin Mart) Tj ET"
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "| Komplexitaet: | 6 |") {
		t.Errorf("Label und Wert wurden nicht getrennt:\n%q", got)
	}
	if !strings.Contains(got, "| Stimmung: | unheimlich |") {
		t.Errorf("zweite Zeile nicht getrennt:\n%q", got)
	}
}

// TestSingleColonLineStaysProse guards the rule above: one line with a colon is
// a sentence, not a table.
func TestSingleColonLineStaysProse(t *testing.T) {
	content := "BT /F1 10 Tf 72 700 Td (Hinweis: bitte puenktlich kommen.) Tj ET " +
		"BT /F1 10 Tf 72 675 Td (Ein ganz normaler Satz, der weiterlaeuft und nichts mit einem Formular zu tun hat.) Tj ET"
	got, err := Convert(pageWith("", content))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "|") {
		t.Errorf("einzelne Zeile mit Doppelpunkt wurde zur Tabelle:\n%q", got)
	}
}
