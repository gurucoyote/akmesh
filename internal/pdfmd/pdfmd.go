// Package pdfmd gives the text of a PDF file.
//
// A PDF is not a text format: it is a set of objects, and the text is inside a
// content stream that has to be decompressed and interpreted. This reader does
// that with the standard library only (compress/zlib, compress/flate):
//
//   - objects are found by scanning for "N G obj ... endobj", which also works
//     for a file whose cross-reference table is stale or missing;
//   - a stream with /Filter /FlateDecode is inflated;
//   - pages are taken in the order of the page tree, not the order of objects;
//   - the content stream is tokenized, and BT/Td/TD/T*/TL/Tm start a new line
//     while Tj and TJ add text;
//   - a byte becomes a character through the font: /ToUnicode when it exists,
//     the /Encoding otherwise, two bytes per character for a Type0 font.
//
// What it cannot do, and does not pretend to: a scanned page has no text at all
// (there are only images), an encrypted document fails with a clear error, and
// a composite font without /ToUnicode and without an identity map comes out as
// wrong characters rather than as a guess. Structure - headings, columns,
// tables - is not recovered: a PDF does not carry it, only positions.
package pdfmd

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Convert returns the text of a PDF, one paragraph per line.
func Convert(data []byte) (string, error) {
	if !bytes.Contains(data[:min(len(data), 1024)], []byte("%PDF")) {
		return "", fmt.Errorf("not a PDF (no %%PDF header)")
	}
	if bytes.Contains(data, []byte("/Encrypt")) {
		return "", fmt.Errorf("encrypted PDF, the text cannot be read without the password")
	}
	doc := parseObjects(data)
	if len(doc) == 0 {
		return "", fmt.Errorf("no objects found")
	}
	pages := pageObjects(doc)
	if len(pages) == 0 {
		return "", fmt.Errorf("no pages found")
	}
	var out strings.Builder
	for _, page := range pages {
		text := pageText(doc, page)
		if strings.TrimSpace(text) == "" {
			continue
		}
		if out.Len() > 0 {
			out.WriteString("\n")
		}
		out.WriteString(strings.TrimRight(text, " \t\n"))
		out.WriteString("\n")
	}
	text := out.String()
	// Collapse runs of empty lines that a column or a table leaves behind.
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return text, nil
}

// object is one indirect object: its dictionary (or value) and its stream.
type object struct {
	dict   string
	stream []byte
}

// parseObjects scans the file for objects. The scan also covers files whose
// xref table points nowhere, which is why it is preferred over following it.
//
// An object runs from its "N G obj" header to the next header, not to the next
// "endobj": binary data inside a stream may contain that word, and cutting
// there loses the objects behind it.
func parseObjects(data []byte) map[string]*object {
	out := map[string]*object{}
	type header struct {
		key   string
		start int
		body  int
	}
	var headers []header
	s := string(data)
	for i := 0; i < len(s); {
		idx := strings.Index(s[i:], " obj")
		if idx < 0 {
			break
		}
		at := i + idx
		j := at
		for j > 0 && (s[j-1] == ' ' || isDigit(s[j-1])) {
			j--
		}
		key := strings.TrimSpace(s[j:at])
		if key != "" && strings.Contains(key, " ") {
			headers = append(headers, header{key: key, start: at + 4, body: at + 4})
		}
		i = at + 4
	}
	for i, h := range headers {
		end := len(s)
		if i+1 < len(headers) {
			end = headers[i+1].start
		}
		raw := s[h.body:end]
		if e := strings.LastIndex(raw, "endobj"); e >= 0 {
			raw = raw[:e]
		}
		obj := &object{dict: raw}
		if sIdx := strings.Index(raw, "stream"); sIdx >= 0 {
			body := raw[sIdx+len("stream"):]
			body = strings.TrimPrefix(body, "\r\n")
			body = strings.TrimPrefix(body, "\n")
			if e := strings.LastIndex(body, "endstream"); e >= 0 {
				body = body[:e]
			}
			obj.stream = []byte(body)
			obj.dict = raw[:sIdx]
		}
		out[h.key] = obj
	}
	return out
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// squash removes the whitespace of a dictionary, so that "/Type /Page" and
// "/Type/Page" are the same thing. Both spellings occur in the wild, and a
// reader that only knows one of them finds no pages at all.
func squash(s string) string {
	return strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(s)
}

// hasTag says whether a dictionary contains a tag such as "/Type/Page".
func hasTag(dict, tag string) bool {
	return strings.Contains(squash(dict), tag)
}

// decoded returns the stream of an object, inflated when it is Flate encoded.
func decoded(obj *object) []byte {
	if obj == nil || len(obj.stream) == 0 {
		return nil
	}
	if !hasTag(obj.dict, "/FlateDecode") {
		return obj.stream
	}
	if zr, err := zlib.NewReader(bytes.NewReader(obj.stream)); err == nil {
		if b, err := io.ReadAll(zr); err == nil {
			return b
		}
	}
	// Some writers store raw deflate data without the zlib wrapper.
	if fr := flate.NewReader(bytes.NewReader(obj.stream)); fr != nil {
		if b, err := io.ReadAll(fr); err == nil {
			return b
		}
	}
	return nil
}

// value returns the raw value that follows "/key" in a dictionary.
func value(dict, key string) string {
	idx := strings.Index(dict, "/"+key)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimLeft(dict[idx+len(key)+1:], " \t\r\n")
	switch {
	case strings.HasPrefix(rest, "<<"):
		depth := 0
		for i := 0; i < len(rest)-1; {
			switch {
			case rest[i] == '<' && rest[i+1] == '<':
				depth++
				i += 2
			case rest[i] == '>' && rest[i+1] == '>':
				depth--
				i += 2
				if depth == 0 {
					return rest[:i]
				}
			default:
				i++
			}
		}
		return rest
	case strings.HasPrefix(rest, "["):
		depth := 0
		for i := 0; i < len(rest); i++ {
			if rest[i] == '[' {
				depth++
			} else if rest[i] == ']' {
				depth--
				if depth == 0 {
					return rest[:i+1]
				}
			}
		}
		return rest
	case strings.HasPrefix(rest, "/"):
		end := 0
		for end < len(rest) && !strings.ContainsRune(" \t\r\n/>[]", rune(rest[end])) {
			end++
		}
		return rest[:end]
	default:
		first, next := nextToken(rest, 0)
		if !isNumber(first) {
			return first
		}
		// An indirect reference: "5 0 R" is one value, and stopping after the
		// number loses the object it points at.
		second, next := nextToken(rest, next)
		third, _ := nextToken(rest, next)
		if isNumber(first) && isNumber(second) && third == "R" {
			return first + " " + second + " R"
		}
		if first != "" {
			return first
		}
		return ""
	}
}

// nextToken reads one token from s at i, where a token ends at whitespace or at
// one of the delimiters of the format.
func nextToken(s string, i int) (string, int) {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n') {
		i++
	}
	start := i
	for i < len(s) && !strings.ContainsRune(" \t\r\n/[]<>()", rune(s[i])) {
		i++
	}
	return s[start:i], i
}

