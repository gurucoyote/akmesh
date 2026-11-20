// Package htmlmd turns an HTML fragment into Markdown.
//
// The file exists twice, identical: as internal/htmlmd/htmlmd.go in akmesh and
// in rekr. One implementation, two copies; a fix goes into both, and a diff
// between the two files has to be empty. What each project adds around it lives
// beside this file (rekr: page.go with the header and the blocked
// constructs).
//
// It is meant for what a hull meets in practice: a saved page, a mail body, a
// caption. The rules follow Turndown (the converter every JavaScript tool uses)
// and JohannesKaufmann/html-to-markdown, so the output is what an agent expects
// from "html to markdown", not a house dialect:
//
//	h1..h6 -> "#".."######", strong/b -> **, em/i -> *, del/s/strike -> ~~,
//	code -> `, pre -> fenced block, a -> [text](url), img -> ![alt](src),
//	ul/li -> "- ", ol/li -> "1. ", blockquote -> "> ", hr -> "---",
//	br -> two spaces and a newline.
//
// Text is escaped with the same set Turndown uses ("\" first, then the
// characters Markdown would read as markup), so a paragraph that contained
// "a*b" comes back as "a*b" and does not turn into emphasis.
//
// No library: the fragment is tokenized here, about a hundred lines, because
// the only thing needed is tags, attributes and text. Entity references are
// resolved by the standard library (html.UnescapeString).
package htmlmd

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// Convert turns one HTML fragment into Markdown. Unknown elements keep their
// text but lose their tag, except pre and code, which carry content that must
// not be mangled.
func Convert(fragment string) string {
	return convert(fragment, false)
}

// ConvertGFM is Convert with tables written as a GFM table: a header row, a
// separator row, then the body. For reading a page the plain form is nicer, but
// a writer that has to build a real table needs the separator - without it a
// Markdown table is a run of lines with pipes in it.
func ConvertGFM(fragment string) string {
	return convert(fragment, true)
}

func convert(fragment string, gfm bool) string {
	p := &parser{src: fragment, lineStart: true, gfmTables: gfm}
	p.run()
	p.closeLinks()
	return p.finish()
}

// parser walks the fragment once and writes Markdown as it goes.
type parser struct {
	src string
	pos int
	out strings.Builder
	// state that the handlers need
	listDepth   int
	listOrdered []bool
	inPre       bool
	inCode      bool
	inRow       bool
	cells       int
	hrefs       []string
	quoteDepth  int
	lineStart   bool
	cdata       string // "script" or "style" while inside one
	gfmTables   bool   // write a GFM table with its separator row
	rowIndex    int    // which row of the current table is being written
}

func (p *parser) run() {
	for p.pos < len(p.src) {
		if p.cdata != "" {
			p.skipCDATA()
			continue
		}
		switch {
		case strings.HasPrefix(p.src[p.pos:], "<!--"):
			end := strings.Index(p.src[p.pos:], "-->")
			if end < 0 {
				p.pos = len(p.src)
			} else {
				p.pos += end + 3
			}
		case p.src[p.pos] == '<':
			start := p.pos
			name, closing, selfClosing, attrs, next := parseTag(p.src, p.pos)
			if next < 0 {
				// A "<" that starts no tag is text.
				p.text("<")
				p.pos = start + 1
				continue
			}
			p.pos = next
			p.tag(name, closing, selfClosing, attrs)
		default:
			next := strings.IndexByte(p.src[p.pos:], '<')
			if next < 0 {
				next = len(p.src) - p.pos
			}
			p.text(html.UnescapeString(p.src[p.pos : p.pos+next]))
			p.pos += next
		}
	}
}

// skipCDATA advances past the content of script or style. A browser shows none
// of it; written out as text it would put JavaScript and URLs on the page.
// pos stops at the closing tag, which the normal loop then handles.
func (p *parser) skipCDATA() {
	lower := strings.ToLower(p.src[p.pos:])
	if i := strings.Index(lower, "</"+p.cdata); i >= 0 {
		p.pos += i
	} else {
		p.pos = len(p.src)
	}
	p.cdata = ""
}

// text writes text, escaping what Markdown would otherwise read as markup, and
// leaves the content of pre alone.
func (p *parser) text(s string) {
	if s == "" {
		return
	}
	// A saved page from Windows carries CRLF, and a stray CR is a line break
	// in some editors: both become the LF Markdown expects.
	if strings.ContainsRune(s, '\r') {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
	}
	if p.inPre || p.inCode {
		p.out.WriteString(s)
		p.lineStart = strings.HasSuffix(s, "\n")
		return
	}
	if p.quoteDepth > 0 && p.lineStart {
		p.out.WriteString(p.prefix())
	}
	// Indentation in the source is layout, not text: four spaces or a tab at a
	// line start would make a code block out of the line (CommonMark 4.4).
	if p.lineStart {
		s = strings.TrimLeft(s, " 	")
	}
	if s == "" {
		return
	}
	if i := strings.Index(s, "\n"); i >= 0 {
		s = trimLineIndents(s)
	}
	p.out.WriteString(escapeText(s))
	if s != "" {
		last := s[len(s)-1]
		p.lineStart = last == '\n'
	}
}

