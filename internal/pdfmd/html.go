package pdfmd

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
)

// HTML returns the document as one HTML page.
//
// A tagged PDF says what its text is, so the structure tree is the source: a
// heading becomes a heading, a list a list, a table a table with its head row.
// Without a tree the page text has to be guessed at as before, and the guessed
// Markdown is rendered to HTML - one rule set, one result, no second reader.
func HTML(data []byte) (string, error) {
	if !bytes.Contains(data[:min(len(data), 1024)], []byte("%PDF")) {
		return "", fmt.Errorf("not a PDF (no %%PDF header)")
	}
	if bytes.Contains(data, []byte("/Encrypt")) {
		return "", fmt.Errorf("encrypted PDF, the text cannot be read without the password")
	}
	d := parseObjects(data)
	if len(d) == 0 {
		return "", fmt.Errorf("no objects found")
	}
	if len(pageObjects(d)) == 0 {
		return "", fmt.Errorf("no pages found")
	}
	if tree := structTreeOf(d); tree != nil {
		text := textOfStructPages(d, pageRefs(d))
		if body := htmlTree(tree, text, ""); strings.TrimSpace(body) != "" {
			return htmlPage(body, ""), nil
		}
	}
	// No tree, or a tree without text: read the pages the way we always did.
	md, err := Convert(data)
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	if err := goldmark.New().Convert([]byte(md), &rendered); err != nil {
		return "", fmt.Errorf("markdown to html: %w", err)
	}
	return htmlPage(rendered.String(), ""), nil
}

