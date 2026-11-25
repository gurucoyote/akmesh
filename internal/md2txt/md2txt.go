// Package md2txt turns Markdown into plain text, the way pandoc -t plain does.
//
// The reference is pandoc 3.x, measured on the machine, not guessed:
//
//   - a heading loses its marks and keeps its text;
//   - emphasis, code spans and escapes lose their markers;
//   - a link keeps its text, an autolink keeps its URL, an image becomes its
//     description in brackets;
//   - lists are indented by two spaces per level, an ordered item has two
//     spaces after the number;
//   - a blockquote is indented by two spaces, a code block by four;
//   - a table keeps its columns padded to their width, with a line of dashes
//     after the first row;
//   - a horizontal rule becomes 72 dashes, a paragraph is wrapped at 72
//     characters.
//
// The point of matching pandoc this closely is that an agent may compare the
// two, and a difference would look like a bug in both.
package md2txt

import (
	"strings"
)

// wrapWidth is where pandoc wraps a paragraph in plain text.
const wrapWidth = 72

// Convert returns the plain text of a Markdown document.
func Convert(md string) string {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	md = strings.ReplaceAll(md, "\r", "\n")
	lines := strings.Split(md, "\n")
	var blocks []string
	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			i++
		case isFence(trimmed):
			fence := fenceOf(trimmed)
			var body []string
			i++
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), fence) {
				body = append(body, lines[i])
				i++
			}
			i++ // the closing fence
			blocks = append(blocks, codeBlock(body))
		case headingLevel(trimmed) > 0:
			text := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			text = strings.TrimSpace(strings.TrimRight(text, "#"))
			blocks = append(blocks, wrapInline(text, ""))
			i++
		case isRule(trimmed):
			blocks = append(blocks, strings.Repeat("-", 72))
			i++
		case strings.HasPrefix(trimmed, ">"):
			var body []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">") {
				body = append(body, strings.TrimPrefix(strings.TrimSpace(lines[i]), ">"))
				i++
			}
			blocks = append(blocks, indentBlock(wrapInline(strings.Join(body, "\n"), ""), "  "))
		case isTableStart(lines, i):
			tbl, next := readTable(lines, i)
			blocks = append(blocks, tbl)
			i = next
		case listMarker(trimmed) != "":
			items, next := readList(lines, i, 0)
			blocks = append(blocks, items)
			i = next
		case isDefTerm(lines, i):
			term := strings.TrimSpace(lines[i])
			var def []string
			i++
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ":") {
				def = append(def, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ":")))
				i++
			}
			blocks = append(blocks, wrapInline(term, "")+"\n"+indentBlock(wrapInline(strings.Join(def, "\n"), ""), "    "))
		case strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t"):
			var body []string
			for i < len(lines) && (strings.HasPrefix(lines[i], "    ") || strings.HasPrefix(lines[i], "\t") || strings.TrimSpace(lines[i]) == "") {
				body = append(body, strings.TrimPrefix(strings.TrimPrefix(lines[i], "    "), "\t"))
				i++
			}
			for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
				body = body[:len(body)-1]
			}
			blocks = append(blocks, codeBlock(body))
		default:
			var body []string
			for i < len(lines) {
				l := lines[i]
				if strings.TrimSpace(l) == "" || headingLevel(strings.TrimSpace(l)) > 0 ||
					isFence(strings.TrimSpace(l)) || isRule(strings.TrimSpace(l)) ||
					strings.HasPrefix(strings.TrimSpace(l), ">") || isTableStart(lines, i) {
					break
				}
				body = append(body, l)
				i++
			}
			blocks = append(blocks, wrapInline(strings.Join(body, "\n"), ""))
		}
	}
	var out strings.Builder
	for i, b := range blocks {
		if i > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(strings.TrimRight(b, " \t\n"))
	}
	if out.Len() == 0 {
		return ""
	}
	return out.String() + "\n"
}

func isFence(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

func fenceOf(trimmed string) string {
	if strings.HasPrefix(trimmed, "```") {
		return "```"
	}
	return "~~~"
}

func headingLevel(trimmed string) int {
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0
	}
	if n == len(trimmed) || trimmed[n] == ' ' {
		return n
	}
	return 0
}

func isRule(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}
	c := trimmed[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != c && trimmed[i] != ' ' {
			return false
		}
	}
	return true
}

// listMarker returns the marker of a list item and its text, or "".
func listMarker(trimmed string) string {
	if len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ' {
		return string(trimmed[0])
	}
	i := 0
	for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(trimmed) && (trimmed[i] == '.' || trimmed[i] == ')') && trimmed[i+1] == ' ' {
		return trimmed[:i+1]
	}
	return ""
}

