package pdfmd

import (
	"strconv"
	"strings"
)

// The structure tree of a tagged PDF.
//
// A tagged document does not leave its meaning to the positions of the text: it
// carries a tree of elements - heading, paragraph, list, table - and every
// element points at the marked content blocks (MCID) that make up its text. A
// reader that follows the tree gets the headings back as headings instead of
// guessing them from font sizes, and gets a table as a table even where the
// cells are not drawn.
//
// Trees are only used when they are there and when they carry text. Anything
// else falls back to reading the page, because a half filled tree would be
// worse than no tree.

// structElem is one element of the tree: its role (S in the file), an optional
// alternative text, and its children - elements or marked content references.
type structElem struct {
	role string
	alt  string
	kids []any
}

// mcr is one marked content reference: a block on a page.
type mcr struct {
	page string // object key of the page
	mcid int
}

// structTreeOf returns the root of the structure tree, or nil.
func structTreeOf(d map[string]*object) *structElem {
	var catalog *object
	for _, obj := range d {
		if hasTag(obj.dict, "/Type/Catalog") {
			catalog = obj
			break
		}
	}
	if catalog == nil {
		return nil
	}
	refs := refsIn(value(catalog.dict, "StructTreeRoot"))
	if len(refs) == 0 {
		return nil
	}
	root := d[refKey(refs[0])]
	if root == nil {
		return nil
	}
	elems := kidsOf(d, value(root.dict, "K"), "", 0)
	if len(elems) == 0 {
		return nil
	}
	if e, ok := elems[0].(*structElem); ok {
		return e
	}
	// The root's kids are bare marked content blocks: there is no element to
	// speak of, and the page reader does a better job.
	return nil
}

// refKey is the key an object is stored under. The parser keeps the whole
// reference ("18 0"), because that is how it finds the header in the file.
func refKey(ref string) string { return strings.TrimSpace(ref) }

// kidsOf reads a /K value: a dictionary, an array, or a reference to either.
func kidsOf(d map[string]*object, k string, pg string, depth int) []any {
	if depth > 24 {
		return nil
	}
	k = strings.TrimSpace(k)
	if k == "" {
		return nil
	}
	var items []string
	if strings.HasPrefix(k, "[") {
		items = arrayItems(k)
	} else {
		items = []string{k}
	}
	var out []any
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		// A bare number is a marked content block on the page the element
		// names with /Pg, which is how one writer in this house writes it.
		if isNumber(item) {
			if n, err := strconv.Atoi(item); err == nil {
				out = append(out, mcr{page: pg, mcid: n})
			}
			continue
		}
		if !strings.HasPrefix(item, "<<") {
			// A reference: the element lives in its own object.
			refs := refsIn(item)
			if len(refs) == 0 {
				continue
			}
			obj := d[refKey(refs[0])]
			if obj == nil {
				continue
			}
			item = obj.dict
		}
		if mcid := value(item, "MCID"); mcid != "" {
			n, err := strconv.Atoi(strings.TrimSpace(strings.Fields(mcid)[0]))
			if err != nil {
				continue
			}
			page := ""
			if refs := refsIn(value(item, "Pg")); len(refs) > 0 {
				page = refKey(refs[0])
			}
			out = append(out, mcr{page: page, mcid: n})
			continue
		}
		role := value(item, "S")
		if role == "" {
			// A grouping dictionary without a role still has children.
			out = append(out, kidsOf(d, value(item, "K"), pg, depth+1)...)
			continue
		}
		e := &structElem{role: role, alt: value(item, "Alt")}
		// The element's own /Pg is where its bare MCID numbers point.
		ownPg := pg
		if refs := refsIn(value(item, "Pg")); len(refs) > 0 {
			ownPg = refKey(refs[0])
		}
		e.kids = kidsOf(d, value(item, "K"), ownPg, depth+1)
		out = append(out, e)
	}
	return out
}