// htmlPage wraps a body in a document that names its language.
func htmlPage(body, lang string) string {
	if lang == "" {
		lang = "de"
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"" + lang + "\">\n<head>\n<meta charset=\"UTF-8\"/>\n")
	if title := firstHeadingText(body); title != "" {
		b.WriteString("<title>" + escHTML(title) + "</title>\n")
	}
	b.WriteString("</head>\n<body>\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// firstHeadingText is the text of the first heading, used as the page title.
func firstHeadingText(body string) string {
	for _, tag := range []string{"h1", "h2", "h3"} {
		start := strings.Index(body, "<"+tag+">")
		if start < 0 {
			continue
		}
		start += len(tag) + 2
		end := strings.Index(body[start:], "</"+tag+">")
		if end < 0 {
			continue
		}
		if t := strings.TrimSpace(body[start : start+end]); t != "" {
			return stripTags(t)
		}
	}
	return ""
}

// stripTags removes markup, for a title attribute or a file name.
func stripTags(s string) string {
	var b strings.Builder
	inside := false
	for _, r := range s {
		switch {
		case r == '<':
			inside = true
		case r == '>':
			inside = false
		case !inside:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(escHTML(unescHTML(b.String())))
}

// escHTML makes text safe for HTML content and attributes.
func escHTML(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// unescHTML reverses the five entities we escape, so a title is not escaped
// twice when it passes through stripTags.
func unescHTML(s string) string {
	return strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`).Replace(s)
}

// htmlTree renders the structure tree as HTML body content.
func htmlTree(e *structElem, text structText, fallbackPage string) string {
	var b strings.Builder
	htmlWrite(e, &b, text, fallbackPage)
	return b.String()
}

// htmlWrite renders one element and its children.
func htmlWrite(e *structElem, b *strings.Builder, text structText, fallbackPage string) {
	switch role := normalizeRole(e.role); {
	case strings.HasPrefix(role, "H") && len(role) == 2 && role[1] >= '1' && role[1] <= '6':
		if txt := e.ownText(text, fallbackPage); txt != "" {
			b.WriteString("<h" + role[1:] + ">" + escHTML(txt) + "</h" + role[1:] + ">\n")
		}
	case role == "P", role == "Caption", role == "Note":
		if txt := e.ownText(text, fallbackPage); txt != "" {
			b.WriteString("<p>" + escHTML(txt) + "</p>\n")
		}
	case role == "BlockQuote":
		if txt := e.ownText(text, fallbackPage); txt != "" {
			b.WriteString("<blockquote><p>" + escHTML(txt) + "</p></blockquote>\n")
		}
	case role == "Code":
		if txt := e.ownText(text, fallbackPage); txt != "" {
			b.WriteString("<pre><code>" + escHTML(txt) + "</code></pre>\n")
		}
	case role == "L":
		htmlWriteList(e, b, text, fallbackPage)
	case role == "LI", role == "Lbl", role == "LBody":
		// Reached only if a list is missing its wrapper: the item is written on
		// its own rather than dropped.
		if txt := e.ownText(text, fallbackPage); txt != "" {
			b.WriteString("<p>" + escHTML(txt) + "</p>\n")
		}
	case role == "Table":
		htmlWriteTable(e, b, text, fallbackPage)
	case role == "Figure":
		alt := e.alt
		if alt == "" {
			alt = e.ownText(text, fallbackPage)
		}
		if alt == "" {
			alt = "Bild"
		}
		b.WriteString(`<figure><img alt="` + escHTML(alt) + `"/></figure>` + "\n")
	case role == "":
		for _, k := range e.kids {
			if kid, ok := k.(*structElem); ok {
				htmlWrite(kid, b, text, fallbackPage)
			}
		}
	default:
		// Unknown block: its text still belongs to the document.
		if txt := e.ownText(text, fallbackPage); txt != "" {
			b.WriteString("<p>" + escHTML(txt) + "</p>\n")
		}
		for _, k := range e.kids {
			kid, ok := k.(*structElem)
			if !ok || inlineRole(normalizeRole(kid.role)) {
				continue
			}
			htmlWrite(kid, b, text, fallbackPage)
		}
	}
}

// htmlWriteList renders a list with its items.
func htmlWriteList(e *structElem, b *strings.Builder, text structText, fallbackPage string) {
	items := listItems(e, text, fallbackPage)
	if len(items) == 0 {
		return
	}
	b.WriteString("<ul>\n")
	for _, it := range items {
		b.WriteString("<li>" + escHTML(it.text))
		for _, sub := range it.nested {
			var inner strings.Builder
			htmlWriteList(sub, &inner, text, fallbackPage)
			b.WriteString("\n" + inner.String())
		}
		b.WriteString("</li>\n")
	}
	b.WriteString("</ul>\n")
}

// listItem is one entry of a list: its text and the lists inside it.
type listItem struct {
	text   string
	nested []*structElem
}

// listItems collects the items of a list element.
func listItems(e *structElem, text structText, fallbackPage string) []listItem {
	var items []listItem
	for _, k := range e.kids {
		kid, ok := k.(*structElem)
		if !ok || normalizeRole(kid.role) != "LI" {
			continue
		}
		it := listItem{text: kid.ownText(text, fallbackPage)}
		for _, kk := range kid.kids {
			sub, ok := kk.(*structElem)
			if !ok || normalizeRole(sub.role) != "L" {
				continue
			}
			it.nested = append(it.nested, sub)
		}
		if it.text == "" && len(it.nested) == 0 {
			continue
		}
		items = append(items, it)
	}
	return items
}

// htmlWriteTable renders a table, a head row as a head row.
func htmlWriteTable(e *structElem, b *strings.Builder, text structText, fallbackPage string) {
	var rows [][]string
	var head []string
	for _, k := range e.kids {
		row, ok := k.(*structElem)
		if !ok || normalizeRole(row.role) != "TR" {
			continue
		}
		var cells []string
		header := 0
		for _, kk := range row.kids {
			cell, ok := kk.(*structElem)
			if !ok {
				continue
			}
			role := normalizeRole(cell.role)
			if role != "TD" && role != "TH" {
				continue
			}
			if role == "TH" {
				header++
			}
			cells = append(cells, cell.ownText(text, fallbackPage))
		}
		if len(cells) == 0 || strings.TrimSpace(strings.Join(cells, "")) == "" {
			continue
		}
		if header == len(cells) && head == nil {
			head = cells
			continue
		}
		rows = append(rows, cells)
	}
	if len(head) == 0 && len(rows) == 0 {
		return
	}
	b.WriteString("<table>\n")
	if len(head) > 0 {
		b.WriteString("<thead><tr>")
		for _, c := range head {
			b.WriteString("<th>" + escHTML(c) + "</th>")
		}
		b.WriteString("</tr></thead>\n")
	}
	if len(rows) > 0 {
		b.WriteString("<tbody>\n")
		for _, r := range rows {
			b.WriteString("<tr>")
			for _, c := range r {
				b.WriteString("<td>" + escHTML(c) + "</td>")
			}
			b.WriteString("</tr>\n")
		}
		b.WriteString("</tbody>\n")
	}
	b.WriteString("</table>\n")
}
