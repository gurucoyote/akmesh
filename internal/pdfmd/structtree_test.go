package pdfmd

import (
	"os"
	"strings"
	"testing"
)

// TestValueReadsAName is the regression for the bug that made the whole
// structure tree unreadable: value() returned "" for every name value, so a
// role "/S /Document" read as an empty string.
func TestValueReadsAName(t *testing.T) {
	dict := `<</Type /StructElem /S /Document /P 18 0 R /K [20 0 R]>>`
	if got := value(dict, "S"); got != "/Document" {
		t.Errorf("S = %q, want /Document", got)
	}
	if got := value(dict, "Type"); got != "/StructElem" {
		t.Errorf("Type = %q, want /StructElem", got)
	}
	// The key has to end where a name ends: /S is also the front of /StructElem.
	if got := value(`<</StructTreeRoot 8 0 R /S /P>>`, "S"); got != "/P" {
		t.Errorf("S = %q, want /P", got)
	}
}

// TestArrayItemsKeepsReferencesWhole: a reference is three tokens, and tearing
// it apart turned the number of one object into a marked content id.
func TestArrayItemsKeepsReferencesWhole(t *testing.T) {
	got := arrayItems("[27 0 R 28 0 R 4 <</MCID 2>>]")
	want := []string{"27 0 R", "28 0 R", "4", "<</MCID 2>>"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// structBook builds the tree of a small book by hand and the text its blocks
// carry, so the rendering is tested without a tagged file to read.
func structBook() (*structElem, structText) {
	page := "3 0"
	text := structText{
		pages:  []string{page},
		byPage: map[string]map[int]string{page: {0: "Kapitel eins", 1: "Der erste Absatz.", 2: "Erster Punkt", 3: "Zweiter Punkt", 4: "Name", 5: "Wert", 6: "Alpha", 7: "1"}},
	}
	doc := &structElem{role: "/Document", kids: []any{
		&structElem{role: "/H1", kids: []any{mcr{page: page, mcid: 0}}},
		&structElem{role: "/P", kids: []any{mcr{page: page, mcid: 1}}},
		&structElem{role: "/L", kids: []any{
			&structElem{role: "/LI", kids: []any{
				&structElem{role: "/Lbl", kids: []any{mcr{page: page, mcid: 2}}},
			}},
			&structElem{role: "/LI", kids: []any{
				&structElem{role: "/Lbl", kids: []any{mcr{page: page, mcid: 3}}},
			}},
		}},
		&structElem{role: "/Table", kids: []any{
			&structElem{role: "/TR", kids: []any{
				&structElem{role: "/TH", kids: []any{mcr{page: page, mcid: 4}}},
				&structElem{role: "/TH", kids: []any{mcr{page: page, mcid: 5}}},
			}},
			&structElem{role: "/TR", kids: []any{
				&structElem{role: "/TD", kids: []any{mcr{page: page, mcid: 6}}},
				&structElem{role: "/TD", kids: []any{mcr{page: page, mcid: 7}}},
			}},
		}},
	}}
	return doc, text
}

// TestTreeRendersStructure: the point of the tree is that a heading comes back
// as a heading and a list as a list, without guessing from font sizes.
func TestTreeRendersStructure(t *testing.T) {
	doc, text := structBook()
	got := doc.toMarkdown(text, "")
	for _, want := range []string{
		"# Kapitel eins",
		"Der erste Absatz.",
		"- Erster Punkt",
		"- Zweiter Punkt",
		"| Name | Wert |",
		"| Alpha | 1 |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	// The heading must not also come out as a plain paragraph.
	if strings.Count(got, "Kapitel eins") != 1 {
		t.Errorf("the heading is printed more than once:\n%s", got)
	}
}

// TestTreeWithoutTextFallsBack: a tree that carries no text is no tree, and the
// caller has to read the pages instead.
func TestTreeWithoutTextFallsBack(t *testing.T) {
	doc, _ := structBook()
	empty := structText{pages: []string{"3 0"}, byPage: map[string]map[int]string{"3 0": {}}}
	if got := doc.toMarkdown(empty, ""); strings.TrimSpace(got) != "" {
		t.Errorf("an empty tree produced %q, want nothing", got)
	}
}

// TestPageLessBlockUsesTheFirstPage: a writer may leave /Pg off an element; the
// block then belongs to the first page that carries it.
func TestPageLessBlockUsesTheFirstPage(t *testing.T) {
	text := structText{
		pages:  []string{"3 0"},
		byPage: map[string]map[int]string{"3 0": {5: "ohne Seite"}},
	}
	if got := text.get("", 5); got != "ohne Seite" {
		t.Errorf("got %q", got)
	}
	if got := text.get("", 9); got != "" {
		t.Errorf("a missing block gave %q", got)
	}
}

// TestPunctuationSticksToTheWord: a period is its own block in some documents,
// and joining blocks with a space put it on its own.
func TestPunctuationSticksToTheWord(t *testing.T) {
	page := "3 0"
	text := structText{
		pages:  []string{page},
		byPage: map[string]map[int]string{page: {1: "Ein Satz", 2: "."}},
	}
	p := &structElem{role: "/P", kids: []any{mcr{page: page, mcid: 1}, mcr{page: page, mcid: 2}}}
	if got := p.ownText(text, ""); got != "Ein Satz." {
		t.Errorf("got %q, want %q", got, "Ein Satz.")
	}
}

// TestTaggedFileKeepsItsStructure is the end to end case: a document we wrote
// ourselves, with a structure tree, read back. The file is our own output
// (joshua's html2pdf, 2026-11-19), so nothing foreign is committed - and a
// reader that cannot read what this house writes is not finished.
func TestTaggedFileKeepsItsStructure(t *testing.T) {
	data, err := os.ReadFile("testdata/tagged.pdf")
	if err != nil {
		t.Skipf("no fixture: %v", err)
	}
	md, err := Convert(data)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	for _, want := range []string{
		"# Kapitel eins",
		"## Unterkapitel",
		"Der erste Absatz steht unter der Ueberschrift.",
		"- Erster Punkt",
		"- Zweiter Punkt",
		"| Name | Wert |",
		"| Alpha | 1 |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("the read back document lacks %q:\n%s", want, md)
		}
	}
	// Without the tree the heading level comes from a font size guess; with it,
	// the second heading is a level two and not a level one.
	if strings.Contains(md, "\n# Unterkapitel") {
		t.Errorf("the heading level was guessed, not read:\n%s", md)
	}
}