// arrayItems cuts a [...] value into its items and keeps what belongs
// together: a reference is three tokens ("19 0 R"), a dictionary runs to its
// closing brackets. Splitting on whitespace alone tears both apart.
func arrayItems(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		return nil
	}
	s = s[1:]
	var out []string
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
			continue
		case c == ']':
			return out
		case c == '<' && i+1 < len(s) && s[i+1] == '<':
			start, depth := i, 0
			for i < len(s) {
				if i+1 < len(s) && s[i] == '<' && s[i+1] == '<' {
					depth++
					i += 2
					continue
				}
				if i+1 < len(s) && s[i] == '>' && s[i+1] == '>' {
					depth--
					i += 2
					if depth == 0 {
						break
					}
					continue
				}
				i++
			}
			out = append(out, s[start:i])
			continue
		}
		// A word: a number, a name, or the first number of a reference.
		start := i
		for i < len(s) && !strings.ContainsRune(" \t\r\n]<>", rune(s[i])) {
			i++
		}
		word := s[start:i]
		if isNumber(word) {
			// "N G R" is one reference; the generation and the R belong to it.
			j := skipSpace(s, i)
			gStart := j
			for j < len(s) && isDigit(s[j]) {
				j++
			}
			if j > gStart {
				k := skipSpace(s, j)
				if k < len(s) && s[k] == 'R' && (k+1 >= len(s) || strings.ContainsRune(" \t\r\n]", rune(s[k+1]))) {
					out = append(out, word+" "+s[gStart:j]+" R")
					i = k + 1
					continue
				}
			}
		}
		out = append(out, word)
	}
	return out
}

// skipSpace returns the first index at or after i that is not whitespace.
func skipSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n') {
		i++
	}
	return i
}

// structText is the text of the marked content blocks, page by page and in the
// order the pages are read. The order matters: an element that names no page
// takes the first page that has its block.
type structText struct {
	pages  []string
	byPage map[string]map[int]string
}

// get returns the text of one block. A block without a page is looked up in the
// pages in reading order, because several writers leave /Pg off an element and
// let it be inherited.
func (t structText) get(page string, mcid int) string {
	if page != "" {
		return t.byPage[page][mcid]
	}
	for _, p := range t.pages {
		if s, ok := t.byPage[p][mcid]; ok {
			return s
		}
	}
	return ""
}

// textOfStructPages reads every page once and keeps the text per MCID, so the
// tree can be filled with it.
func textOfStructPages(d map[string]*object, pages []*pageRef) structText {
	out := structText{byPage: map[string]map[int]string{}}
	for _, p := range pages {
		res := value(p.obj.dict, "Resources")
		if !strings.HasPrefix(res, "<<") {
			if refs := refsIn(res); len(refs) > 0 {
				if obj := d[refs[0]]; obj != nil {
					res = obj.dict
				}
			}
		}
		var content []byte
		key := value(p.obj.dict, "Contents")
		if strings.HasPrefix(key, "[") {
			for _, ref := range refsIn(key) {
				if obj := d[ref]; obj != nil {
					content = append(content, decoded(obj)...)
					content = append(content, '\n')
				}
			}
		} else if refs := refsIn(key); len(refs) > 0 {
			if obj := d[refs[0]]; obj != nil {
				content = decoded(obj)
			}
		}
		r := &contentReader{
			fonts:     readFonts(d, res),
			doc:       d,
			m:         identity(),
			xobjects:  xobjectsOf(d, res),
			lineSizes: map[float64]int{},
		}
		r.run(content, 0)
		r.flushLine()
		// The text of a block is its pieces in reading order. A word space is
		// either marked on the piece or comes from a line break inside the same
		// block - a paragraph that wraps is still one paragraph.
		type pieceRef struct {
			pc piece
			y  float64
		}
		byID := map[int][]pieceRef{}
		for _, line := range r.lines {
			for _, pc := range line.pieces {
				if pc.mcid < 0 {
					continue
				}
				byID[pc.mcid] = append(byID[pc.mcid], pieceRef{pc: pc, y: line.y})
			}
		}
		m := map[int]string{}
		for id, pieces := range byID {
			var b strings.Builder
			for i, p := range pieces {
				if i > 0 && (p.pc.space || p.y != pieces[i-1].y) && !strings.HasSuffix(b.String(), " ") {
					b.WriteString(" ")
				}
				b.WriteString(p.pc.text)
			}
			m[id] = strings.Join(strings.Fields(b.String()), " ")
		}
		out.pages = append(out.pages, p.key)
		out.byPage[p.key] = m
	}
	return out
}

