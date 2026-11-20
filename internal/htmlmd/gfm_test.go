package htmlmd

import (
	"strings"
	"testing"
)

func TestConvertGFMWritesASeparator(t *testing.T) {
	got := ConvertGFM("<table><thead><tr><th>A</th><th>B</th></tr></thead><tbody><tr><td>1</td><td>2</td></tr></tbody></table>")
	want := "A | B\n--- | ---\n1 | 2\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if plain := Convert("<table><tr><td>a</td><td>b</td></tr></table>"); plain != "a | b\n" {
		t.Errorf("plain form changed: %q", plain)
	}
}

// TestGFMTableSurvivesTableSections: a table whose rows sit in thead and tbody
// must not get a blank line in the middle - that ends the table for every
// Markdown reader, and the document loses its head row.
func TestGFMTableSurvivesTableSections(t *testing.T) {
	html := "<table>\n<thead>\n<tr><th>A</th><th>B</th></tr>\n</thead>\n<tbody>\n<tr><td>1</td><td>2</td></tr>\n<tr><td>3</td><td>4</td></tr>\n</tbody>\n</table>"
	got := ConvertGFM(html)
	want := "A | B\n--- | ---\n1 | 2\n3 | 4\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestListItemsStayTogether: whitespace between items is one newline, not a
// paragraph break.
func TestListItemsStayTogether(t *testing.T) {
	got := ConvertGFM("<ul>\n<li>eins</li>\n\n<li>zwei</li>\n</ul>")
	if strings.Contains(got, "\n\n") {
		t.Errorf("a blank line broke the list: %q", got)
	}
}
