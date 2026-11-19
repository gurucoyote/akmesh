// Package rtfmd turns an RTF file into Markdown.
//
// RTF is a text format: groups in braces, control words behind a backslash, and
// the characters in a code page. So the reader is a tokenizer and a small
// paragraph assembler, no library and no dependency.
//
// What it takes from the file: \par and a backslash at the end of a line are
// paragraph breaks, \line a break inside the paragraph, \tab a tab, \'hh one
// byte in the document's code page, \uN a character by number, \b and \i the
// emphasis markers, and the style of a paragraph (\s1 with the name "Heading 1"
// in the stylesheet) its heading level. The font table, the colour table, the
// document info and pictures are skipped: they carry no text for a reader.
package rtfmd

import (
	"fmt"
	"strconv"
	"strings"

	"akmesh/internal/htmlmd"
)

// Convert reads RTF and returns the Markdown for it.
func Convert(data []byte) (string, error) {
	text := string(data)
	if !strings.HasPrefix(strings.TrimSpace(text), `{\rtf`) {
		return "", fmt.Errorf("not an RTF document (no {\\rtf header)")
	}
	w := &writer{styles: stylesheet(text)}
	w.run(text)
	return w.finish(), nil
}

// token is one piece of the markup.
type token struct {
	kind  int // tokText, tokControl, tokOpen, tokClose, tokChar
	name  string
	param int
	hasP  bool
	text  string
}

const (
	tokText = iota
	tokControl
	tokOpen
	tokClose
	tokChar
)

// tokenizer walks the markup.
type tokenizer struct {
	s   string
	pos int
}

// next returns the next token; ok is false at the end.
func (t *tokenizer) next() (token, bool) {
	for t.pos < len(t.s) {
		c := t.s[t.pos]
		switch {
		case c == '{':
			t.pos++
			return token{kind: tokOpen}, true
		case c == '}':
			t.pos++
			return token{kind: tokClose}, true
		case c == '\\':
			return t.control()
		case c == '\r':
			t.pos++
		case c == '\n':
			// A newline inside the text is just formatting of the markup.
			t.pos++
		default:
			start := t.pos
			for t.pos < len(t.s) && !strings.ContainsRune("{}\\\r\n", rune(t.s[t.pos])) {
				t.pos++
			}
			return token{kind: tokText, text: t.s[start:t.pos]}, true
		}
	}
	return token{}, false
}

// control reads one backslash sequence.
func (t *tokenizer) control() (token, bool) {
	t.pos++ // the backslash
	if t.pos >= len(t.s) {
		return token{kind: tokChar, text: "\\"}, true
	}
	c := t.s[t.pos]
	switch c {
	case '\\', '{', '}':
		t.pos++
		return token{kind: tokChar, text: string(c)}, true
	case '\n', '\r':
		// A backslash before a newline is a paragraph mark.
		t.pos++
		if c == '\r' && t.pos < len(t.s) && t.s[t.pos] == '\n' {
			t.pos++
		}
		return token{kind: tokControl, name: "par"}, true
	case '\'':
		if t.pos+2 < len(t.s) {
			b, err := strconv.ParseUint(t.s[t.pos+1:t.pos+3], 16, 8)
			t.pos += 3
			if err == nil {
				return token{kind: tokChar, text: string(cp1252(byte(b)))}, true
			}
			return token{}, true
		}
		t.pos++
		return token{}, true
	case '*':
		t.pos++
		return token{kind: tokControl, name: "*"}, true
	case '~':
		t.pos++
		return token{kind: tokChar, text: " "}, true
	case '_', '-':
		t.pos++
		return token{}, true
	}
	if !isAlpha(c) {
		t.pos++
		return token{kind: tokChar, text: string(c)}, true
	}
	start := t.pos
	for t.pos < len(t.s) && isAlpha(t.s[t.pos]) {
		t.pos++
	}
	tok := token{kind: tokControl, name: t.s[start:t.pos]}
	if t.pos < len(t.s) && (t.s[t.pos] == '-' || (t.s[t.pos] >= '0' && t.s[t.pos] <= '9')) {
		numStart := t.pos
		if t.s[t.pos] == '-' {
			t.pos++
		}
		for t.pos < len(t.s) && t.s[t.pos] >= '0' && t.s[t.pos] <= '9' {
			t.pos++
		}
		if n, err := strconv.Atoi(t.s[numStart:t.pos]); err == nil {
			tok.param, tok.hasP = n, true
		}
	}
	if t.pos < len(t.s) && t.s[t.pos] == ' ' {
		t.pos++ // one space belongs to the control word
	}
	return tok, true
}