// toMarkdown renders the tree. The empty string means the tree carried no text,
// and the caller falls back to reading the pages.
func (e *structElem) toMarkdown(text structText, fallbackPage string) string {
	var b strings.Builder
	e.write(&b, text, fallbackPage)
	out := collapseBlank(b.String())
	if strings.TrimSpace(out) == "" {
		return ""
	}
	return out
}

// write renders one element and its children.
func (e *structElem) write(b *strings.Builder, text structText, fallbackPage string) {
	switch role := normalizeRole(e.role); {
	case strings.HasPrefix(role, "H") && len(role) == 2:
		level := int(role[1] - '0')
		if level < 1 || level > 6 {
			level = 1
		}
		// An empty block is not written: a heading without text would leave a
		// bare "#" behind, and that is worse than nothing.
		if txt := e.ownText(text, fallbackPage); txt != "" {
			addBlock(b, strings.Repeat("#", level)+" "+txt)
		}
		return
	case role == "P", role == "Caption", role == "BlockQuote", role == "Note", role == "Code":
		body := e.ownText(text, fallbackPage)
		if role == "Code" {
			addBlock(b, "```\n"+body+"\n```")
			return
		}
		addBlock(b, body)
		return
	case role == "L", role == "LI", role == "Lbl", role == "LBody":
		// Lists are rendered from their items below; a list item carries its
		// own text plus whatever is nested inside it.
		if role == "LI" {
			// The label and the body of one item belong on one line; what is
			// nested (a list inside a list) goes underneath.
			body := e.ownText(text, fallbackPage)
			var inner strings.Builder
			for _, k := range e.kids {
				kid, ok := k.(*structElem)
				if !ok {
					continue
				}
				if inlineRole(normalizeRole(kid.role)) {
					continue // already in body
				}
				kid.write(&inner, text, fallbackPage)
			}
			if body == "" && strings.TrimSpace(inner.String()) == "" {
				return
			}
			item := "- " + body
			if s := strings.TrimSpace(inner.String()); s != "" {
				item += "\n" + indent(s)
			}
			addBlock(b, strings.TrimRight(item, " "))
			return
		}
		var items []string
		for _, k := range e.kids {
			kid, ok := k.(*structElem)
			if !ok {
				continue
			}
			if normalizeRole(kid.role) == "LI" {
				var one strings.Builder
				kid.write(&one, text, fallbackPage)
				if s := strings.TrimSpace(one.String()); s != "" {
					items = append(items, s)
				}
				continue
			}
			kid.write(b, text, fallbackPage)
		}
		if len(items) > 0 {
			addBlock(b, strings.Join(items, "\n"))
		}
		return
	case role == "Table":
		if t := e.asTable(text, fallbackPage); t != "" {
			addBlock(b, t)
			return
		}
		// No rows: the text of the table is still text.
		addBlock(b, e.ownText(text, fallbackPage))
		return
	case role == "TR", role == "TD", role == "TH", role == "THead", role == "TBody", role == "TFoot":
		// Rendering happens in asTable; reaching here means the table had no
		// usable rows.
		addBlock(b, e.ownText(text, fallbackPage))
		return
	case role == "Figure", role == "Formula", role == "Form":
		// There is no picture without its bytes, but the alternative text is
		// exactly what a reader wants in that case.
		if e.alt != "" {
			addBlock(b, "["+e.alt+"]")
			return
		}
		addBlock(b, e.ownText(text, fallbackPage))
		return
	}
	// Document, Part, Sect, Div, Article, Span, Link and everything unknown:
	// their content belongs to the document, in order.
	own := e.ownText(text, fallbackPage)
	if own != "" {
		addBlock(b, own)
	}
	for _, k := range e.kids {
		if kid, ok := k.(*structElem); ok {
			kid.write(b, text, fallbackPage)
		}
	}
}