// refsIn returns the object keys referenced in a raw value such as
// "[3 0 R 4 0 R]".
func refsIn(raw string) []string {
	var out []string
	// The delimiters are separated from the numbers first: in "[13 0 R]" the
	// first field would otherwise be "[13", which is not a number.
	clean := strings.NewReplacer("[", " ", "]", " ", "<", " ", ">", " ", "\t", " ").Replace(raw)
	f := strings.Fields(clean)
	for i := 0; i+2 < len(f); i++ {
		if f[i+2] == "R" && isNumber(f[i]) && isNumber(f[i+1]) {
			out = append(out, f[i]+" "+f[i+1])
			i += 2
		}
	}
	return out
}

// pageObjects returns the page objects in the order of the page tree.
func pageObjects(d map[string]*object) []*object {
	// Catalog first.
	var rootKey string
	for key, obj := range d {
		if hasTag(obj.dict, "/Type/Catalog") {
			rootKey = key
			break
		}
	}
	var pages []*object
	seen := map[string]bool{}
	var walk func(key string, depth int)
	walk = func(key string, depth int) {
		if depth > 32 || seen[key] {
			return
		}
		seen[key] = true
		obj := d[key]
		if obj == nil {
			return
		}
		if hasTag(obj.dict, "/Type/Page") && !hasTag(obj.dict, "/Type/Pages") {
			pages = append(pages, obj)
			return
		}
		for _, kid := range refsIn(value(obj.dict, "Kids")) {
			walk(kid, depth+1)
		}
	}
	if rootKey != "" {
		tree := refsIn(value(d[rootKey].dict, "Pages"))
		if len(tree) > 0 {
			walk(tree[0], 0)
		} else if raw := value(d[rootKey].dict, "Pages"); raw != "" {
			// A single page as the root's /Pages is unusual but possible.
			if obj := d[raw]; obj != nil {
				pages = append(pages, obj)
			}
		}
	}
	if len(pages) == 0 {
		// Fall back to the order in the file.
		for _, key := range sortedKeys(d) {
			obj := d[key]
			if hasTag(obj.dict, "/Type/Page") && !hasTag(obj.dict, "/Type/Pages") {
				pages = append(pages, obj)
			}
		}
	}
	return pages
}