// Escape protects text that is about to be written as Markdown, so that what
// looked like a character stays a character. Other converters in this binary
// (docxmd) write the same kind of output and use the same rules.
func Escape(text string) string { return escapeText(text) }

// EscapeMidLine is Escape for text that does not start a line: the rules for
// "#", "-" and "1." only apply at the start of a line, and applying them
// everywhere turns "11. Juni" into "11\\. Juni".
func EscapeMidLine(text string) string { return escapeTextOpt(text, false) }

// escapeText is Turndown's escaping: the characters that would start markup are
// escaped, and nothing else, so that "x := 1" stays readable.
func escapeText(s string) string { return escapeTextOpt(s, true) }

// escapeTextOpt is escapeText with the line start as a parameter.
func escapeTextOpt(s string, startOfLine bool) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	lineStart := startOfLine
	// digitsOnLine says whether everything on this line so far is a number that
	// began it: only then is a dot a list marker.
	digitsOnLine := startOfLine
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\n':
			b.WriteByte(c)
			lineStart = true
			digitsOnLine = true
			continue
		case strings.IndexByte("\\`*_[]|", c) >= 0:
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '#' && lineStart:
			b.WriteByte('\\')
			b.WriteByte(c)
		case (c == '-' || c == '+' || c == '=' || c == '>') && lineStart:
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '.' && digitsOnLine && digitsBefore(b.String()):
			// "1." at the start of a line would become an ordered list item;
			// "11. Juni" in the middle of a sentence is a date.
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
		if c != ' ' && c != '\t' {
			lineStart = false
		}
		if c < '0' || c > '9' {
			if c != ' ' && c != '\t' {
				digitsOnLine = false
			}
		}
	}
	return b.String()
}

// digitsBefore says whether the text on this line is only digits so far, which
// is what makes a following dot a list marker.
func digitsBefore(s string) bool {
	if s == "" {
		return false
	}
	line := s
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		line = s[i+1:]
	}
	if line == "" {
		return false
	}
	for i := 0; i < len(line); i++ {
		if line[i] < '0' || line[i] > '9' {
			return false
		}
	}
	return true
}

// block ends the current paragraph and starts a new one.
func (p *parser) block() {
	s := p.out.String()
	if s == "" || strings.HasSuffix(s, "\n\n") {
		p.lineStart = true
		return
	}
	if !strings.HasSuffix(s, "\n") {
		p.out.WriteString("\n")
	}
	p.out.WriteString("\n")
	p.lineStart = true
}

// itemBreak starts a new list item or table row: one newline, not a paragraph
// break, or every entry would get a blank line between them.
func (p *parser) itemBreak() {
	s := p.out.String()
	if s == "" {
		return
	}
	s = strings.TrimSuffix(s, "\n\n")
	s = strings.TrimSuffix(s, "\n")
	p.out.Reset()
	p.out.WriteString(s)
	p.out.WriteString("\n")
	p.lineStart = true
}

// prefix is what goes in front of every line at this point: quote markers and
// list indentation.
func (p *parser) prefix() string {
	var b strings.Builder
	for i := 0; i < p.quoteDepth; i++ {
		b.WriteString("> ")
	}
	if p.listDepth > 0 {
		b.WriteString(strings.Repeat("  ", p.listDepth-1))
	}
	return b.String()
}

