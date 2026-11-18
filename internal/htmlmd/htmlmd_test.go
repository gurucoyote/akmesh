package htmlmd

import (
	"strings"
	"testing"
)

// TestHeadingsKeepTheirLevel is the defect the rekr converter has: every level
// became ### and the marker stood alone on its line.
func TestHeadingsKeepTheirLevel(t *testing.T) {
	cases := map[string]string{
		"<h1>Titel</h1>":           "# Titel\n",
		"<h2>Zwei</h2>":            "## Zwei\n",
		"<h4>Vier</h4>":            "#### Vier\n",
		"<h2>Zwei</h2><p>Text</p>": "## Zwei\n\nText\n",
		"<p>vor</p><h3>Drei</h3>":  "vor\n\n### Drei\n",
	}
	for in, want := range cases {
		if got := Convert(in); got != want {
			t.Errorf("Convert(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}

// TestInlineMarkup maps the elements an agent expects to see as Markdown.
func TestInlineMarkup(t *testing.T) {
	cases := map[string]string{
		"<p><b>fett</b> und <i>kursiv</i></p>": "**fett** und *kursiv*\n",
		"<p><strong>a</strong> <em>b</em></p>": "**a** *b*\n",
		"<p><code>x := 1</code></p>":           "`x := 1`\n",
		"<p><del>weg</del></p>":                "~~weg~~\n",
	}
	for in, want := range cases {
		if got := Convert(in); got != want {
			t.Errorf("Convert(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}

// TestLinksAndImages covers the two things a page carries most often.
func TestLinksAndImages(t *testing.T) {
	got := Convert(`<p>ein <a href="http://x.tld/a b">Link</a></p>`)
	if got != "ein [Link](http://x.tld/a%20b)\n" {
		t.Errorf("link: %q", got)
	}
	got = Convert(`<p><img src="bild.png" alt="Ein Bild"></p>`)
	if got != "![Ein Bild](bild.png)\n" {
		t.Errorf("img: %q", got)
	}
	// An anchor without a target keeps its text and gets no brackets.
	got = Convert(`<p><a name="x">Anker</a></p>`)
	if got != "Anker\n" {
		t.Errorf("anchor without href: %q", got)
	}
}

// TestLists covers unordered, ordered and nested lists.
func TestLists(t *testing.T) {
	got := Convert("<ul><li>eins</li><li>zwei</li></ul>")
	if got != "- eins\n- zwei\n" {
		t.Errorf("ul: %q", got)
	}
	got = Convert("<ol><li>erster</li><li>zweiter</li></ol>")
	if got != "1. erster\n1. zweiter\n" {
		t.Errorf("ol: %q", got)
	}
	got = Convert("<ul><li>oben<ul><li>unten</li></ul></li></ul>")
	if !strings.Contains(got, "- oben") || !strings.Contains(got, "  - unten") {
		t.Errorf("nested: %q", got)
	}
}

// TestBlockquoteAndRule pins the two markers that survive markdown cleanly.
func TestBlockquoteAndRule(t *testing.T) {
	got := Convert("<blockquote><p>Zitat</p></blockquote>")
	if got != "> Zitat\n" {
		t.Errorf("blockquote: %q", got)
	}
	got = Convert("<p>a</p><hr><p>b</p>")
	if got != "a\n\n---\n\nb\n" {
		t.Errorf("hr: %q", got)
	}
}

// TestPreKeepsCodeVerbatim: inside a code block nothing may be escaped or
// reformatted, or the sample stops being the sample.
func TestPreKeepsCodeVerbatim(t *testing.T) {
	got := Convert("<pre>if x &lt; 3 {\n\tp(\"a*b\")\n}</pre>")
	want := "```\nif x < 3 {\n\tp(\"a*b\")\n}\n```\n"
	if got != want {
		t.Errorf("pre:\n got %q\nwant %q", got, want)
	}
}

// TestEscapingKeepsTheWording is the rule the whole converter exists for: text
// that looked like markup must still look like text.
func TestEscapingKeepsTheWording(t *testing.T) {
	cases := map[string]string{
		"<p>5*3 und 2_1</p>": "5\\*3 und 2\\_1\n",
		"<p>[kein Link]</p>": "\\[kein Link\\]\n",
		"<p>a | b</p>":       "a \\| b\n",
		"<p>1. Punkt</p>":    "1\\. Punkt\n",
	}
	for in, want := range cases {
		if got := Convert(in); got != want {
			t.Errorf("Convert(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}

// TestEntitiesAndBreaks: an entity is text, and br is a hard line break.
func TestEntitiesAndBreaks(t *testing.T) {
	if got := Convert("<p>a &amp; b</p>"); got != "a & b\n" {
		t.Errorf("entity: %q", got)
	}
	if got := Convert("<p>eins<br>zwei</p>"); got != "eins  \nzwei\n" {
		t.Errorf("br: %q", got)
	}
}

// TestTablesKeepTheirCells: a table has no clean Markdown form here, so the
// cells are joined with a pipe and the row ends the line. That keeps the text
// and is readable, without claiming to be a GFM table.
func TestTablesKeepTheirCells(t *testing.T) {
	got := Convert("<table><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>")
	if !strings.Contains(got, "a | b") || !strings.Contains(got, "c | d") {
		t.Errorf("table: %q", got)
	}
}

// TestUnknownElementsKeepText: dropping a tag must never drop its words.
func TestUnknownElementsKeepText(t *testing.T) {
	got := Convert("<p>vor <custom-el>drin</custom-el> nach</p>")
	if !strings.Contains(got, "drin") {
		t.Errorf("text lost: %q", got)
	}
	if strings.Contains(got, "custom-el") {
		t.Errorf("tag kept: %q", got)
	}
}

// TestEmpty: nothing in, nothing out.
func TestEmpty(t *testing.T) {
	if got := Convert(""); got != "" {
		t.Errorf("empty: %q", got)
	}
	if got := Convert("<p></p>"); got != "" {
		t.Errorf("empty paragraph: %q", got)
	}
}

// TestUnterminatedTagIsText: a "<" that starts no tag is a character, not a
// reason to swallow the rest of the page.
func TestUnterminatedTagIsText(t *testing.T) {
	got := Convert("<p>a < b</p>")
	if !strings.Contains(got, "a < b") {
		t.Errorf("got %q", got)
	}
}