// sortedKeys returns the object keys in numeric order, so that a fallback
// traversal at least follows the file.
func sortedKeys(d map[string]*object) []string {
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keyNum(keys[j]) < keyNum(keys[j-1]); j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// keyNum is the object number of a key like "12 0".
func keyNum(k string) int {
	f := strings.Fields(k)
	if len(f) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(f[0])
	return n
}

// pageText reads the content stream of one page.
func pageText(d map[string]*object, page *object) string {
	contentKey := value(page.dict, "Contents")
	var content []byte
	if strings.HasPrefix(contentKey, "[") {
		for _, ref := range refsIn(contentKey) {
			content = append(content, decoded(d[ref])...)
			content = append(content, '\n')
		}
	} else if refs := refsIn(contentKey); len(refs) > 0 {
		content = decoded(d[refs[0]])
	}

	resRaw := value(page.dict, "Resources")
	resources := resRaw
	if !strings.HasPrefix(resRaw, "<<") {
		if refs := refsIn(resRaw); len(refs) > 0 {
			if obj := d[refs[0]]; obj != nil {
				resources = obj.dict
			}
		}
	}
	return textOfPage(d, content, readFonts(d, resources), resources)
}

// readFonts maps a resource name to its font. The name to object mapping lives
// in the /Font dictionary of the resources, not in the font object itself.
func readFonts(d map[string]*object, resources string) map[string]*font {
	out := map[string]*font{}
	fontRaw := value(resources, "Font")
	dict := fontRaw
	if !strings.HasPrefix(fontRaw, "<<") {
		if refs := refsIn(fontRaw); len(refs) > 0 {
			if obj := d[refs[0]]; obj != nil {
				dict = obj.dict
			}
		}
	}
	for name, ref := range nameRefs(dict) {
		if obj := d[ref]; obj != nil {
			out[name] = readFont(d, obj)
		}
	}
	return out
}

// nameRefs returns the "/Name N G R" pairs of a font dictionary. The dictionary
// delimiters are separated from the names first, or "<</F1 9 0 R>>" would look
// like one token.
func nameRefs(dict string) map[string]string {
	out := map[string]string{}
	t := strings.NewReplacer("<<", " ", ">>", " ", "/", " /").Replace(dict)
	f := strings.Fields(t)
	for i := 0; i < len(f); i++ {
		name := f[i]
		if !strings.HasPrefix(name, "/") || len(name) < 2 {
			continue
		}
		if i+3 < len(f) && isNumber(f[i+1]) && isNumber(f[i+2]) && f[i+3] == "R" {
			out[name[1:]] = f[i+1] + " " + f[i+2]
			i += 3
		}
	}
	return out
}

// font is what the reader needs from a font object.
type font struct {
	twoByte  bool
	toUni    map[uint32]rune
	widths   map[uint32]float64 // in thousandths of the font size
	defaultW float64
	encoding string
}

// readFont reads the encoding and the character map of a font.
func readFont(d map[string]*object, obj *object) *font {
	f := &font{encoding: value(obj.dict, "Encoding")}
	if hasTag(obj.dict, "/Type0") {
		f.twoByte = true
	}
	f.widths, f.defaultW = readWidths(d, obj)
	if refs := refsIn(value(obj.dict, "ToUnicode")); len(refs) > 0 {
		if cm := d[refs[0]]; cm != nil {
			f.toUni = parseCMap(string(decoded(cm)))
		}
	}
	return f
}

// readWidths returns the advance of each character, which is what tells a word
// space from the gap a writer leaves between two glyphs.
func readWidths(d map[string]*object, obj *object) (map[uint32]float64, float64) {
	out := map[uint32]float64{}
	def := 0.0
	if dw := value(obj.dict, "DW"); dw != "" {
		if v, err := strconv.ParseFloat(dw, 64); err == nil {
			def = v
		}
	}
	// A Type0 font keeps its widths in the descendant font.
	raw := value(obj.dict, "W")
	if raw == "" {
		if refs := refsIn(value(obj.dict, "DescendantFonts")); len(refs) > 0 {
			if desc := d[refs[0]]; desc != nil {
				raw = value(desc.dict, "W")
				if dw := value(desc.dict, "DW"); dw != "" {
					if v, err := strconv.ParseFloat(dw, 64); err == nil {
						def = v
					}
				}
			}
		}
	}
	if raw != "" {
		parseW(raw, out)
	}
	// A simple font lists the widths of a range of codes.
	if raw == "" {
		first, _ := strconv.Atoi(value(obj.dict, "FirstChar"))
		if w := value(obj.dict, "Widths"); strings.HasPrefix(w, "[") {
			f := strings.Fields(strings.Trim(w, "[]"))
			for i, tok := range f {
				if v, err := strconv.ParseFloat(tok, 64); err == nil {
					out[uint32(first+i)] = v
				}
			}
		}
	}
	return out, def
}

// parseW reads a CID width array: "c [w w w]" or "c1 c2 w".
func parseW(raw string, out map[uint32]float64) {
	f := strings.Fields(strings.Trim(raw, "[]"))
	for i := 0; i < len(f); i++ {
		if strings.HasPrefix(f[i], "[") || f[i] == "[" {
			continue
		}
		c, err := strconv.Atoi(f[i])
		if err != nil {
			continue
		}
		if i+1 < len(f) && strings.HasPrefix(f[i+1], "[") {
			list := strings.Trim(f[i+1], "[]")
			w := 0
			for _, tok := range strings.Fields(list) {
				if v, e := strconv.ParseFloat(tok, 64); e == nil {
					out[uint32(c+w)] = v
					w++
				}
			}
			for i+1 < len(f) && !strings.HasSuffix(f[i+1], "]") {
				i++
			}
			i++
			continue
		}
		if i+2 < len(f) {
			hi, e1 := strconv.Atoi(f[i+1])
			w, e2 := strconv.ParseFloat(f[i+2], 64)
			if e1 == nil && e2 == nil {
				for cid := c; cid <= hi && cid-c < 65536; cid++ {
					out[uint32(cid)] = w
				}
				i += 2
			}
		}
	}
}

// width returns the advance of a string in text space.
func (f *font) width(b []byte, size float64) float64 {
	if f == nil {
		return 0
	}
	total := 0.0
	if f.twoByte {
		for i := 0; i+1 < len(b); i += 2 {
			code := uint32(b[i])<<8 | uint32(b[i+1])
			if w, ok := f.widths[code]; ok {
				total += w
			} else {
				total += f.defaultW
			}
		}
	} else {
		for _, c := range b {
			if w, ok := f.widths[uint32(c)]; ok {
				total += w
			} else {
				total += f.defaultW
			}
		}
	}
	if total == 0 {
		// No width table: a rough average is better than none.
		return float64(len(b)) * 0.5 * size
	}
	return total / 1000 * size
}

// decode turns the bytes of a string operand into characters.
func (f *font) decode(b []byte) string {
	if f == nil {
		return string(b)
	}
	var out strings.Builder
	if f.twoByte {
		for i := 0; i+1 < len(b); i += 2 {
			code := uint32(b[i])<<8 | uint32(b[i+1])
			if r, ok := f.toUni[code]; ok {
				out.WriteRune(r)
			} else if r := rune(code); r >= 32 {
				out.WriteRune(r)
			}
		}
		return out.String()
	}
	for _, c := range b {
		if r, ok := f.toUni[uint32(c)]; ok {
			out.WriteRune(r)
			continue
		}
		out.WriteRune(cp1252(c))
	}
	return out.String()
}

// parseCMap reads the bfchar and bfrange sections of a ToUnicode map.
func parseCMap(s string) map[uint32]rune {
	out := map[uint32]rune{}
	readHex := func(tok string) (uint32, bool) {
		tok = strings.Trim(tok, "<>")
		if tok == "" {
			return 0, false
		}
		n, err := strconv.ParseUint(tok, 16, 32)
		if err != nil {
			return 0, false
		}
		return uint32(n), true
	}
	lines := strings.Split(s, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case strings.Contains(line, "beginbfchar"):
			for i++; i < len(lines) && !strings.Contains(lines[i], "endbfchar"); i++ {
				f := strings.Fields(strings.TrimSpace(lines[i]))
				if len(f) < 2 {
					continue
				}
				src, ok1 := readHex(f[0])
				dst, ok2 := readHex(f[1])
				if ok1 && ok2 {
					out[src] = rune(dst)
				}
			}
		case strings.Contains(line, "beginbfrange"):
			for i++; i < len(lines) && !strings.Contains(lines[i], "endbfrange"); i++ {
				f := strings.Fields(strings.TrimSpace(lines[i]))
				if len(f) < 3 {
					continue
				}
				lo, ok1 := readHex(f[0])
				hi, ok2 := readHex(f[1])
				if !ok1 || !ok2 {
					continue
				}
				if strings.HasPrefix(f[2], "[") {
					// A list of destinations, one per code.
					code := lo
					for _, tok := range f[2:] {
						tok = strings.Trim(tok, "[]")
						if d, ok := readHex(tok); ok {
							out[code] = rune(d)
							code++
						}
					}
					continue
				}
				dst, ok := readHex(f[2])
				if !ok {
					continue
				}
				for code := lo; code <= hi && code-lo < 65536; code++ {
					out[code] = rune(dst + (code - lo))
				}
			}
		}
	}
	return out
}