// tag handles one start or end tag.
func (p *parser) tag(name string, closing, selfClosing bool, attrs map[string]string) {
	if p.inPre && name != "pre" && name != "code" {
		// Inside pre the text is taken verbatim; tags keep their raw form so
		// that nothing in a code block is lost.
		if !closing && !selfClosing {
			p.rawTag(name, attrs, false)
			return
		}
		if closing {
			p.out.WriteString("</" + name + ">")
			return
		}
	}

	switch name {
	case "p", "div", "section", "article", "header", "footer", "main", "h1", "h2", "h3", "h4", "h5", "h6", "table", "tr", "td", "th", "ul", "ol", "blockquote", "pre":
		if closing {
			p.closeBlock(name)
		} else if !selfClosing {
			p.openBlock(name, attrs)
		}
	case "br":
		p.out.WriteString("  \n")
		p.lineStart = true
	case "hr":
		p.block()
		p.out.WriteString(p.prefix() + "---\n\n")
	case "a":
		if closing {
			if href := p.popHref(); href != "" {
				p.out.WriteString("](" + linkTarget(href) + ")")
			}
			return
		}
		// An anchor without href is text in a box: no brackets at all.
		href := attrs["href"]
		if href != "" && len(p.hrefs) > 0 {
			// A link inside a link is invalid; the inner one starts after the
			// outer one has ended.
			p.closeLinks()
		}
		p.hrefs = append(p.hrefs, href)
		if href != "" {
			p.out.WriteString("[")
		}
	case "img":
		alt := attrs["alt"]
		src := attrs["src"]
		if src == "" {
			return
		}
		p.out.WriteString("![" + escapeText(alt) + "](" + linkTarget(src) + ")")
	case "strong", "b":
		p.out.WriteString("**")
	case "em", "i":
		p.out.WriteString("*")
	case "del", "s", "strike":
		p.out.WriteString("~~")
	case "code":
		if p.inPre {
			p.out.WriteString("`")
			return
		}
		p.inCode = !closing
		p.out.WriteString("`")
	case "li":
		if closing {
			return
		}
		p.itemBreak()
		marker := "- "
		if n := len(p.listOrdered); n > 0 && p.listOrdered[n-1] {
			marker = "1. "
		}
		p.out.WriteString(p.prefix() + marker)
		p.lineStart = false
	case "script", "style":
		// A browser shows nothing of either, so neither the element nor its
		// content belongs in the Markdown.
		if !closing && !selfClosing {
			p.cdata = name
		}
		return
	case "span", "font", "small", "big", "u", "sup", "sub", "center", "body", "html", "head", "title":
		// Structure without a Markdown counterpart: keep the text.
	default:
		// Unknown element: keep the text, drop the tag. A block-level
		// unknown (custom elements) ends the paragraph like a div would.
		if !closing && !selfClosing && isBlockUnknown(name) {
			p.block()
		}
	}
}

// openBlock starts a block element.
func (p *parser) openBlock(name string, attrs map[string]string) {
	p.closeLinks()
	switch name {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		p.block()
		level := int(name[1] - '0')
		p.out.WriteString(p.prefix() + strings.Repeat("#", level) + " ")
		p.lineStart = false
	case "ul", "ol":
		p.block()
		p.listDepth++
		p.listOrdered = append(p.listOrdered, name == "ol")
	case "blockquote":
		p.block()
		p.quoteDepth++
	case "pre":
		p.block()
		p.inPre = true
		p.out.WriteString("```\n")
	case "table":
		p.block()
		p.rowIndex = 0
	case "tr":
		p.itemBreak()
		p.inRow = true
		p.cells = 0
	case "td", "th":
		if p.cells > 0 {
			p.out.WriteString(" | ")
		}
		p.cells++
	default:
		p.block()
	}
}

// closeBlock ends a block element.
func (p *parser) closeBlock(name string) {
	p.closeLinks()
	switch name {
	case "ul", "ol":
		if p.listDepth > 0 {
			p.listDepth--
		}
		if n := len(p.listOrdered); n > 0 {
			p.listOrdered = p.listOrdered[:n-1]
		}
		p.block()
	case "blockquote":
		if p.quoteDepth > 0 {
			p.quoteDepth--
		}
		p.block()
	case "pre":
		p.inPre = false
		if !strings.HasSuffix(p.out.String(), "\n") {
			p.out.WriteString("\n")
		}
		p.out.WriteString("```\n\n")
	case "td", "th":
		// Closing a cell changes nothing: the separator went in at its start.
	case "tr":
		p.out.WriteString("\n")
		// The first row of a GFM table is its head; the separator below it is
		// what makes a Markdown reader see a table at all.
		if p.gfmTables && p.rowIndex == 0 && p.cells > 0 {
			parts := make([]string, p.cells)
			for i := range parts {
				parts[i] = "---"
			}
			p.out.WriteString(strings.Join(parts, " | ") + "\n")
		}
		p.rowIndex++
		p.inRow = false
		p.cells = 0
		p.lineStart = true
	default:
		p.block()
	}
}

// rawTag writes a tag back out inside pre, so that code samples survive.
func (p *parser) rawTag(name string, attrs map[string]string, closing bool) {
	var b strings.Builder
	b.WriteString("<" + name)
	for k, v := range attrs {
		if v == "" {
			b.WriteString(" " + k)
			continue
		}
		b.WriteString(fmt.Sprintf(" %s=%q", k, v))
	}
	b.WriteString(">")
	p.out.WriteString(b.String())
}

// hrefs is the stack of link targets being written.
func (p *parser) popHref() string {
	if n := len(p.hrefs); n > 0 {
		h := p.hrefs[n-1]
		p.hrefs = p.hrefs[:n-1]
		return h
	}
	return ""
}