func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// cp1252 maps one byte of the Windows code page to a character. The range
// 0x80..0x9F is where it differs from Latin-1, and that is exactly the range
// that holds the quotes and dashes a word processor writes.
func cp1252(b byte) rune {
	if b < 0x80 || b >= 0xA0 {
		return rune(b)
	}
	table := [32]rune{
		'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡',
		'ˆ', '‰', 'Š', '‹', 'Œ', 0x8D, 'Ž', 0x8F,
		0x90, '‘', '’', '“', '”', '•', '–', '—',
		'˜', '™', 'š', '›', 'œ', 0x9D, 'ž', 'Ÿ',
	}
	if r := table[b-0x80]; r != 0 {
		return r
	}
	return rune(b)
}

// headingLevel maps a style name to a Markdown level, the same way the docx
// reader does it: Word writes "Heading 1", and a German document has
// "Überschrift 1" with the umlaut lost in the style name.
func headingLevel(name string) int {
	s := strings.ToLower(strings.ReplaceAll(name, " ", ""))
	switch {
	case s == "title":
		return 1
	case s == "subtitle":
		return 2
	}
	for _, prefix := range []string{"heading", "berschrift", "ueberschrift"} {
		if strings.HasPrefix(s, prefix) {
			rest := strings.TrimLeft(strings.TrimPrefix(s, prefix), "-_")
			if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= 6 {
				return n
			}
		}
	}
	return 0
}

// stylesheet collects the style names, so that "\s1" can become a heading.
func stylesheet(text string) map[int]string {
	out := map[int]string{}
	tz := &tokenizer{s: text}
	depth := 0
	inSheet := false
	first := map[int]bool{}
	cur := -1
	var name strings.Builder
	for {
		tok, ok := tz.next()
		if !ok {
			break
		}
		switch tok.kind {
		case tokOpen:
			depth++
			first[depth] = true
		case tokClose:
			if inSheet && depth == 3 && cur >= 0 {
				// Word ends the name with a semicolon.
				out[cur] = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(name.String()), ";"))
				cur = -1
				name.Reset()
			}
			delete(first, depth)
			depth--
			if depth < 2 {
				inSheet = false
			}
		case tokControl:
			if first[depth] {
				first[depth] = false
				if tok.name == "stylesheet" {
					inSheet = true
				}
			}
			// The style groups sit one level deeper than the stylesheet.
			// The style number is the parameter of \s (the name letters stop
			// at the digit), and \snext and friends are different words.
			if inSheet && depth == 3 && cur < 0 && tok.name == "s" && tok.hasP {
				cur = tok.param
			}
		case tokText:
			if inSheet && cur >= 0 {
				name.WriteString(tok.text)
			}
		}
	}
	return out
}

// seg is a piece of one line with its formatting, merged before writing so that
// neighbouring bold runs do not produce four asterisks.
type seg struct {
	text   string
	bold   bool
	italic bool
}

// writer assembles the Markdown.
type writer struct {
	out     strings.Builder
	styles  map[int]string
	atStart bool

	bold, italic bool
	skip         int // depth of a skipped destination, 0 when writing
	ucSkip       int // characters to drop after \uN
	pending      int // characters still to drop

	par      []seg
	bullet   bool
	numbered bool
	listNo   int
	inList   bool
	styleNo  int
}

// run walks the document and writes paragraphs as they end.
func (w *writer) run(text string) {
	w.atStart = true
	w.ucSkip = 1
	tz := &tokenizer{s: text}
	var groupStack []int // skip depth per open group, -1 when writing
	for {
		tok, ok := tz.next()
		if !ok {
			break
		}
		switch tok.kind {
		case tokOpen:
			groupStack = append(groupStack, -1)
			groupStack[len(groupStack)-1] = w.skip
		case tokClose:
			if len(groupStack) > 0 {
				w.skip = groupStack[len(groupStack)-1]
				groupStack = groupStack[:len(groupStack)-1]
			}
			if len(groupStack) == 0 {
				w.ending()
			}
		case tokText:
			w.text(tok.text)
		case tokChar:
			w.text(tok.text)
		case tokControl:
			w.control(tok)
		}
	}
	w.ending()
}