// textOf interprets one content stream.
//
// The text matrix matters: "Td 1 0" between two pieces is how many writers put
// one glyph after another, so a space may only be inserted where the gap is
// wider than a word space. Without that, a heading comes out as "W o r l d".
func textOf(content []byte, fonts map[string]*font) string {
	return textOfPage(nil, content, fonts, "")
}

// textOfPage reads one page with the document at hand, so that the text inside
// a form XObject is read too.
func textOfPage(d map[string]*object, content []byte, fonts map[string]*font, resources string) string {
	var lines []textLine
	var rects []rect
	if d == nil {
		lines, rects = textOfLines(content, fonts)
	} else {
		r := &contentReader{
			fonts:     fonts,
			doc:       d,
			m:         identity(),
			xobjects:  xobjectsOf(d, resources),
			lineSizes: map[float64]int{},
		}
		r.run(content, 0)
		r.flushLine()
		lines, rects = r.lines, r.rects
	}
	// A page whose cells are drawn as rectangles is a table; the text that sits
	// inside those cells is the table, and the rest of the page order would only
	// repeat it.
	if t := tableFromRects(rects, lines); t != "" {
		return t
	}
	return renderLines(lines)
}

// matrix is the current transformation matrix of a content stream. Text and
// rectangles are both placed through it, so a reader that ignores it cannot put
// a piece of text into the cell it belongs to.
type matrix struct {
	a, b, c, d, e, f float64
}

func identity() matrix { return matrix{a: 1, d: 1} }

// mul returns m applied after n ("cm" concatenates).
func (m matrix) mul(n matrix) matrix {
	return matrix{
		a: m.a*n.a + m.b*n.c,
		b: m.a*n.b + m.b*n.d,
		c: m.c*n.a + m.d*n.c,
		d: m.c*n.b + m.d*n.d,
		e: m.e*n.a + m.f*n.c + n.e,
		f: m.e*n.b + m.f*n.d + n.f,
	}
}

// apply puts a point of the current space into page coordinates.
func (m matrix) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