// readList reads a list, including its nested items, and returns it with the
// index of the first line after it. A nested item is indented by two spaces or
// by the width of its parent marker; the output uses two spaces per level, as
// pandoc does.
func readList(lines []string, start, depth int) (string, int) {
	var out strings.Builder
	i := start
	for i < len(lines) {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		marker := listMarker(trimmed)
		if marker == "" {
			break
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if indent < depth*2 {
			break
		}
		text := strings.TrimSpace(trimmed[len(marker):])
		bullet := "- "
		if marker[0] >= '0' && marker[0] <= '9' {
			bullet = marker + "  "
			if len(bullet) > 4 {
				bullet = marker + " "
			}
		}
		out.WriteString(strings.Repeat("  ", depth) + bullet + inline(text) + "\n")
		i++
		// A nested list follows immediately.
		if i < len(lines) {
			next := lines[i]
			nextTrim := strings.TrimSpace(next)
			nextIndent := len(next) - len(strings.TrimLeft(next, " "))
			if listMarker(nextTrim) != "" && nextIndent > indent {
				sub, n := readList(lines, i, depth+1)
				out.WriteString(sub + "\n")
				i = n
			}
		}
	}
	return strings.TrimRight(out.String(), "\n"), i
}

// isTableStart says whether a GFM table begins here: a row with a pipe and a
// separator row of dashes below it.
func isTableStart(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	if !strings.Contains(lines[i], "|") {
		return false
	}
	sep := strings.TrimSpace(lines[i+1])
	if !strings.Contains(sep, "-") || !strings.Contains(sep, "|") {
		return false
	}
	for _, r := range sep {
		if r != '|' && r != '-' && r != ':' && r != ' ' {
			return false
		}
	}
	return true
}

// readTable writes the table the way pandoc does: every column is padded to its
// widest cell plus two, the columns are separated by one space, and a line of
// dashes of the same width follows the first row.
func readTable(lines []string, start int) (string, int) {
	var rows [][]string
	i := start
	for i < len(lines) && strings.Contains(lines[i], "|") && strings.TrimSpace(lines[i]) != "" {
		if i == start+1 {
			i++
			continue // the separator row
		}
		rows = append(rows, splitRow(lines[i]))
		i++
	}
	if len(rows) == 0 {
		return "", i
	}
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	width := make([]int, cols)
	for _, r := range rows {
		for c, cell := range r {
			if w := len([]rune(cell)); w > width[c] {
				width[c] = w
			}
		}
	}
	var out strings.Builder
	for ri, r := range rows {
		out.WriteString("  ")
		for c := 0; c < cols; c++ {
			cell := ""
			if c < len(r) {
				cell = r[c]
			}
			if c < cols-1 {
				// The column is padded to its width plus two, then one space
				// separates it from the next; that is the shape pandoc writes.
				out.WriteString(cell + strings.Repeat(" ", width[c]+3-len([]rune(cell))))
			} else {
				out.WriteString(cell)
			}
		}
		out.WriteString("\n")
		if ri == 0 {
			out.WriteString("  ")
			for c := 0; c < cols; c++ {
				out.WriteString(strings.Repeat("-", width[c]+2))
				if c < cols-1 {
					out.WriteString(" ")
				}
			}
			out.WriteString("\n")
		}
	}
	return strings.TrimRight(out.String(), "\n"), i
}

// splitRow splits a table row into cells, honouring an escaped pipe.
func splitRow(line string) []string {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '|' {
			cur.WriteString("|")
			i++
			continue
		}
		if s[i] == '|' {
			cells = append(cells, strings.TrimSpace(inline(cur.String())))
			cur.Reset()
			continue
		}
		cur.WriteRune(rune(s[i]))
	}
	cells = append(cells, strings.TrimSpace(inline(cur.String())))
	return cells
}

func isDefTerm(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	next := strings.TrimSpace(lines[i+1])
	return strings.HasPrefix(next, ": ") && strings.TrimSpace(lines[i]) != ""
}

// codeBlock indents every line by four spaces, the way pandoc writes a code
// block in plain text.
func codeBlock(body []string) string {
	var out strings.Builder
	for i, l := range body {
		if i > 0 {
			out.WriteString("\n")
		}
		if l == "" {
			continue
		}
		out.WriteString("    " + l)
	}
	return out.String()
}

func indentBlock(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

// wrapInline removes the inline markup of a block and wraps it at the width
// pandoc uses. A line break written with two trailing spaces or a backslash is
// kept.
func wrapInline(text, prefix string) string {
	text = strings.TrimSpace(text)
	// A hard break is a line end with two spaces or a backslash before it.
	var hard []string
	for l := range strings.SplitSeq(text, "\n") {
		hard = append(hard, l)
	}
	var paragraphs []string
	var cur strings.Builder
	for i, l := range hard {
		brokeAfter := strings.HasSuffix(l, "  ") || strings.HasSuffix(l, "\\")
		clean := strings.TrimRight(inline(l), " ")
		if cur.Len() > 0 {
			cur.WriteString(" ")
		}
		cur.WriteString(clean)
		if brokeAfter || i == len(hard)-1 {
			paragraphs = append(paragraphs, cur.String())
			cur.Reset()
		}
	}
	var out []string
	for _, p := range paragraphs {
		out = append(out, wrapText(p, wrapWidth))
	}
	return strings.Join(out, "\n")
}

// wrapText breaks a line at spaces so that no line is longer than width.
func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var out strings.Builder
	lineLen := 0
	for i, w := range words {
		wl := len([]rune(w))
		if i == 0 {
			out.WriteString(w)
			lineLen = wl
			continue
		}
		if lineLen+1+wl > width {
			out.WriteString("\n" + w)
			lineLen = wl
			continue
		}
		out.WriteString(" " + w)
		lineLen += 1 + wl
	}
	return out.String()
}

