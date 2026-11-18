package rtfmd

import (
	"strings"
	"testing"
)

// rtf wraps a body in the smallest header a reader would accept, with the code
// page a Mac or Windows word processor writes.
func rtf(body string) []byte {
	return []byte(`{\rtf1\ansi\ansicpg1252\deff0` + body + `}`)
}

func mustConvert(t *testing.T, data []byte) string {
	t.Helper()
	md, err := Convert(data)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	return md
}

// TestParagraphBreaks: in RTF a paragraph is \par, and a backslash followed by
// a newline means the same thing. Both have to end the paragraph, or the whole
// document arrives as one line.
func TestParagraphBreaks(t *testing.T) {
	got := mustConvert(t, rtf(`\pard eins\par zwei\par`))
	if got != "eins\n\nzwei\n" {
		t.Errorf("got %q", got)
	}
	got = mustConvert(t, rtf("eins\\\nzwei\\\n"))
	if got != "eins\n\nzwei\n" {
		t.Errorf("backslash-newline: %q", got)
	}
}

// TestLineBreakAndTab: \line is a break inside the paragraph, \tab a tab.
func TestLineBreakAndTab(t *testing.T) {
	got := mustConvert(t, rtf(`\pard eins\line zwei\tab drei\par`))
	if got != "eins  \nzwei\t drei\n" && got != "eins  \nzwei\tdrei\n" {
		t.Errorf("got %q", got)
	}
}

// TestEscapedBracesAndBackslash: \\ \{ \} are characters, not markup.
func TestEscapedBracesAndBackslash(t *testing.T) {
	got := mustConvert(t, rtf(`\pard a\\b \{c\} d\par`))
	if got != "a\\\\b {c} d\n" && got != `a\b {c} d`+"\n" {
		t.Errorf("got %q", got)
	}
}

// TestCodePageBytes: \'hh is one byte in the document's code page, and 0x94 is
// the right double quote, not four characters.
func TestCodePageBytes(t *testing.T) {
	got := mustConvert(t, rtf(`\pard \'93Hallo\'94 \'84deutsch\'93 \'e4\'f6\'fc\par`))
	if got != "“Hallo” „deutsch“ äöü\n" {
		t.Errorf("got %q", got)
	}
}

// TestUnicodeEscape: \u228 is the character itself, and the fallback that
// follows (\uc1) must not be printed a second time.
func TestUnicodeEscape(t *testing.T) {
	got := mustConvert(t, rtf(`\pard \uc1\u228\'e4 und \uc2\u8364??\par`))
	if got != "ä und €\n" {
		t.Errorf("got %q", got)
	}
}

// TestFormatting: bold and italic become their markers, and a run that ends
// closes them.
func TestFormatting(t *testing.T) {
	got := mustConvert(t, rtf(`\pard vor \b fett\b0  und \i kursiv\i0\par`))
	if got != "vor **fett** und *kursiv*\n" {
		t.Errorf("got %q", got)
	}
}

// TestSkippedDestinations: the font table, the colour table and the document
// info carry no text for the reader; printing "ArialMT" would be noise in every
// converted document.
func TestSkippedDestinations(t *testing.T) {
	body := `{\fonttbl\f0\fswiss ArialMT;}` +
		`{\colortbl;\red255\green0\blue0;}` +
		`{\info{\title Geheim}{\author Martin}}` +
		`{\*\generator Mac OS X 10.6;}` +
		`\pard Der Text\par`
	got := mustConvert(t, rtf(body))
	if got != "Der Text\n" {
		t.Errorf("got %q", got)
	}
	if strings.Contains(got, "ArialMT") || strings.Contains(got, "Geheim") {
		t.Errorf("table or info leaked into the text: %q", got)
	}
}

// TestHeadingFromTheStylesheet: Word writes "\s1" on a heading paragraph and the
// name of that style is in the stylesheet, so the level has to come from there.
func TestHeadingFromTheStylesheet(t *testing.T) {
	body := `{\stylesheet{\s1\b Heading 1;}{\s2 Heading 2;}}` +
		`\pard\s1 Erste Ebene\par \pard\s2 Zweite Stufe\par \pard Fliesstext\par`
	got := mustConvert(t, rtf(body))
	want := "# Erste Ebene\n\n## Zweite Stufe\n\nFliesstext\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// TestBulletList: a bullet paragraph is a list item, and consecutive items stay
// in one list.
func TestBulletList(t *testing.T) {
	body := `\pard{\pntext\f0 \'b7\tab}{\*\pn\pnlvlblt\pnf0{\pnindent0{\pntxtb\'b7}}}eins\par` +
		`\pard{\pntext\f0 \'b7\tab}{\*\pn\pnlvlblt\pnf0{\pnindent0{\pntxtb\'b7}}}zwei\par`
	got := mustConvert(t, rtf(body))
	if got != "- eins\n- zwei\n" {
		t.Errorf("got %q", got)
	}
}

// TestNotRtfIsAnError: an empty result must not look like an empty document.
func TestNotRtfIsAnError(t *testing.T) {
	if _, err := Convert([]byte("das ist kein rtf")); err == nil {
		t.Errorf("no error for text that is not RTF")
	}
}

// TestMarkdownCharactersAreEscaped: a document about "*" stays a document about
// "*".
func TestMarkdownCharactersAreEscaped(t *testing.T) {
	got := mustConvert(t, rtf(`\pard 5*3 und [x]\par`))
	if !strings.Contains(got, `5\*3`) || !strings.Contains(got, `\[x\]`) {
		t.Errorf("got %q", got)
	}
}
