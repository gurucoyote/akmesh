// Markdown over the blocks: the second writer of this package. Parse does the
// reading (parse.go), this file only formats.
package rtfmd

import (
	"strconv"
	"strings"

	"github.com/gurucoyote/akmesh/internal/htmlmd"
)

// Convert reads RTF and returns the Markdown for it.
func Convert(data []byte) (string, error) {
	blocks, err := Parse(data)
	if err != nil {
		return "", err
	}
	return Render(blocks), nil
}

// Render writes blocks as Markdown. Tables become pipe tables, links become
// [text](url), list items are indented by their level.
func Render(blocks []Block) string {
	w := &writer{}
	for _, b := range blocks {
		w.block(b)
	}
	return w.finish()
}

// writer assembles the Markdown.
type writer struct {
	out     strings.Builder
	atStart bool

	inList   bool
	listKind ListKind
	listNo   int
}

func (w *writer) block(b Block) {
	switch b.Kind {
	case Table:
		w.endList()
		w.blank()
		w.table(b)
		w.atStart = true
	case ListItem:
		w.startList(b.List)
		indent := ""
		if b.Level > 1 {
			indent = strings.Repeat("  ", b.Level-1)
		}
		body := w.runs(b.Runs)
		if b.List == Numbered {
			w.listNo++
			body = indent + strconv.Itoa(w.listNo) + ". " + body
		} else {
			body = indent + "- " + body
		}
		w.out.WriteString(body + "\n")
		w.atStart = false
	case Heading:
		w.endList()
		w.blank()
		w.out.WriteString(strings.Repeat("#", b.Level) + " " + w.runs(b.Runs) + "\n")
		w.atStart = false
	default:
		w.endList()
		w.blank()
		w.out.WriteString(w.runs(b.Runs) + "\n")
		w.atStart = false
	}
}

// runs writes the text of one block with its markers.
func (w *writer) runs(runs []Run) string {
	runs = trimRuns(runs)
	var b strings.Builder
	for _, r := range runs {
		text := w.inline(r.Text, b.Len() == 0 && w.atStart)
		switch {
		case r.Bold && r.Italic:
			text = "***" + text + "***"
		case r.Bold:
			text = "**" + text + "**"
		case r.Italic:
			text = "*" + text + "*"
		}
		if r.Link != "" {
			text = "[" + text + "](" + linkTarget(r.Link) + ")"
		}
		b.WriteString(text)
	}
	return b.String()
}

// inline protects the text of a run. A newline inside a run is a break inside
// the paragraph (\line), which Markdown writes as two spaces and a newline.
func (w *writer) inline(text string, atLineStart bool) string {
	parts := strings.Split(text, "\n")
	for i, part := range parts {
		leading := atLineStart && i == 0 || i > 0
		parts[i] = escape(part, leading)
	}
	return strings.Join(parts, "  \n")
}

// table writes a pipe table. Markdown has no table without a header row, so the
// first row becomes one - the column count comes from the widest row.
func (w *writer) table(b Block) {
	if len(b.Rows) == 0 {
		return
	}
	cols := 0
	for _, row := range b.Rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return
	}
	line := func(row [][]Run) {
		w.out.WriteString("|")
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = cellText(w.runs(row[i]))
			}
			w.out.WriteString(" " + cell + " |")
		}
		w.out.WriteString("\n")
	}
	line(b.Rows[0])
	w.out.WriteString("|")
	for i := 0; i < cols; i++ {
		w.out.WriteString(" --- |")
	}
	w.out.WriteString("\n")
	for _, row := range b.Rows[1:] {
		line(row)
	}
}

// cellText keeps a table cell on one line: a pipe would end the cell, a newline
// would end the row.
func cellText(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// linkTarget writes a link address so that it survives the parenthesis syntax:
// an address with a space or a bracket goes into angle brackets.
func linkTarget(url string) string {
	if strings.ContainsAny(url, " ()<>") {
		return "<" + strings.NewReplacer("<", "%3C", ">", "%3E").Replace(url) + ">"
	}
	return url
}

// startList breaks the paragraph before the first item of a list.
func (w *writer) startList(kind ListKind) {
	if w.inList {
		// A bullet after a numbered item is the same list in the document, so
		// it must not start a new block here; only the marker changes.
		w.listKind = kind
		return
	}
	w.endList()
	w.blank()
	w.inList, w.listKind, w.listNo = true, kind, 0
}

// endList forgets the list state, so the next paragraph starts fresh.
func (w *writer) endList() {
	w.inList = false
	w.listKind = NoList
	w.listNo = 0
}

// escape protects text that is about to be written as Markdown. The rules for
// the start of a line only apply there, or a date like "11. Juni" would be
// escaped in the middle of a sentence.
func escape(text string, atLineStart bool) string {
	if atLineStart {
		return htmlmd.Escape(text)
	}
	return htmlmd.EscapeMidLine(text)
}

// blank ends the previous line before a new block starts.
func (w *writer) blank() {
	s := w.out.String()
	if s == "" || strings.HasSuffix(s, "\n\n") {
		w.atStart = true
		return
	}
	if !strings.HasSuffix(s, "\n") {
		w.out.WriteString("\n")
	}
	w.out.WriteString("\n")
	w.atStart = true
}

// finish tidies the result.
func (w *writer) finish() string {
	s := strings.TrimLeft(w.out.String(), "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	return s + "\n"
}
