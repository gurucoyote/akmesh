package rtfmd

import (
	"strings"
	"testing"
)

// TestTableBecomesAPipeTable: before, \cell and \row were not read at all, so
// the cells of a row ran together into one word ("NameWert") and a whole table
// arrived as one paragraph. Markdown has no table without a header row, so the
// first row becomes one (acmesh-crc9).
func TestTableBecomesAPipeTable(t *testing.T) {
	body := `\pard Vorher\par` +
		`\trowd\trhdr\cellx2000\cellx4000 Name\cell Wert\cell\row` +
		`\trowd\cellx2000\cellx4000 Alpha\cell Eins\cell\row` +
		`\trowd\cellx2000\cellx4000 Beta\cell Zwei\cell\row` +
		`\pard Nachher\par`
	got := mustConvert(t, rtf(body))
	want := "Vorher\n\n" +
		"| Name | Wert |\n" +
		"| --- | --- |\n" +
		"| Alpha | Eins |\n" +
		"| Beta | Zwei |\n\n" +
		"Nachher\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// TestTableRowWithoutTrailingCell: a row that ends with \row and no \cell must
// not gain an empty column.
func TestTableRowWithoutTrailingCell(t *testing.T) {
	body := `\trowd\cellx1000\cellx2000 A\cell B\cell\row` + `\trowd\cellx1000\cellx2000 C\cell D\row`
	got := mustConvert(t, rtf(body))
	for _, line := range strings.Split(got, "\n") {
		if strings.Count(line, "|") != 3 && line != "" {
			t.Errorf("line %q has %d pipes, want 3", line, strings.Count(line, "|"))
		}
	}
	if !strings.Contains(got, "| C | D |") {
		t.Errorf("last row is not a row of two cells: %q", got)
	}
}

// TestHyperlinkKeepsTextAndTarget: a link used to disappear as a whole, because
// \field and \fldinst were skipped and the result text with them. That hit
// any2md and rg --pre alike (acmesh-crc9).
func TestHyperlinkKeepsTextAndTarget(t *testing.T) {
	body := `\pard Ein {\field{\*\fldinst{HYPERLINK "https://example.com/x"}}{\fldrslt{\ul Verweis}}} im Satz.\par`
	got := mustConvert(t, rtf(body))
	if got != "Ein [Verweis](https://example.com/x) im Satz.\n" {
		t.Errorf("got %q", got)
	}
}

// TestFieldWithoutHyperlinkKeepsItsText: a DATE field has no address, its result
// is plain text - the text must survive, the field itself must not.
func TestFieldWithoutHyperlinkKeepsItsText(t *testing.T) {
	body := `\pard Datum: {\field{\*\fldinst{DATE}}{\fldrslt{24.11.2026}}}\par`
	got := mustConvert(t, rtf(body))
	if got != "Datum: 24.11.2026\n" {
		t.Errorf("got %q", got)
	}
}

// TestListLevelsAreIndented: \li carries the indent, 720 twips is one level.
func TestListLevelsAreIndented(t *testing.T) {
	body := `\pard\li360{\pntext\f0 1.\tab}\pn\pnlvlbody\li360 Eins\par` +
		`\pard\li1080{\pntext\f0 a)\tab}\pn\pnlvlblt\li1080 Zwei\par` +
		`\pard\li1800{\pntext\f0 i.\tab}\pn\pnlvlblt\li1800 Drei\par` +
		`\pard Ende.\par`
	got := mustConvert(t, rtf(body))
	want := "1. Eins\n  - Zwei\n    - Drei\n\nEnde.\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// TestLinkWithSpaceInTarget: an address with a space would end the Markdown
// destination, so it goes into angle brackets.
func TestLinkWithSpaceInTarget(t *testing.T) {
	body := `\pard {\field{\*\fldinst{HYPERLINK "https://example.com/a b"}}{\fldrslt{Text}}}\par`
	got := mustConvert(t, rtf(body))
	if !strings.Contains(got, "(<https://example.com/a b>)") {
		t.Errorf("got %q", got)
	}
}

// TestUnicodeFallbackInTextRun: the fallback characters of \uN can arrive as a
// text run, not only one by one; both have to be dropped (found in suntoad's
// copy on 25.11.2026, the same defect was here).
func TestUnicodeFallbackInTextRun(t *testing.T) {
	got := mustConvert(t, rtf(`\pard \uc2\u8364?? Ende\par`))
	if got != "€ Ende\n" {
		t.Errorf("got %q", got)
	}
}

// TestOtherCodepageIsRefused: only cp1252 is mapped, so a document in another
// code page must fail loudly instead of coming out as mojibake. Before
// acmesh-crc9 it was read silently and wrongly.
func TestOtherCodepageIsRefused(t *testing.T) {
	_, err := Convert([]byte(`{\rtf1\ansi\ansicpg1251 \pard x\par}`))
	if err == nil {
		t.Fatal("cp1251 wurde ohne Fehler gelesen")
	}
	if !strings.Contains(err.Error(), "1251") {
		t.Fatalf("die Meldung nennt die Code page nicht: %v", err)
	}
	if _, err := Convert([]byte(`{\rtf1\ansi\ansicpg65001 \pard x\par}`)); err != nil {
		t.Fatalf("UTF-8 ist nicht unterstuetzt: %v", err)
	}
}