// closeLinks ends every link that is still open. A page loses its </a> now and
// then; without this the rest of the page would become link text.
func (p *parser) closeLinks() {
	for len(p.hrefs) > 0 {
		target := p.popHref()
		if target == "" {
			continue
		}
		p.out.WriteString("](" + linkTarget(target) + ")")
	}
}

// linkTarget makes a link or image target Markdown can read. A parenthesis in
// the target would end it early and the whole link would show up as text, so
// such a target goes in angle brackets, which allow it.
func linkTarget(target string) string {
	t := strings.ReplaceAll(target, " ", "%20")
	if strings.ContainsAny(t, "()<>") || strings.ContainsAny(t, "\n\r") {
		t = "<" + strings.ReplaceAll(strings.ReplaceAll(t, ">", "%3E"), "<", "%3C") + ">"
	}
	return t
}

// trimLineIndents removes the indentation of every line after the first. It is
// layout from the source, not text: four spaces or a tab at a line start would
// make a code block out of the line (CommonMark 4.4).
func trimLineIndents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lineStart := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if lineStart && (c == ' ' || c == '	') {
			continue
		}
		b.WriteByte(c)
		lineStart = c == '\n'
	}
	return b.String()
}

// finish tidies the result: no trailing spaces before a newline unless they are
// a hard line break, no more than one blank line, one final newline.
func (p *parser) finish() string {
	s := p.out.String()
	// A hard line break is two spaces and a newline; it has to survive the
	// cleanup, so it is parked under a marker that cannot occur in text.
	s = strings.ReplaceAll(s, "  \n", "\x00HARD\x00")
	s = regexp.MustCompile("[ \t]+\n").ReplaceAllString(s, "\n")
	// "  \n" is a hard break and must survive, so it is restored after the
	// cleanup above removed its spaces.
	s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	s = strings.ReplaceAll(s, "\x00HARD\x00", "  \n")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return s + "\n"
}

// isBlockUnknown says whether an unknown element should end the paragraph. A
// custom element is treated like a div, which is what an agent expects from a
// saved page.
func isBlockUnknown(name string) bool {
	switch name {
	case "video", "audio", "figure", "figcaption", "details", "summary", "dl", "dt", "dd", "nav", "aside", "form", "noscript", "iframe", "canvas", "svg":
		return true
	}
	return false
}

// parseTag reads one tag at s[i] and returns its name, direction, self-closing
// flag, attributes, and the position after it. next is -1 when s[i] does not
// start a tag.
func parseTag(s string, i int) (name string, closing, selfClosing bool, attrs map[string]string, next int) {
	if i >= len(s) || s[i] != '<' {
		return "", false, false, nil, -1
	}
	j := i + 1
	if j < len(s) && s[j] == '/' {
		closing = true
		j++
	}
	start := j
	for j < len(s) && isNameByte(s[j]) {
		j++
	}
	if j == start {
		return "", false, false, nil, -1 // "<" followed by nothing usable
	}
	name = strings.ToLower(s[start:j])
	attrs = map[string]string{}

	// Attributes up to the closing '>'.
	for j < len(s) {
		for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
			j++
		}
		if j >= len(s) {
			return "", false, false, nil, -1 // unterminated tag
		}
		if s[j] == '>' {
			return name, closing, selfClosing, attrs, j + 1
		}
		if s[j] == '/' {
			selfClosing = true
			j++
			continue
		}
		kStart := j
		for j < len(s) && isNameByte(s[j]) {
			j++
		}
		if j == kStart {
			// Stray character inside a tag: skip it rather than loop forever.
			j++
			continue
		}
		k := strings.ToLower(s[kStart:j])
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if j < len(s) && s[j] == '=' {
			j++
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			var v string
			v, j = readAttrValue(s, j)
			attrs[k] = v
		} else {
			attrs[k] = ""
		}
	}
	return "", false, false, nil, -1
}

// readAttrValue reads a quoted or unquoted attribute value.
func readAttrValue(s string, i int) (string, int) {
	if i >= len(s) {
		return "", i
	}
	if q := s[i]; q == '"' || q == '\'' {
		j := i + 1
		for j < len(s) && s[j] != q {
			j++
		}
		v := html.UnescapeString(s[i+1 : min(j, len(s))])
		if j < len(s) {
			j++
		}
		return v, j
	}
	j := i
	for j < len(s) && s[j] != '>' && s[j] != ' ' && s[j] != '\t' && s[j] != '\n' {
		j++
	}
	return html.UnescapeString(s[i:j]), j
}

func isNameByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '-' || b == '_' || b == ':' || b == '.':
		return true
	}
	return false
}
