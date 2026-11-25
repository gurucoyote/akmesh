// Package rtfmd reads RTF and writes Markdown.
//
// The reader has two stages, so each can be tested on its own: Parse turns the
// markup into blocks (paragraphs, headings, list items, tables) with runs for
// the formatting, and the Markdown writer in rtfmd.go turns the blocks into text.
// A second target would be a second writer over the same blocks, not a second
// parser.
//
// The block model and the parser here come back from suntoad's HTML reader
// (github.com/... suntoad, internal/rtfhtml, commit ec96e0f "RTF als HTML lesen",
// suntoad-2g7.10). The rule agreed there: the parser belongs here, the HTML
// writer belongs there; suntoad copies this parser instead of growing its own
// (acmesh-crc9). Changes to the tokenizer or the block structure belong here
// first, then in the copy.
package rtfmd

import (
	"fmt"
	"strconv"
	"strings"
)

// token is one piece of the markup.
type token struct {
	kind  int // tokText, tokControl, tokOpen, tokClose, tokChar
	name  string
	param int
	hasP  bool
	text  string
}

// Kind is what a block is.
type Kind int

const (
	Paragraph Kind = iota
	Heading
	ListItem
	Table
)

// ListKind separates the two markers RTF can put before a list item.
type ListKind int

const (
	NoList ListKind = iota
	Bullet
	Numbered
)

// Run is a piece of text with its formatting. A run carries the link it belongs
// to, because a link is formatting over text in RTF, not a block.
type Run struct {
	Text      string
	Bold      bool
	Italic    bool
	Underline bool
	Size      int    // half points, 0 = unknown
	Link      string // href, empty when there is none
}

