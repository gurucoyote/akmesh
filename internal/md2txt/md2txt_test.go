package md2txt

import (
	"strings"
	"testing"
)

// The expectations below are the output of pandoc 3.x for the same input
// ("pandoc -t plain"), measured on 19.11.2026. That is what an agent means when
// it asks for plain text.

func TestHeadingsLoseTheirMarks(t *testing.T) {
	got := Convert("# Titel Eins\n\nText.\n\n## Zweiter Punkt\n\nMehr.\n")
	want := "Titel Eins\n\nText.\n\nZweiter Punkt\n\nMehr.\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestEmphasisAndCodeLoseTheirMarks(t *testing.T) {
	got := Convert("Ein Absatz mit **fett**, *kursiv* und `code`.\n")
	want := "Ein Absatz mit fett, kursiv und code.\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// A link keeps its text only: pandoc drops the target in plain text, because a
// plain text document has nowhere to point.
func TestLinksKeepTheTextOnly(t *testing.T) {
	got := Convert("Ein [Link](https://example.org) und ein <https://auto.link>.\n")
	want := "Ein Link und ein https://auto.link.\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// An image has no text of its own, so its description stands in for it.
func TestImagesBecomeTheirDescription(t *testing.T) {
	got := Convert("Ein Bild: ![Bildtext](bild.png).\n")
	want := "Ein Bild: [Bildtext].\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestListsAreIndentedByTwo(t *testing.T) {
	got := Convert("- a\n  - a1\n  - a2\n- b\n")
	want := "- a\n  - a1\n  - a2\n- b\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestOrderedListHasTwoSpacesAfterTheNumber(t *testing.T) {
	got := Convert("1. eins\n2. zwei\n")
	want := "1.  eins\n2.  zwei\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestBlockquoteIsIndentedByTwo(t *testing.T) {
	got := Convert("> Ein Zitat ueber zwei\n> Zeilen.\n")
	want := "  Ein Zitat ueber zwei Zeilen.\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestCodeBlockIsIndentedByFour(t *testing.T) {
	got := Convert("```go\nfmt.Println(\"hallo\")\n```\n")
	want := "    fmt.Println(\"hallo\")\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// A table keeps its shape: pandoc writes the columns, then a line of dashes
// under the first row, with the columns padded to their width.
func TestTableKeepsItsColumns(t *testing.T) {
	md := "| Name | Ort |\n| --- | --- |\n| Anna | Bremen |\n"
	want := "  Name   Ort\n  ------ --------\n  Anna   Bremen\n"
	if got := Convert(md); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestHorizontalRuleIsSeventyTwoDashes(t *testing.T) {
	want := strings.Repeat("-", 72) + "\n"
	if got := Convert("Absatz.\n\n---\n"); !strings.HasSuffix(got, want) {
		t.Errorf("got %q want suffix %q", got, want)
	}
}

// A paragraph is wrapped at 72 characters, because that is what pandoc -t plain
// does with --wrap=auto, and an agent comparing the two must not see a
// difference.
func TestParagraphIsWrappedAt72(t *testing.T) {
	long := "Ein langer Absatz, der deutlich mehr als zweiundsiebzig Zeichen enthaelt und deshalb umgebrochen werden muss, wenn pandoc ihn als reinen Text ausgibt, was hier geprueft wird."
	got := Convert(long + "\n")
	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if len([]rune(line)) > 72 {
			t.Errorf("Zeile laenger als 72 Zeichen (%d): %q", len([]rune(line)), line)
		}
	}
	if strings.Count(got, "\n") < 2 {
		t.Errorf("Absatz wurde nicht umbrochen: %q", got)
	}
}

// A hard break survives: two spaces at the end of a line mean a line break in
// the text.
func TestHardBreakStaysABreak(t *testing.T) {
	got := Convert("Text mit hartem Umbruch am Zeilenende  \nund der naechsten Zeile.\n")
	want := "Text mit hartem Umbruch am Zeilenende\nund der naechsten Zeile.\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// Escaped characters lose their backslash, because there is nothing left to
// escape in plain text.
func TestEscapesAreResolved(t *testing.T) {
	got := Convert("Fuenf \\* drei ist 5\\*3.\n")
	want := "Fuenf * drei ist 5*3.\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// A definition list puts the term on its own line and indents the definition by
// four spaces.
func TestDefinitionListIsIndented(t *testing.T) {
	got := Convert("Term\n: Definition einer Definitionsliste.\n")
	want := "Term\n    Definition einer Definitionsliste.\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// An empty document is empty, not a run of blank lines.
func TestEmptyInput(t *testing.T) {
	if got := Convert(""); got != "" {
		t.Errorf("got %q want empty", got)
	}
}
