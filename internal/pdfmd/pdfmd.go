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
	return textOf(content, readFonts(d, resources))
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
	// A line is collected with the size it was set in: the size is the only
	// thing in a PDF that says "heading", so it has to survive the reading.
	var lines []textLine
	var buf bytes.Buffer
	lineSizes := map[float64]int{}
	curSize := 10.0
	flushLine := func() {
		t := strings.TrimRight(buf.String(), " \t")
		if strings.TrimSpace(t) != "" {
			size := curSize
			best := 0
			for sz, n := range lineSizes {
				if n > best {
					best, size = n, sz
				}
			}
			lines = append(lines, textLine{text: t, size: size})
		}
		buf.Reset()
		lineSizes = map[float64]int{}
	}
	var cur *font
	size := 10.0
	// tx, ty are the text matrix: Td and Tm move it, and the text of the next
	// operator starts there. The pen moves on its own while a string is
	// written, so the gap between two pieces is tx minus where the last piece
	// ended - not the Td value itself.
	tx, ty := 0.0, 0.0
	lastStart, lastWidth := 0.0, 0.0
	lineHasText := false
	spacePending := false

	newLine := func() {
		if lineHasText {
			flushLine()
			lineHasText = false
		}
		spacePending = false
	}
	advance := func(b []byte) float64 {
		if w := cur.width(b, size); w > 0 {
			return w
		}
		return float64(len([]rune(string(b)))) * 0.5 * size
	}
	emit := func(b []byte) {
		if len(b) == 0 {
			return
		}
		text := string(b)
		if cur != nil {
			text = cur.decode(b)
		}
		if text == "" {
			return
		}
		gap := tx - (lastStart + lastWidth)
		// A word space is about a quarter of the font size. Missing widths in a
		// CID font make the estimate uncertain, so the line stays readable and
		// the threshold is generous rather than putting a space inside a word.
		if lineHasText && (spacePending || gap > 0.35*size) {
			if !strings.HasSuffix(buf.String(), " ") && !strings.HasPrefix(text, " ") {
				buf.WriteString(" ")
			}
		}
		spacePending = false
		buf.WriteString(text)
		lineSizes[size] += len([]rune(text))
		curSize = size
		lastStart = tx
		lastWidth = advance(b)
		lineHasText = true
	}

	for _, t := range tokens(string(content)) {
		switch t.op {
		case "BT":
			tx, ty = 0, 0
			lastStart, lastWidth = 0, 0
		case "Tf":
			if len(t.nums) > 0 {
				size = t.nums[0]
			}
			cur = fonts[t.name]
		case "Td", "TD":
			if len(t.nums) >= 2 {
				tx += t.nums[0]
				ty += t.nums[1]
				if t.nums[1] != 0 {
					newLine()
					lastStart, lastWidth = tx, 0
				}
			}
		case "T*":
			newLine()
		case "Tm":
			if len(t.nums) >= 6 {
				if t.nums[5] != ty && lineHasText {
					newLine()
				}
				tx, ty = t.nums[4], t.nums[5]
				lastStart, lastWidth = tx, 0
			}
		case "Tj":
			emit(t.str)
		case "TJ":
			for _, piece := range t.pieces {
				emit(piece)
			}
			for _, n := range t.nums {
				// A large negative adjustment is a word space, a small one is
				// kerning inside a word.
				if n <= -100 {
					spacePending = true
				}
			}
		case "'", `"`:
			newLine()
			lastStart, lastWidth = tx, 0
			emit(t.str)
		}
	}
	flushLine()
	return renderLines(lines)
}

// textLine is one line of the page with the size most of its characters were
// set in.
type textLine struct {
	text string
	size float64
}

// renderLines turns the collected lines into Markdown. The body size is the one
// that carries the most characters; a line set clearly larger than that is a
// heading, and the level follows the distance from the body size.
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
	for _, l := range lines {
		level := 0
		if body > 0 && len([]rune(l.text)) <= 90 {
			ratio := l.size / body
			switch {
			case ratio >= 1.8:
				level = 1
			case ratio >= 1.4:
				level = 2
			case ratio >= 1.15:
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