// Block is one paragraph, heading, list item or table.
type Block struct {
	Kind  Kind
	Level int      // heading 1..6, list level 1..6
	List  ListKind // only for ListItem
	Align string   // "", "left", "right", "center", "justify"
	Runs  []Run
	// Rows is the table: row, cell, runs. Head marks the first row as a header
	// row, which is only set when the document says so (\trhdr) - guessing from
	// the formatting would be a guess.
	Rows [][][]Run
	Head bool
	// Widths are the column edges in twips from \cellx; empty when unknown.
	Widths []int
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
		case c == '\r', c == '\n':
			// A newline inside the text is formatting of the markup.
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
// 0x80..0x9F is where it differs from Latin-1, and that is the range a word
// processor writes quotes and dashes in.
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

// headingLevel maps a style name to a level: Word writes "Heading 1", a German
// document "Überschrift 1" (with the umlaut often lost in the style name).
func headingLevel(name string) int {
	s := strings.ToLower(strings.ReplaceAll(name, " ", ""))
	switch s {
	case "title":
		return 1
	case "subtitle":
		return 2
	}
	for _, prefix := range []string{"heading", "berschrift", "ueberschrift"} {
		if after, ok := strings.CutPrefix(s, prefix); ok {
			rest := strings.TrimLeft(after, "-_")
			if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= 6 {
				return n
			}
		}
	}
	return 0
}

// Parse reads the markup into blocks.
func Parse(data []byte) ([]Block, error) {
	text := string(data)
	if !strings.HasPrefix(strings.TrimSpace(text), `{\rtf`) {
		return nil, fmt.Errorf("not an RTF document (no {\\rtf header)")
	}
	p := &parser{
		styles: stylesheet(text),
		ucSkip: 1,
	}
	p.walk(text)
	p.endParagraph()
	if p.inTable() {
		p.endTable()
	}
	if !supportedCodepage(p.cpBytes) {
		return p.blocks, fmt.Errorf("codepage %s is not supported, only cp1252", p.cpBytes)
	}
	return p.blocks, nil
}

// supportedCodepage says whether the bytes can be mapped. Every other code page
// would come out as mojibake, and a wrong letter is worse than a clear refusal.
func supportedCodepage(cp string) bool {
	switch strings.ToLower(cp) {
	case "1252", "65001", "0", "":
		return true
	}
	return false
}

// frame is the formatting state at one group. RTF groups inherit: a group that
// ends puts back what was there before it.
type frame struct {
	bold, italic, underline bool
	size                    int
	align                   string
	styleNo                 int
	level                   int
	list                    ListKind
	link                    string
	skip                    bool // this group is a destination not shown
	field                   int  // 0 outside a field, 1 collecting fldinst, 2 showing fldrslt
	starNext                bool // the next control word names a \* destination
}

type parser struct {
	blocks    []Block
	styles    map[int]string
	frames    []frame
	cpBytes   string
	fieldInst strings.Builder

	run        []Run
	buf        strings.Builder
	pendingSki int
	ucSkip     int

	// pendingList and pendingLevel carry a list marker to the paragraph it
	// belongs to. Word writes the marker in a group of its own before the
	// text ({\*\pn\pnlvlblt...}), so a marker inside a frame would be thrown
	// away when that group closes; it lives on the parser for that reason.
	pendingList  ListKind
	pendingLevel int

	// table under construction
	rows     [][][]Run
	row      [][]Run
	cell     []Run
	cellOpen bool // a cell is being written into
	widths   []int
	head     bool
}

func (p *parser) cur() *frame {
	if len(p.frames) == 0 {
		p.frames = append(p.frames, frame{})
	}
	return &p.frames[len(p.frames)-1]
}

func (p *parser) inTable() bool { return p.rows != nil || p.row != nil || p.cellOpen }

func (p *parser) skip() bool { return p.cur().skip }

func (p *parser) walk(text string) {
	tz := &tokenizer{s: text}
	depth := 0
	for {
		tok, ok := tz.next()
		if !ok {
			break
		}
		switch tok.kind {
		case tokOpen:
			depth++
			p.frames = append(p.frames, *p.cur())
		case tokClose:
			if depth > 0 {
				p.frames = p.frames[:len(p.frames)-1]
				depth--
			}
		case tokChar:
			p.write(tok.text)
		case tokText:
			p.write(tok.text)
		case tokControl:
			p.control(tok)
		}
	}
}

// write adds text to the current run, merging with the previous one when the
// formatting is the same: neighbouring pieces of one bold word must not become
// two <strong>.
func (p *parser) write(s string) {
	if s == "" || p.skip() {
		return
	}
	// Characters that follow \uN are the fallback for readers without Unicode;
	// they have to go, and they can arrive as text as well as one by one.
	if p.pendingSki > 0 {
		runes := []rune(s)
		n := min(p.pendingSki, len(runes))
		p.pendingSki -= n
		s = string(runes[n:])
		if s == "" {
			return
		}
	}
	if p.cur().field == 1 {
		// The instruction of a field is not visible; it says where a link goes.
		p.fieldInst.WriteString(s)
		return
	}
	f := p.cur()
	text := p.buf.String() + s
	p.buf.Reset()
	if text == "" {
		return
	}
	r := Run{
		Text:      text,
		Bold:      f.bold,
		Italic:    f.italic,
		Underline: f.underline,
		Size:      f.size,
		Link:      f.link,
	}
	if n := len(p.current()); n > 0 {
		last := &p.current()[n-1]
		if sameFormat(*last, r) {
			last.Text += r.Text
			return
		}
	}
	p.currentAppend(r)
}

// current is where a run goes: the cell while a table is open, else the
// paragraph being collected.
func (p *parser) current() []Run {
	if p.cellOpen {
		return p.cell
	}
	return p.run
}

func (p *parser) currentAppend(r Run) {
	if p.cellOpen {
		p.cell = append(p.cell, r)
		return
	}
	p.run = append(p.run, r)
}

func sameFormat(a, b Run) bool {
	return a.Bold == b.Bold && a.Italic == b.Italic && a.Underline == b.Underline &&
		a.Size == b.Size && a.Link == b.Link
}

// knownDestination lists the destinations this reader understands. Everything
// else after a \* is markup we do not need - and its text is not page text, so
// it has to go: otherwise a document's own metadata ("AppVersion 12.0000")
// appears in the heading.
var knownDestination = map[string]bool{
	"pn": true, "pntext": true, "pntxta": true, "pntxtb": true, "listtext": true,
	"fldinst": true, "fldtype": true, "shppict": true, "nonshppict": true,
}

func (p *parser) control(t token) {
	if p.cur().starNext {
		p.cur().starNext = false
		if !knownDestination[t.name] {
			p.cur().skip = true
			return
		}
	}
	switch t.name {
	case "*":
		p.cur().starNext = true
	case "rtf", "ansi", "ansicpg", "deff", "deflang", "deflangfe", "uc":
		switch t.name {
		case "ansicpg":
			if t.hasP {
				p.cpBytes = strconv.Itoa(t.param)
			}
		case "uc":
			if t.hasP {
				p.ucSkip = t.param
			}
		}
	case "fonttbl", "colortbl", "info", "pict", "object", "header", "footer",
		"headerl", "headerr", "footerl", "footerr", "footnote", "annotation",
		"stylesheet", "listtable", "listoverridetable", "rsidtbl", "generator",
		"xmlnstbl", "themedata", "colorschememapping", "latentstyles", "datastore",
		"shppict", "nonshppict", "fldtype", "pntext", "pntxta", "pntxtb",
		"listtext", "bkmkstart", "bkmkend", "xe", "tc":
		p.cur().skip = true
	case "b":
		p.cur().bold = !t.hasP || t.param != 0
	case "i":
		p.cur().italic = !t.hasP || t.param != 0
	case "ul":
		p.cur().underline = !t.hasP || t.param != 0
	case "ulnone":
		p.cur().underline = false
	case "plain":
		f := p.cur()
		f.bold, f.italic, f.underline = false, false, false
		f.size, f.align = 0, ""
	case "pard":
		f := p.cur()
		f.align, f.styleNo, f.level, f.list = "", 0, 0, NoList
		// The list marker of the previous paragraph does not reach this one.
		p.pendingList, p.pendingLevel = NoList, 0
	case "fs":
		if t.hasP {
			p.cur().size = t.param
		}
	case "qc":
		p.cur().align = "center"
	case "qr":
		p.cur().align = "right"
	case "qj":
		p.cur().align = "justify"
	case "ql":
		p.cur().align = "left"
	case "s":
		if t.hasP {
			p.cur().styleNo = t.param
		}
	case "li":
		if t.hasP {
			// An indent of 720 twips (half an inch) is one list level.
			f := p.cur()
			f.level = 720
			if t.param > 0 {
				f.level = t.param
			}
			p.pendingLevel = f.level
		}
	case "pnlvlblt":
		p.cur().list = Bullet
		p.pendingList = Bullet
	case "pnlvlbody":
		p.cur().list = Numbered
		p.pendingList = Numbered
	case "bullet":
		p.pendingList = Bullet
	case "line":
		// A line break inside the paragraph: the text goes on, on a new line.
		p.write("\n")
	case "par":
		p.endParagraph()
	case "tab":
		p.write("\t")
	case "u":
		if t.hasP {
			r := t.param
			if r < 0 {
				r += 65536
			}
			p.write(string(rune(r)))
			p.pendingSki = p.ucSkip
		}
	case "trowd":
		p.startRow()
	case "trhdr":
		p.head = true
	case "cellx":
		if t.hasP {
			p.widths = append(p.widths, t.param)
		}
	case "cell":
		p.endCell()
	case "row":
		p.endRow()
	case "field":
		p.cur().field = 1
		p.cur().skip = false
	case "fldinst":
		p.cur().field = 1
		p.cur().skip = false
		p.fieldInst.Reset()
	case "fldrslt":
		// The result is the visible text; the instruction before it is not.
		p.cur().field = 2
		p.cur().skip = false
		p.cur().link = hyperlinkTarget(p.fieldInst.String())
	}
}

// hyperlinkTarget reads the address out of a field instruction. Only HYPERLINK
// is understood; other fields keep their result text without a target.
func hyperlinkTarget(inst string) string {
	s := strings.TrimSpace(inst)
	const kw = "HYPERLINK"
	i := strings.Index(strings.ToUpper(s), kw)
	if i < 0 {
		return ""
	}
	rest := strings.TrimSpace(s[i+len(kw):])
	if !strings.HasPrefix(rest, `"`) {
		// HYPERLINK without a quoted address: take the first word.
		if fields := strings.Fields(rest); len(fields) > 0 {
			return strings.Trim(fields[0], `"`)
		}
		return ""
	}
	end := strings.Index(rest[1:], `"`)
	if end < 0 {
		return ""
	}
	return rest[1 : end+1]
}

// endParagraph closes the runs collected so far into a block.
func (p *parser) endParagraph() {
	// A table without an open row or cell is finished, and its block belongs
	// before this paragraph: otherwise the whole table would be appended at the
	// end of the document.
	if len(p.rows) > 0 && p.row == nil && !p.cellOpen {
		p.endTable()
	}
	if p.cellOpen {
		// A paragraph mark inside a cell: the text belongs to the cell, which
		// ends with \cell, so it must not be dropped as an empty paragraph.
		p.cell = append(p.cell, p.run...)
		p.run = nil
		return
	}
	f := p.cur()
	level := headingLevel(p.styles[f.styleNo])
	listLevel, listKind := f.level, f.list
	if listKind == NoList {
		listKind = p.pendingList
	}
	if listLevel == 0 {
		listLevel = p.pendingLevel
	}
	if listKind != NoList && listLevel == 0 {
		// The document says "list item" but not how deep: that is the first
		// level, the same assumption the old reader made for \pntext bullets.
		listLevel = 720
	}
	p.pendingList, p.pendingLevel = NoList, 0
	if listLevel > 0 && listKind != NoList {
		listLevel = min(max((listLevel+719)/720, 1), 6)
	} else {
		listLevel = 0
	}
	runs := trimRuns(p.run)
	p.run = nil
	if len(runs) == 0 {
		return
	}
	b := Block{Runs: runs, Align: f.align}
	switch {
	case listLevel > 0 && listKind != NoList:
		b.Kind, b.Level, b.List = ListItem, listLevel, listKind
	case level > 0:
		b.Kind, b.Level = Heading, level
	default:
		b.Kind = Paragraph
	}
	p.blocks = append(p.blocks, b)
}

func trimRuns(runs []Run) []Run {
	for len(runs) > 0 {
		last := &runs[len(runs)-1]
		last.Text = strings.TrimRight(last.Text, "\n")
		if strings.TrimSpace(last.Text) != "" {
			break
		}
		runs = runs[:len(runs)-1]
	}
	if len(runs) > 0 {
		runs[0].Text = strings.TrimLeft(runs[0].Text, "\n")
	}
	return runs
}

func (p *parser) startRow() {
	if p.row == nil {
		p.row = [][]Run{}
		p.widths = nil
	}
	p.cell = nil
	p.cellOpen = true
}

func (p *parser) endCell() {
	p.row = append(p.row, trimRuns(p.cell))
	// \cell closes the cell, and the next text belongs to the next cell of the
	// same row - RTF has no opening control word for a cell.
	p.cell = nil
	p.cellOpen = true
}

func (p *parser) endRow() {
	// A row whose last cell was not closed by \cell ends here.
	if p.cellOpen && len(p.cell) > 0 {
		p.endCell()
	}
	if len(p.row) > 0 {
		p.rows = append(p.rows, p.row)
	}
	p.row = nil
	p.cell = nil
	p.cellOpen = false
}

func (p *parser) endTable() {
	if len(p.rows) == 0 {
		p.rows, p.row, p.cell, p.cellOpen, p.widths, p.head = nil, nil, nil, false, nil, false
		return
	}
	p.blocks = append(p.blocks, Block{
		Kind:   Table,
		Rows:   p.rows,
		Head:   p.head,
		Widths: p.widths,
	})
	p.rows, p.row, p.cell, p.cellOpen, p.widths, p.head = nil, nil, nil, false, nil, false
}

// stylesheet collects the style names, so "\s1" can become a heading.
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
