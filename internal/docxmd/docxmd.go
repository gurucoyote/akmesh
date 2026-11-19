// Package docxmd turns a .docx into Markdown.
//
// A docx is a zip with word/document.xml inside. The text is in w:t elements,
// but everything that makes the document readable is in attributes and side
// parts: the heading level is a paragraph style, the list shape is a numId that
// points into word/numbering.xml, the address of a link is in
// word/_rels/document.xml.rels, and formatting lives on the run. So the reader
// walks the element tree once and takes those four lookups with it.
//
// What comes out is what an agent expects from "docx to markdown": "#" for a
// heading, ** and * for bold and italic, "- " and "1. " for lists, a GFM table
// for a table, [text](url) for a link, and blank lines between paragraphs.
// Text that looks like markup is escaped, so a document about "*" stays a
// document about "*".
package docxmd

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"akmesh/internal/htmlmd"
)

// Convert reads a docx and returns the Markdown for it.
func Convert(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("not a docx (no zip): %w", err)
	}
	document, err := part(zr, "word/document.xml")
	if err != nil {
		return "", err
	}
	numbering, _ := part(zr, "word/numbering.xml") // optional
	rels, _ := part(zr, "word/_rels/document.xml.rels")

	root, err := parse(document)
	if err != nil {
		return "", fmt.Errorf("word/document.xml: %w", err)
	}
	body := root.find("body")
	if body == nil {
		return "", fmt.Errorf("word/document.xml has no w:body")
	}

	w := &writer{
		numFmt:      readNumbering(numbering),
		rel:         readRels(rels),
		counts:      map[string]int{},
		atLineStart: true,
	}
	w.blocks(body)
	return w.finish(), nil
}

// part returns one member of the zip.
func part(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("%s is missing", name)
}

// node is one XML element with the attributes and text the reader needs. A tree
// of these is enough for a docx: nothing in it depends on order-sensitive
// decoding.
type node struct {
	name     string
	attrs    map[string]string
	text     string
	children []*node
}

// parse builds the element tree of one XML part.
func parse(data []byte) (*node, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var root *node
	var stack []*node
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			if len(stack) == 0 {
				root = n
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(t)
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("no element found")
	}
	return root, nil
}

// find returns the first element with that local name, depth first.
func (n *node) find(name string) *node {
	if n.name == name {
		return n
	}
	for _, c := range n.children {
		if f := c.find(name); f != nil {
			return f
		}
	}
	return nil
}

// kids returns the children with that local name, in order.
func (n *node) kids(name string) []*node {
	var out []*node
	for _, c := range n.children {
		if c.name == name {
			out = append(out, c)
		}
	}
	return out
}

// readNumbering maps numId to the list shape per level: "bullet", "decimal" or
// the format Word wrote. Nothing in the document part says whether an item is a
// bullet or a number; only this part does.
func readNumbering(data []byte) map[string]map[string]string {
	out := map[string]map[string]string{}
	if len(data) == 0 {
		return out
	}
	root, err := parse(data)
	if err != nil {
		return out
	}
	abstract := map[string]map[string]string{}
	for _, a := range root.kids("abstractNum") {
		id := a.attrs["abstractNumId"]
		levels := map[string]string{}
		for _, l := range a.kids("lvl") {
			fmtNode := l.find("numFmt")
			if fmtNode != nil {
				levels[l.attrs["ilvl"]] = fmtNode.attrs["val"]
			}
		}
		abstract[id] = levels
	}
	for _, n := range root.kids("num") {
		ref := n.find("abstractNumId")
		if ref == nil {
			continue
		}
		out[n.attrs["numId"]] = abstract[ref.attrs["val"]]
	}
	return out
}

// readRels maps a relationship id to its target, which is how a hyperlink gets
// its address.
func readRels(data []byte) map[string]string {
	out := map[string]string{}
	if len(data) == 0 {
		return out
	}
	root, err := parse(data)
	if err != nil {
		return out
	}
	for _, r := range root.kids("Relationship") {
		out[r.attrs["Id"]] = r.attrs["Target"]
	}
	return out
}

// writer collects the Markdown.
type writer struct {
	out         strings.Builder
	numFmt      map[string]map[string]string
	rel         map[string]string
	counts      map[string]int // "numId/ilvl" to the last number written
	inList      bool           // the previous block was a list item
	atLineStart bool           // the next text starts a line, so the line rules apply
}

// blocks writes the block level children of one element: paragraphs and tables.
func (w *writer) blocks(n *node) {
	for _, c := range n.children {
		switch c.name {
		case "p":
			w.paragraph(c)
		case "tbl":
			w.table(c)
		case "sdt", "sdtContent", "txbxContent":
			// Content controls and text boxes wrap paragraphs; the text is in
			// there and a reader wants it.
			w.blocks(c)
		}
	}
}