// inline removes the inline markup of one line or paragraph.
func inline(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			out.WriteByte(s[i+1])
			i += 2
		case strings.HasPrefix(s[i:], "!["):
			// An image: its description stands in for it, in brackets.
			alt, rest, ok := bracket(s[i+2:])
			if ok {
				out.WriteString("[" + inline(alt) + "]")
				i = len(s) - len(rest)
				i = skipTarget(s, i)
				continue
			}
			out.WriteByte(c)
			i++
		case c == '[':
			text, rest, ok := bracket(s[i+1:])
			if ok && isLinkTarget(rest) {
				out.WriteString(inline(text))
				i = len(s) - len(rest)
				i = skipTarget(s, i)
				continue
			}
			out.WriteByte(c)
			i++
		case c == '<':
			end := strings.IndexByte(s[i:], '>')
			if end < 0 {
				out.WriteByte(c)
				i++
				continue
			}
			inner := s[i+1 : i+end]
			if strings.HasPrefix(inner, "http://") || strings.HasPrefix(inner, "https://") || strings.HasPrefix(inner, "mailto:") {
				out.WriteString(strings.TrimPrefix(inner, "mailto:"))
			}
			// An HTML tag vanishes; plain text has no tags.
			i += end + 1
		case c == '`':
			// A code span: keep the text, drop the ticks.
			n := 1
			for i+n < len(s) && s[i+n] == '`' {
				n++
			}
			closeAt := strings.Index(s[i+n:], strings.Repeat("`", n))
			if closeAt < 0 {
				out.WriteByte(c)
				i++
				continue
			}
			code := s[i+n : i+n+closeAt]
			out.WriteString(strings.TrimSpace(code))
			i = i + n + closeAt + n
		case strings.HasPrefix(s[i:], "**") || strings.HasPrefix(s[i:], "__"):
			marker := s[i : i+2]
			end := strings.Index(s[i+2:], marker)
			if end < 0 {
				out.WriteString(marker)
				i += 2
				continue
			}
			out.WriteString(inline(s[i+2 : i+2+end]))
			i = i + 2 + end + 2
		case strings.HasPrefix(s[i:], "~~"):
			end := strings.Index(s[i+2:], "~~")
			if end < 0 {
				out.WriteString("~~")
				i += 2
				continue
			}
			out.WriteString(inline(s[i+2 : i+2+end]))
			i = i + 2 + end + 2
		case c == '*' || c == '_':
			// Emphasis needs its marker tight against the text on both sides: a
			// marker followed by a space opens nothing, and one preceded by a
			// space closes nothing. Without that rule "5*3" and a lone "*" would
			// swallow the rest of the line.
			if i+1 >= len(s) || s[i+1] == ' ' || s[i+1] == '\t' {
				out.WriteByte(c)
				i++
				continue
			}
			if c == '_' && i > 0 && isWordByte(s[i-1]) {
				out.WriteByte(c)
				i++
				continue
			}
			end := -1
			for k := i + 1; k < len(s); k++ {
				if s[k] == c && s[k-1] != ' ' && k > i+1 {
					end = k
					break
				}
			}
			if end < 0 {
				out.WriteByte(c)
				i++
				continue
			}
			out.WriteString(inline(s[i+1 : end]))
			i = end + 1
		default:
			out.WriteByte(c)
			i++
		}
	}
	return out.String()
}

// isWordByte says whether b can be part of a word, which decides whether an
// underscore is emphasis or part of a name.
func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// bracket returns the text up to the closing bracket and what follows it.
func bracket(s string) (string, string, bool) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			if depth == 0 {
				return s[:i], s[i+1:], true
			}
			depth--
		}
	}
	return "", "", false
}

// isLinkTarget says whether the text starts with a link target such as "(url)".
func isLinkTarget(rest string) bool {
	rest = strings.TrimLeft(rest, " ")
	return strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "[")
}

// skipTarget returns the index after the "(url)" or "[ref]" of a link.
func skipTarget(s string, i int) int {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if i >= len(s) {
		return i
	}
	switch s[i] {
	case '(':
		depth := 0
		for ; i < len(s); i++ {
			if s[i] == '(' {
				depth++
			} else if s[i] == ')' {
				depth--
				if depth == 0 {
					return i + 1
				}
			}
		}
		return len(s)
	case '[':
		for ; i < len(s); i++ {
			if s[i] == ']' {
				return i + 1
			}
		}
		return len(s)
	}
	return i
}