// rect is a filled or stroked rectangle in page coordinates.
type rect struct {
	x, y, w, h float64
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// textOfLines reads a content stream into lines with their positions; the
// rendering is a separate step, because a table needs the whole page.
func textOfLines(content []byte, fonts map[string]*font) ([]textLine, []rect) {
	r := &contentReader{fonts: fonts, m: identity(), lineSizes: map[float64]int{}}
	r.run(content, 0)
	r.flushLine()
	return r.lines, r.rects
}

// contentReader walks one content stream and collects the text and the
// rectangles in page coordinates. It is a struct rather than a function because
// a form XObject is a content stream of its own: it has to be read with the
// matrix it is placed with, and its text belongs to the same page.
type contentReader struct {
	fonts map[string]*font
	doc   map[string]*object
	m     matrix

	lines []textLine
	rects []rect

	cur          *font
	size         float64
	tx, ty       float64
	lastStart    float64
	lastWidth    float64
	buf          bytes.Buffer
	pieces       []piece
	lineSizes    map[float64]int
	lineHasText  bool
	spacePending bool

	xobjects    map[string]string
	pendingRect *rect
}

func (r *contentReader) newLine() {
	if r.lineHasText {
		r.flushLine()
		r.lineHasText = false
	}
	r.spacePending = false
}

func (r *contentReader) flushLine() {
	t := strings.TrimRight(r.buf.String(), " \t")
	if strings.TrimSpace(t) != "" {
		size := r.size
		best := 0
		for sz, n := range r.lineSizes {
			if n > best {
				best, size = n, sz
			}
		}
		y := r.ty
		if len(r.pieces) > 0 {
			y = r.pieces[0].y
		}
		r.lines = append(r.lines, textLine{text: t, size: size, y: y, pieces: r.pieces})
	}
	r.buf.Reset()
	r.pieces = nil
	r.lineSizes = map[float64]int{}
}

func (r *contentReader) advance(b []byte) float64 {
	if w := r.cur.width(b, r.size); w > 0 {
		return w
	}
	return float64(len([]rune(string(b)))) * 0.5 * r.size
}

func (r *contentReader) emit(b []byte) {
	if len(b) == 0 {
		return
	}
	text := string(b)
	if r.cur != nil {
		text = r.cur.decode(b)
	}
	if text == "" {
		return
	}
	gap := r.tx - (r.lastStart + r.lastWidth)
	if r.lineHasText && (r.spacePending || gap > 0.35*r.size) {
		if !strings.HasSuffix(r.buf.String(), " ") && !strings.HasPrefix(text, " ") {
			r.buf.WriteString(" ")
		}
	}
	r.spacePending = false
	r.buf.WriteString(text)
	px, py := r.m.apply(r.tx, r.ty)
	// A run with a wide run of spaces inside is how many writers lay out a row
	// of cells: the gap is in the string, not in the positions.
	if parts := splitWideGaps(text); len(parts) > 1 {
		offset := 0.0
		for _, part := range parts {
			x := px + offset*r.size/1000
			if part.text != "" {
				r.pieces = append(r.pieces, piece{text: part.text, x: x, y: py, w: advancePart(part.text, r.size, r.m)})
			}
			offset += part.offset
		}
	} else {
		r.pieces = append(r.pieces, piece{text: text, x: px, y: py, w: r.advance(b)})
	}
	r.lineSizes[r.size] += len([]rune(text))
	r.lastStart = r.tx
	r.lastWidth = r.advance(b)
	r.lineHasText = true
}

// run interprets one content stream. depth keeps a form inside a form inside a
// page from running away.
func (r *contentReader) run(content []byte, depth int) {
	if depth > 4 {
		return
	}
	stack := []matrix{}
	saved := r.m
	defer func() { r.m = saved }()
	for _, t := range tokens(string(content)) {
		switch t.op {
		case "q":
			stack = append(stack, r.m)
		case "Q":
			if n := len(stack); n > 0 {
				r.m = stack[n-1]
				stack = stack[:n-1]
			}
		case "cm":
			if len(t.nums) >= 6 {
				r.m = r.m.mul(matrix{a: t.nums[0], b: t.nums[1], c: t.nums[2], d: t.nums[3], e: t.nums[4], f: t.nums[5]})
			}
		case "re":
			if len(t.nums) >= 4 {
				x1, y1 := r.m.apply(t.nums[0], t.nums[1])
				x2, y2 := r.m.apply(t.nums[0]+t.nums[2], t.nums[1]+t.nums[3])
				r.pendingRect = &rect{x: minf(x1, x2), y: minf(y1, y2), w: absf(x2 - x1), h: absf(y2 - y1)}
			}
		case "f", "F", "f*", "S", "s", "B", "B*", "b", "b*":
			if r.pendingRect != nil {
				r.rects = append(r.rects, *r.pendingRect)
				r.pendingRect = nil
			}
		case "n", "W", "W*":
			r.pendingRect = nil
		case "Do":
			r.doXObject(t.name, depth)
		case "BT":
			r.tx, r.ty = 0, 0
			r.lastStart, r.lastWidth = 0, 0
		case "Tf":
			if len(t.nums) > 0 {
				r.size = t.nums[0]
			}
			r.cur = r.fonts[t.name]
		case "Td", "TD":
			if len(t.nums) >= 2 {
				r.tx += t.nums[0]
				r.ty += t.nums[1]
				if t.nums[1] != 0 {
					r.newLine()
					r.lastStart, r.lastWidth = r.tx, 0
				}
			}
		case "T*":
			r.newLine()
		case "Tm":
			if len(t.nums) >= 6 {
				if t.nums[5] != r.ty && r.lineHasText {
					r.newLine()
				}
				r.tx, r.ty = t.nums[4], t.nums[5]
				r.lastStart, r.lastWidth = r.tx, 0
			}
		case "Tj":
			r.emit(t.str)
		case "TJ":
			for _, piece := range t.pieces {
				r.emit(piece)
			}
			for _, n := range t.nums {
				// A large negative adjustment is a word space, a small one is
				// kerning inside a word.
				if n <= -100 {
					r.spacePending = true
				}
			}
		case "'", `"`:
			r.newLine()
			r.lastStart, r.lastWidth = r.tx, 0
			r.emit(t.str)
		}
	}
}

// pendingRect is the rectangle drawn last, waiting for the paint operator.
var _ = 0

func (r *contentReader) doXObject(name string, depth int) {
	if name == "" || r.doc == nil {
		return
	}
	ref := r.xobjects[name]
	if ref == "" {
		return
	}
	obj := r.doc[ref]
	if obj == nil || !hasTag(obj.dict, "/Subtype/Form") {
		return
	}
	// The form is placed with its own matrix, applied after the current one.
	fm := identity()
	if raw := value(obj.dict, "Matrix"); strings.HasPrefix(raw, "[") {
		f := strings.Fields(strings.NewReplacer("[", " ", "]", " ").Replace(raw))
		if len(f) >= 6 {
			nums := make([]float64, 6)
			ok := true
			for i := range nums {
				v, err := strconv.ParseFloat(f[i], 64)
				if err != nil {
					ok = false
					break
				}
				nums[i] = v
			}
			if ok {
				fm = matrix{a: nums[0], b: nums[1], c: nums[2], d: nums[3], e: nums[4], f: nums[5]}
			}
		}
	}
	// A form carries its own resources, and they win over the page's.
	fonts := r.fonts
	var xobjects map[string]string
	if raw := value(obj.dict, "Resources"); raw != "" {
		res := raw
		if !strings.HasPrefix(raw, "<<") {
			if refs := refsIn(raw); len(refs) > 0 {
				if resObj := r.doc[refs[0]]; resObj != nil {
					res = resObj.dict
				}
			}
		}
		if f := readFonts(r.doc, res); len(f) > 0 {
			fonts = f
		}
		xobjects = xobjectsOf(r.doc, res)
	}
	body := decoded(obj)
	inner := &contentReader{
		fonts:     fonts,
		doc:       r.doc,
		m:         r.m.mul(fm),
		xobjects:  xobjects,
		lineSizes: map[float64]int{},
	}
	if inner.xobjects == nil {
		inner.xobjects = r.xobjects
	}
	// The text of a form belongs to the line it sits on, so an open line of the
	// page is closed first and the form's lines follow.
	r.newLine()
	inner.run(body, depth+1)
	inner.flushLine()
	r.lines = append(r.lines, inner.lines...)
	r.rects = append(r.rects, inner.rects...)
}

// xobjectsOf returns the name to object mapping of a resource dictionary.
func xobjectsOf(d map[string]*object, resources string) map[string]string {
	raw := value(resources, "XObject")
	if raw == "" {
		return nil
	}
	dict := raw
	if !strings.HasPrefix(raw, "<<") {
		if refs := refsIn(raw); len(refs) > 0 {
			if obj := d[refs[0]]; obj != nil {
				dict = obj.dict
			}
		}
	}
	out := nameRefs(dict)
	if len(out) == 0 {
		return nil
	}
	return out
}

// tableFromRects builds a table from cells drawn as rectangles. A cell takes the
// text whose middle falls inside it, which is how a calendar or an invoice is
// laid out. Only rectangles that could be a cell are used: a page background or
// a decorative band is too large for that.
func tableFromRects(rects []rect, lines []textLine) string {
	var cells []rect
	for _, r := range rects {
		if r.w < 6 || r.h < 6 || r.h > 80 || r.w > 300 {
			continue
		}
		cells = append(cells, r)
	}
	if len(cells) < 4 {
		return ""
	}
	// Group by row: the y of a cell within a few points is the same row.
	sortRects(cells, func(a, b rect) bool { return a.y > b.y })
	type row struct {
		y   float64
		col []rect
	}
	var rows []row
	for _, r := range cells {
		placed := false
		for i := range rows {
			if absf(rows[i].y-r.y) <= 2 {
				rows[i].col = append(rows[i].col, r)
				placed = true
				break
			}
		}
		if !placed {
			rows = append(rows, row{y: r.y, col: []rect{r}})
		}
	}
	// Columns: the distinct left edges across all rows.
	var xs []float64
	for _, r := range cells {
		found := false
		for _, x := range xs {
			if absf(x-r.x) <= 3 {
				found = true
				break
			}
		}
		if !found {
			xs = append(xs, r.x)
		}
	}
	if len(xs) < 2 || len(rows) < 2 {
		return ""
	}
	sortFloats(xs)
	table := make([][]string, len(rows))
	filled := 0
	for i, rw := range rows {
		table[i] = make([]string, len(xs))
		for _, c := range rw.col {
			col := 0
			for k, x := range xs {
				if absf(x-c.x) <= 3 {
					col = k
				}
			}
			text := textInRect(c, lines)
			if text != "" {
				filled++
			}
			if table[i][col] == "" {
				table[i][col] = text
			}
		}
	}
	if filled < len(rows) {
		return ""
	}
	var out strings.Builder
	for i, rw := range table {
		out.WriteString("|")
		for _, c := range rw {
			out.WriteString(" " + strings.ReplaceAll(c, "|", "\\|") + " |")
		}
		out.WriteString("\n")
		if i == 0 {
			out.WriteString("|")
			for range rw {
				out.WriteString(" --- |")
			}
			out.WriteString("\n")
		}
	}
	return out.String()
}

// textInRect returns the text whose middle lies inside the cell.
func textInRect(c rect, lines []textLine) string {
	var parts []string
	for _, l := range lines {
		for _, pc := range l.pieces {
			mx := pc.x + pc.w/2
			my := l.y
			if mx >= c.x && mx <= c.x+c.w && my >= c.y && my <= c.y+c.h {
				parts = append(parts, pc.text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func sortRects(v []rect, less func(a, b rect) bool) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && less(v[j], v[j-1]); j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

func sortFloats(v []float64) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// textLine is one line of the page: its text, the size most of it was set in,
// its y position and the runs it was made of. Columns are found by comparing
// those positions; a PDF carries nothing else that says "table".
type textLine struct {
	text   string
	size   float64
	y      float64
	pieces []piece
}

// piece is one run of text and where it stands on the page.
type piece struct {
	text string
	x    float64
	y    float64
	w    float64
}

// sortedPieces returns the runs of a line from left to right.
func sortedPieces(l textLine) []piece {
	out := make([]piece, len(l.pieces))
	copy(out, l.pieces)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].x < out[j-1].x; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// cellGap is the distance that separates two cells: a word space is much
// narrower than that.
func cellGap(l textLine) float64 {
	if l.size <= 0 {
		return 9
	}
	return 0.9 * l.size
}

// startsOf returns where the cells of a line begin.
func startsOf(l textLine) []float64 {
	sorted := sortedPieces(l)
	if len(sorted) == 0 {
		return nil
	}
	gap := cellGap(l)
	out := []float64{sorted[0].x}
	lastEnd := sorted[0].x + sorted[0].w
	for _, pc := range sorted[1:] {
		if pc.x-lastEnd > gap {
			out = append(out, pc.x)
		}
		lastEnd = pc.x + pc.w
	}
	return out
}

// cellsOf splits a line into its cells at the gaps found by startsOf.
func cellsOf(l textLine) []string {
	sorted := sortedPieces(l)
	if len(sorted) == 0 {
		return nil
	}
	gap := cellGap(l)
	var cells []string
	cur := sorted[0].text
	lastEnd := sorted[0].x + sorted[0].w
	for _, pc := range sorted[1:] {
		if pc.x-lastEnd > gap {
			cells = append(cells, strings.TrimSpace(cur))
			cur = pc.text
		} else {
			cur += pc.text
		}
		lastEnd = pc.x + pc.w
	}
	return append(cells, strings.TrimSpace(cur))
}

// tableGroup returns the rows that form a table starting at i: at least two
// rows, the same number of cells, and cell starts that line up within 3pt. One
// row is not a table - a line of text with a wide gap is still a line.
func tableGroup(lines []textLine, i int) []textLine {
	if i >= len(lines) || len(lines[i].pieces) < 2 {
		return nil
	}
	want := startsOf(lines[i])
	if len(want) < 2 || !shortCells(cellsOf(lines[i])) {
		return nil
	}
	rows := []textLine{lines[i]}
	for j := i + 1; j < len(lines); j++ {
		got := startsOf(lines[j])
		if len(got) != len(want) || !shortCells(cellsOf(lines[j])) {
			break
		}
		aligned := true
		for k := range got {
			if absf(got[k]-want[k]) > 3 {
				aligned = false
				break
			}
		}
		if !aligned {
			break
		}
		rows = append(rows, lines[j])
	}
	if len(rows) < 2 {
		return nil
	}
	return rows
}

func shortCells(cells []string) bool {
	for _, c := range cells {
		if len([]rune(c)) > 90 {
			return false
		}
	}
	return true
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// renderTable writes GFM: the first row is the header, as in every tool an
// agent already knows.
func renderTable(rows []textLine) string {
	width := 0
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		cells := cellsOf(r)
		if len(cells) > width {
			width = len(cells)
		}
		table = append(table, cells)
	}
	var out strings.Builder
	for i, cells := range table {
		for len(cells) < width {
			cells = append(cells, "")
		}
		out.WriteString("|")
		for _, c := range cells {
			out.WriteString(" " + strings.ReplaceAll(c, "|", "\\|") + " |")
		}
		out.WriteString("\n")
		if i == 0 {
			out.WriteString("|")
			for range cells {
				out.WriteString(" --- |")
			}
			out.WriteString("\n")
		}
	}
	return out.String()
}

// renderLines turns the collected lines into Markdown. Aligned rows become a
// table, a line set clearly larger than the body text becomes a heading (the
// size tiers pdf-inspector uses), and the rest is text.
func renderLines(lines []textLine) string {
	total := map[float64]int{}
	for _, l := range lines {
		total[l.size] += len([]rune(l.text))
	}
	body := 0.0
	best := 0
	for size, n := range total {
		if n > best {
			best, body = n, size
		}
	}
	var out strings.Builder
	prevHeading := false
	for i := 0; i < len(lines); i++ {
		if rows := tableGroup(lines, i); rows != nil {
			if out.Len() > 0 {
				out.WriteString("\n")
			}
			out.WriteString(renderTable(rows))
			i += len(rows) - 1
			prevHeading = false
			continue
		}
		l := lines[i]
		level := 0
		if body > 0 && len([]rune(l.text)) <= 90 {
			ratio := l.size / body
			// The tiers are wide, because a document mixes sizes for reasons
			// that are not a heading (a caption, a table cell, a footer).
			switch {
			case ratio >= 2.2:
				level = 1
			case ratio >= 1.7:
				level = 2
			case ratio >= 1.3:
				level = 3
			}
		}
		if level > 0 {
			if out.Len() > 0 {
				out.WriteString("\n")
			}
			out.WriteString(strings.Repeat("#", level) + " " + l.text + "\n")
			prevHeading = true
			continue
		}
		if prevHeading && out.Len() > 0 {
			out.WriteString("\n")
		}
		prevHeading = false
		out.WriteString(l.text + "\n")
	}
	return strings.ReplaceAll(out.String(), "\r", "\n")
}

// tok is one operator with the operands that were collected before it.
type tok struct {
	op     string
	name   string
	str    []byte
	pieces [][]byte
	nums   []float64
}

// tokens tokenizes a content stream into operators with their operands. An
// operand is a string, a number, a name, or an array of strings and numbers
// (which is what TJ carries).
func tokens(s string) []tok {
	var out []tok
	var cur tok
	reset := func() { cur = tok{} }
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '%':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '(':
			start := i
			i = skipString(s, i)
			cur.str = unescapeString(s[start+1 : i-1])
		case c == '<' && i+1 < len(s) && s[i+1] == '<':
			i = skipDict(s, i)
		case c == '<':
			end := strings.IndexByte(s[i:], '>')
			if end < 0 {
				i++
				continue
			}
			cur.str = unhex(s[i+1 : i+end])
			i += end + 1
		case c == '[':
			i++
			for i < len(s) && s[i] != ']' {
				switch {
				case s[i] == '(':
					start := i
					i = skipString(s, i)
					cur.pieces = append(cur.pieces, unescapeString(s[start+1:i-1]))
				case s[i] == '<':
					end := strings.IndexByte(s[i:], '>')
					if end < 0 {
						i = len(s)
						continue
					}
					cur.pieces = append(cur.pieces, unhex(s[i+1:i+end]))
					i += end + 1
				case s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n':
					i++
				default:
					start := i
					for i < len(s) && !strings.ContainsRune(" \t\r\n[]<>()", rune(s[i])) {
						i++
					}
					if n, err := strconv.ParseFloat(s[start:i], 64); err == nil {
						cur.nums = append(cur.nums, n) // kerning inside the array
					}
				}
			}
			i++ // the ]
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '/' || c == '+' || c == '-' || c == '.' || (c >= '0' && c <= '9'):
			start := i
			i++
			for i < len(s) && !strings.ContainsRune(" \t\r\n/[]<>()", rune(s[i])) {
				i++
			}
			word := s[start:i]
			if strings.HasPrefix(word, "/") {
				cur.name = word[1:]
			} else if n, err := strconv.ParseFloat(word, 64); err == nil {
				cur.nums = append(cur.nums, n)
			}
		default:
			start := i
			for i < len(s) && !strings.ContainsRune(" \t\r\n/[]<>()%", rune(s[i])) {
				i++
			}
			op := s[start:i]
			if op == "" || isNumber(op) {
				i++
				continue
			}
			cur.op = op
			out = append(out, cur)
			reset()
		}
	}
	return out
}

func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// skipString returns the index after the closing parenthesis of a string that
// starts at i, honouring nested parentheses and escapes.
func skipString(s string, i int) int {
	depth := 0
	for ; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}

// skipDict returns the index after the dictionary that starts at i.
func skipDict(s string, i int) int {
	depth := 0
	for ; i < len(s)-1; i++ {
		if s[i] == '<' && s[i+1] == '<' {
			depth++
			i++
			continue
		}
		if s[i] == '>' && s[i+1] == '>' {
			depth--
			i++
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}

// unescapeString turns the escapes of a PDF string into bytes.
func unescapeString(s string) []byte {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' {
			out = append(out, c)
			continue
		}
		i++
		if i >= len(s) {
			break
		}
		switch s[i] {
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case '(', ')', '\\':
			out = append(out, s[i])
		case '\r', '\n':
			// A backslash before a line end continues the string.
			if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		default:
			if s[i] >= '0' && s[i] <= '7' {
				v := 0
				n := 0
				for ; n < 3 && i+n < len(s) && s[i+n] >= '0' && s[i+n] <= '7'; n++ {
					v = v*8 + int(s[i+n]-'0')
				}
				out = append(out, byte(v))
				i += n - 1
			} else {
				out = append(out, s[i])
			}
		}
	}
	return out
}

// unhex turns "<41 42>" into bytes.
func unhex(s string) []byte {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	if len(s)%2 == 1 {
		s += "0"
	}
	out := make([]byte, 0, len(s)/2)
	for i := 0; i+1 < len(s); i += 2 {
		n, err := strconv.ParseUint(s[i:i+2], 16, 8)
		if err != nil {
			continue
		}
		out = append(out, byte(n))
	}
	return out
}

// cp1252 maps one byte of the Windows code page, which is the default for a
// simple font, to a character.
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

// gapPart is a piece of a run that was separated by a wide run of spaces.
type gapPart struct {
	text string
	// offset is where the part starts, in thousandths of the font size from the
	// start of the run.
	offset float64
}

// splitWideGaps splits a run at three or more spaces, which is a column gap and
// not a word space. A single or double space stays inside the text. The offset
// of each part is in thousandths of the font size, counted from the start of the
// run: a space is taken as 250, a character as 500, which is close enough to
// place the parts against each other.
func splitWideGaps(text string) []gapPart {
	var parts []gapPart
	pos := 0.0
	i := 0
	for i < len(text) {
		if text[i] == ' ' {
			j := i
			for j < len(text) && text[j] == ' ' {
				j++
			}
			pos += float64(j-i) * 250
			i = j
			continue
		}
		start := pos
		j := i
		for j < len(text) {
			if text[j] == ' ' && j+2 < len(text) && text[j+1] == ' ' && text[j+2] == ' ' {
				break
			}
			j++
		}
		parts = append(parts, gapPart{text: text[i:j], offset: start})
		pos += float64(j-i) * 500
		i = j
	}
	return parts
}

// advancePart estimates the width of a part when no font metrics are used.
func advancePart(text string, size float64, m matrix) float64 {
	return float64(len([]rune(text))) * 0.5 * size * absf(m.a)
}