// ownText is the text an element carries: its own marked content blocks and
// whatever sits inside an inline child - a link, a span, a label. A block child
// is a block and is written on its own, so it is not swallowed here.
func (e *structElem) ownText(text structText, fallbackPage string) string {
	var parts []string
	var collect func(n *structElem)
	collect = func(n *structElem) {
		for _, k := range n.kids {
			switch v := k.(type) {
			case mcr:
				if s := text.get(v.page, v.mcid); s != "" {
					parts = append(parts, s)
				}
			case *structElem:
				if inlineRole(normalizeRole(v.role)) {
					collect(v)
				}
			}
		}
	}
	collect(e)
	// Text from several blocks is joined by a space, except in front of a
	// punctuation mark: a period belongs to the word it closes.
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" && !strings.ContainsAny(p[:1], ".,;:!?)]}") {
			out += " "
		}
		out += p
	}
	return strings.TrimSpace(out)
}

// inlineRole says whether an element lives inside a paragraph rather than next
// to one. Everything unknown counts as inline: dropping an element's text
// because its role is unfamiliar would lose text for no reason.
func inlineRole(role string) bool {
	switch role {
	case "P", "H1", "H2", "H3", "H4", "H5", "H6", "L", "LI",
		"Table", "TR", "TD", "TH", "THead", "TBody", "TFoot",
		"Figure", "Document", "Part", "Sect", "Div", "Article", "TOC", "TOCI":
		return false
	}
	return true
}

// asTable builds a pipe table from the rows of a Table element.
func (e *structElem) asTable(text structText, fallbackPage string) string {
	var rows [][]string
	var walk func(n *structElem)
	walk = func(n *structElem) {
		for _, k := range n.kids {
			kid, ok := k.(*structElem)
			if !ok {
				continue
			}
			switch normalizeRole(kid.role) {
			case "TR":
				var cells []string
				for _, c := range kid.kids {
					cell, ok := c.(*structElem)
					if !ok {
						continue
					}
					switch normalizeRole(cell.role) {
					case "TD", "TH":
						cells = append(cells, strings.ReplaceAll(cell.ownText(text, fallbackPage), "|", "\\|"))
					}
				}
				if len(cells) > 0 && strings.TrimSpace(strings.Join(cells, "")) != "" {
					rows = append(rows, cells)
				}
			case "THead", "TBody", "TFoot":
				walk(kid)
			}
		}
	}
	walk(e)
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	for i, rw := range rows {
		b.WriteString("| " + strings.Join(rw, " | ") + " |\n")
		if i == 0 {
			b.WriteString("|" + strings.Repeat(" --- |", len(rw)) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// normalizeRole turns a role name into its short form: the file may say H1 or
// the namespace prefixed form.
func normalizeRole(role string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		return ""
	}
	role = strings.TrimPrefix(role, "/")
	if i := strings.LastIndex(role, ":"); i >= 0 {
		role = role[i+1:]
	}
	return role
}

// addBlock appends a block, keeping one empty line between blocks.
func addBlock(b *strings.Builder, body string) {
	body = strings.TrimRight(body, " \t\n")
	if body == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(body)
	b.WriteString("\n")
}

// indent shifts a nested block by two spaces, so a list item keeps its own text
// apart from what is inside it.
func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = "  " + l
		}
	}
	return strings.Join(lines, "\n")
}

// collapseBlank leaves at most one empty line in a row.
func collapseBlank(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s) + "\n"
}