// paragraph writes one w:p.
func (w *writer) paragraph(p *node) {
	if ppr := p.find("pPr"); ppr != nil {
		if numPr := ppr.find("numPr"); numPr != nil {
			w.listItem(p, ppr, numPr)
			return
		}
		if style := styleName(ppr); style != "" {
			if level := headingLevel(style); level > 0 {
				text := strings.TrimSpace(w.inline(p))
				if text == "" {
					return
				}
				w.blank()
				w.out.WriteString(strings.Repeat("#", level) + " " + text + "\n")
				return
			}
		}
	}
	// The line has to be broken before the text is rendered: whether the
	// escaping rules for a line start apply depends on where the text lands.
	w.blank()
	w.inList = false
	text := strings.TrimSpace(w.inline(p))
	if text == "" {
		return
	}
	w.out.WriteString(text + "\n")
	w.atLineStart = true
}

// listItem writes one paragraph that carries a numPr.
func (w *writer) listItem(p, ppr, numPr *node) {
	numId := attr(numPr.find("numId"), "val")
	level := attr(numPr.find("ilvl"), "val")
	if level == "" && ppr.find("ilvl") != nil {
		level = attr(ppr.find("ilvl"), "val")
	}
	lvl, _ := strconv.Atoi(level)

	// Items of one list follow each other line by line; a blank line between
	// them would turn the list into separate lists in Markdown.
	if !w.inList {
		w.blank()
	}
	w.inList = true
	text := strings.TrimSpace(w.inline(p))
	if text == "" {
		return
	}
	indent := strings.Repeat("  ", lvl)
	marker := "- "
	format := w.numFmt[numId][level]
	if format != "" && format != "bullet" && format != "none" {
		key := numId + "/" + level
		w.counts[key]++
		marker = strconv.Itoa(w.counts[key]) + ". "
	}
	w.out.WriteString(indent + marker + text + "\n")
	w.atLineStart = true
}

// table writes one w:tbl as a GFM table: the first row is the header, because
// that is what a Markdown table can hold.
func (w *writer) table(tbl *node) {
	var rows [][]string
	for _, tr := range tbl.kids("tr") {
		var cells []string
		for _, tc := range tr.kids("tc") {
			// A cell holds paragraphs, not runs: its text is the text of
			// those paragraphs joined by a space.
			var parts []string
			for _, p := range tc.kids("p") {
				if t := strings.TrimSpace(w.inline(p)); t != "" {
					parts = append(parts, t)
				}
			}
			if len(parts) == 0 {
				if t := strings.TrimSpace(w.inline(tc)); t != "" {
					parts = append(parts, t)
				}
			}
			cell := strings.Join(parts, " ")
			// A pipe inside a cell would split it into two columns.
			cell = strings.ReplaceAll(cell, "|", "\\|")
			cells = append(cells, cell)
		}
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}
	if len(rows) == 0 {
		return
	}
	w.inList = false
	w.blank()
	width := 0
	for _, r := range rows {
		if len(r) > width {
			width = len(r)
		}
	}
	writeRow := func(r []string) {
		w.out.WriteString("|")
		for i := 0; i < width; i++ {
			cell := ""
			if i < len(r) {
				cell = r[i]
			}
			w.out.WriteString(" " + cell + " |")
		}
		w.out.WriteString("\n")
	}
	writeRow(rows[0])
	w.out.WriteString("|")
	for i := 0; i < width; i++ {
		w.out.WriteString(" --- |")
	}
	w.out.WriteString("\n")
	for _, r := range rows[1:] {
		writeRow(r)
	}
}

// seg is a piece of one line together with its formatting. Word splits a
// paragraph into runs on its own terms (spelling, language, revisions), so the
// pieces have to be merged before they are written: two bold runs next to each
// other are one bold phrase, not "**a ****b**".
type seg struct {
	text   string
	bold   bool
	italic bool
	strike bool
	raw    bool // already Markdown: a link, not text
	br     bool // a line break after this piece
}

// inline renders the runs of one element.
func (w *writer) inline(n *node) string {
	return w.render(w.segs(n))
}