// control handles one control word.
func (w *writer) control(tok token) {
	if tok.name == "*" {
		w.skip++
		return
	}
	// The list markers are destinations (their content is the bullet itself),
	// but they say what kind of paragraph follows, so they are read first.
	switch tok.name {
	case "pn", "pntext", "listtext", "pnlvlblt":
		w.bullet = true
	case "pnlvlbody":
		w.numbered = true
	}
	// Destinations whose content is not text for the reader.
	if isSkipDestination(tok.name) {
		w.skip++
		return
	}
	if w.skip > 0 {
		return
	}
	switch tok.name {
	case "par", "pard", "sect", "page":
		if tok.name == "pard" {
			// \pard resets the paragraph formatting: whatever style or list
			// marker came before it belongs to the previous paragraph.
			w.styleNo = 0
			w.bullet = false
			w.numbered = false
			return
		}
		w.ending()
	case "line":
		w.add(seg{text: "  \n"})
	case "tab":
		w.add(seg{text: "\t"})
	case "b":
		w.bold = !tok.hasP || tok.param != 0
	case "i":
		w.italic = !tok.hasP || tok.param != 0
	case "ul", "ulnone":
		// Markdown has no underline; the text stays, the decoration goes.
	case "u":
		if tok.hasP {
			r := rune(tok.param)
			if r < 0 {
				r += 65536
			}
			w.text(string(r))
			w.pending = w.ucSkip
		}
	case "uc":
		if tok.hasP {
			w.ucSkip = tok.param
		}
	case "plain":
		w.bold, w.italic = false, false
	case "bullet":
		w.bullet = true
		w.text("")
	case "s", "cs":
		if tok.hasP && tok.name == "s" {
			w.styleNo = tok.param
		}
	}
}

// isSkipDestination says whether a control word starts a destination whose
// content must not be printed.
func isSkipDestination(name string) bool {
	switch name {
	case "fonttbl", "colortbl", "stylesheet", "info", "pict", "header", "headerl",
		"headerr", "headerf", "footer", "footerl", "footerr", "footerf", "footnote",
		"annotation", "field", "fldinst", "object", "objdata", "themedata",
		"colorschememapping", "datastore", "latentstyles", "listtable",
		"listoverridetable", "rsidtbl", "generator", "xmlnstbl", "mmathPr",
		"pn", "pntext", "listtext":
		return true
	}
	return false
}

// text adds characters, dropping the fallback characters that follow \uN.
func (w *writer) text(s string) {
	if w.skip > 0 {
		return
	}
	if w.pending > 0 {
		runes := []rune(s)
		n := w.pending
		if n > len(runes) {
			n = len(runes)
		}
		w.pending -= n
		s = string(runes[n:])
		if s == "" {
			return
		}
	}
	w.add(seg{text: s, bold: w.bold, italic: w.italic})
}

// add appends one piece, merging it with the previous one when the formatting
// is the same.
func (w *writer) add(s seg) {
	if s.text == "" {
		return
	}
	if n := len(w.par); n > 0 && !strings.HasSuffix(w.par[n-1].text, "\n") &&
		w.par[n-1].bold == s.bold && w.par[n-1].italic == s.italic {
		w.par[n-1].text += s.text
		return
	}
	w.par = append(w.par, s)
}

// ending writes the paragraph that has been assembled.
func (w *writer) ending() {
	segs := w.par
	w.par = nil
	level := headingLevel(w.styles[w.styleNo])
	w.styleNo = 0
	bullet, numbered := w.bullet, w.numbered
	w.bullet, w.numbered = false, false

	// Trim the paragraph: leading and trailing spaces are markup noise.
	for len(segs) > 0 && strings.TrimSpace(segs[0].text) == "" {
		segs = segs[1:]
	}
	if len(segs) > 0 {
		last := len(segs) - 1
		segs[last].text = strings.TrimRight(segs[last].text, " \t")
		if segs[last].text == "" {
			segs = segs[:last]
		}
	}
	if len(segs) == 0 {
		return
	}

	body := w.render(segs)
	switch {
	case bullet:
		w.startList(false)
		w.out.WriteString("- " + body + "\n")
		w.atStart = false
	case numbered:
		w.startList(true)
		w.listNo++
		w.out.WriteString(strconv.Itoa(w.listNo) + ". " + body + "\n")
		w.atStart = false
	case level > 0:
		w.endList()
		w.blank()
		w.out.WriteString(strings.Repeat("#", level) + " " + body + "\n")
		w.atStart = false
	default:
		w.endList()
		w.blank()
		w.out.WriteString(body + "\n")
		w.atStart = false
	}
}

// startList breaks the paragraph before the first item of a list.
func (w *writer) startList(numbered bool) {
	if w.inList {
		return
	}
	w.blank()
	w.inList = true
	w.listNo = 0
	_ = numbered
}

// endList forgets the list state, so the next paragraph starts fresh.
func (w *writer) endList() {
	if !w.inList {
		return
	}
	w.inList = false
	w.listNo = 0
}

// render writes the pieces with their markers.
func (w *writer) render(segs []seg) string {
	var b strings.Builder
	for _, s := range segs {
		text := escape(s.text, b.Len() == 0 && w.atStart)
		switch {
		case s.bold && s.italic:
			text = "***" + text + "***"
		case s.bold:
			text = "**" + text + "**"
		case s.italic:
			text = "*" + text + "*"
		}
		b.WriteString(text)
	}
	return b.String()
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