// segs collects the pieces of one element in order.
func (w *writer) segs(n *node) []seg {
	var out []seg
	for _, c := range n.children {
		switch c.name {
		case "r":
			out = append(out, w.runSeg(c)...)
		case "br":
			// A break as a sibling of the runs is how Word writes a new line
			// inside a paragraph.
			out = append(out, seg{br: true})
		case "hyperlink":
			target := w.rel[c.attrs["id"]]
			inner := w.render(w.segs(c))
			if target == "" || inner == "" {
				out = append(out, seg{text: inner, raw: true})
				continue
			}
			link := "[" + inner + "](" + strings.ReplaceAll(target, " ", "%20") + ")"
			out = append(out, seg{text: link, raw: true})
		case "ins", "smartTag", "sdt", "sdtContent":
			// Wrappers and tracked insertions: their text belongs to the
			// document as it stands.
			out = append(out, w.segs(c)...)
		case "del", "moveFrom":
			// Deleted text is not part of the document any more.
		}
	}
	return out
}

// runSeg turns one w:r into a piece. A run that carries more than text (a tab,
// a break) becomes more than one piece so that the markers wrap the words only.
func (w *writer) runSeg(r *node) []seg {
	bold, italic, strike := false, false, false
	if rpr := r.find("rPr"); rpr != nil {
		bold = on(rpr.find("b"))
		italic = on(rpr.find("i"))
		strike = on(rpr.find("strike"))
	}
	var out []seg
	pending := ""
	flush := func() {
		if pending != "" {
			out = append(out, seg{text: pending, bold: bold, italic: italic, strike: strike})
			pending = ""
		}
	}
	for _, c := range r.children {
		switch c.name {
		case "t":
			pending += c.text
		case "tab":
			pending += "\t"
		case "br", "cr":
			flush()
			out = append(out, seg{br: true})
		}
	}
	flush()
	return out
}

// render writes pieces with their markers, merging neighbours that share the
// same formatting.
func (w *writer) render(segs []seg) string {
	var b strings.Builder
	var cur *seg
	flush := func() {
		if cur == nil || cur.text == "" {
			cur = nil
			return
		}
		text := cur.text
		if !cur.raw {
			// Markers come before the text, so inside them nothing is at the
			// start of a line any more.
			if w.atLineStart && !cur.bold && !cur.italic && !cur.strike {
				text = htmlmd.Escape(text)
			} else {
				text = htmlmd.EscapeMidLine(text)
			}
		}
		switch {
		case cur.bold && cur.italic:
			text = "***" + text + "***"
		case cur.bold:
			text = "**" + text + "**"
		case cur.italic:
			text = "*" + text + "*"
		}
		if cur.strike && !cur.raw {
			text = "~~" + text + "~~"
		}
		b.WriteString(text)
		w.atLineStart = strings.HasSuffix(text, "\n")
		cur = nil
	}
	for _, s := range segs {
		if s.br {
			flush()
			b.WriteString("  \n")
			w.atLineStart = true
			continue
		}
		if cur != nil && !s.raw && !cur.raw &&
			cur.bold == s.bold && cur.italic == s.italic && cur.strike == s.strike {
			cur.text += s.text
			continue
		}
		flush()
		s := s
		cur = &s
	}
	flush()
	return b.String()
}

// on says whether a toggle element is on: <w:b/> means on, <w:b w:val="0"/>
// means off.
func on(n *node) bool {
	if n == nil {
		return false
	}
	v := n.attrs["val"]
	return v != "0" && v != "false" && v != "off"
}

// styleName returns the paragraph style of a pPr.
func styleName(ppr *node) string {
	if s := ppr.find("pStyle"); s != nil {
		return s.attrs["val"]
	}
	return ""
}

// headingLevel maps a Word style name to a Markdown level. Word writes
// "Heading1" or "Heading 1" depending on the version, and a German document has
// "berschrift1" with the umlaut lost in the style id. Title is the top.
func headingLevel(style string) int {
	s := strings.ToLower(strings.ReplaceAll(style, " ", ""))
	switch {
	case s == "title":
		return 1
	case s == "subtitle":
		return 2
	}
	for _, prefix := range []string{"heading", "berschrift", "ueberschrift"} {
		if strings.HasPrefix(s, prefix) {
			rest := strings.TrimPrefix(s, prefix)
			rest = strings.TrimLeft(rest, "-_")
			if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= 6 {
				return n
			}
		}
	}
	return 0
}

// attr returns one attribute of a node that may be nil.
func attr(n *node, key string) string {
	if n == nil {
		return ""
	}
	return n.attrs[key]
}

// blank ends the previous line, so the next block starts on a fresh paragraph.
func (w *writer) blank() {
	s := w.out.String()
	if s == "" || strings.HasSuffix(s, "\n\n") {
		return
	}
	if !strings.HasSuffix(s, "\n") {
		w.out.WriteString("\n")
	}
	w.out.WriteString("\n")
	w.atLineStart = true
}

// finish tidies the result: no blank line at the start, one at the end.
func (w *writer) finish() string {
	s := strings.TrimLeft(w.out.String(), "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	return s + "\n"
}
