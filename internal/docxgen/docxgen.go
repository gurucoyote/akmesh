// Package docxgen turns Markdown into a .docx.
//
// No third-party library and no LibreOffice: a .docx is a ZIP with a handful of
// XML parts, and WordprocessingML is small enough to write. The layout follows
// the traps recorded in the notker project, which edits the same format by hand:
//
//   - every part that the package needs must be listed in [Content_Types].xml
//     and reachable through a relationship, or Word calls the file damaged and
//     says nothing about which part was wrong;
//   - a heading is a style, not a font size: w:pStyle names HeadingN, and
//     styles.xml has to define it;
//   - a list needs three parts that agree: the paragraph carries w:numPr with a
//     numId and an ilvl, word/numbering.xml maps that numId to an abstractNum,
//     and the abstractNum carries one w:lvl per level with format and indent;
//   - a table cell holds paragraphs, not runs.
//
// What it deliberately does not do: no images (a media part needs its own
// relationship, content type and EMU size), no footnotes, no comments, no
// tracked changes. Those are not needed for reading Markdown and would each add
// a part that can be wrong.
package docxgen

import (
	"archive/zip"
	"io"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// Convert writes the .docx of a Markdown document.
func Convert(w io.Writer, markdown []byte) error {
	return ConvertWithBase(w, markdown, "")
}

// ConvertWithBase is Convert with a directory to resolve relative image paths
// against: the folder the Markdown file lives in.
func ConvertWithBase(w io.Writer, markdown []byte, baseDir string) error {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM, extension.Strikethrough))
	root := md.Parser().Parse(text.NewReader(markdown))

	body := &strings.Builder{}
	b := &builder{src: markdown, out: body, baseDir: baseDir}
	b.blocks(root)

	var exts []string
	for _, img := range b.images {
		exts = append(exts, img.ext)
	}
	parts := []struct {
		name string
		body string
	}{
		{"[Content_Types].xml", contentTypes(exts)},
		{"_rels/.rels", packageRels},
		{"word/_rels/document.xml.rels", documentRels(b.links, b.images)},
		{"word/document.xml", documentXML(body.String())},
		{"word/styles.xml", stylesXML},
		{"word/numbering.xml", numberingXML},
	}
	zw := zip.NewWriter(w)
	for _, p := range parts {
		f, err := zw.Create(p.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(f, p.body); err != nil {
			return err
		}
	}
	// The pictures are bytes, not text: written after the XML parts, under
	// word/, which is where the relationship points.
	for _, img := range b.images {
		f, err := zw.Create("word/" + img.name)
		if err != nil {
			return err
		}
		if _, err := f.Write(img.data); err != nil {
			return err
		}
	}
	return zw.Close()
}

// builder walks the Markdown tree and writes WordprocessingML.
type builder struct {
	src []byte
	out *strings.Builder
	// listDepth tracks how deep the current list is nested.
	listDepth int
	// baseDir is where relative image paths are resolved: the folder of the
	// Markdown file.
	baseDir string
	// images are the pictures that could be embedded, in the order they were
	// written.
	images []imagePart
	// links holds one target per hyperlink, in the order they were written;
	// the relationship id is linkBase plus its index.
	links []string
}

// linkBase is the first relationship id used for hyperlinks: rId1 and rId2 are
// the styles and the numbering part.
const linkBase = 3

// imagePart is one picture in the package: its name under word/, its bytes and
// the format they are in.
type imagePart struct {
	name string
	data []byte
	ext  string
}

// imageBase is the first relationship id used for pictures: they follow the
// hyperlinks, so the two lists cannot collide.
func (b *builder) imageBase() int { return linkBase + len(b.links) }

// link writes a clickable hyperlink: the runs of fn, wrapped in w:hyperlink with
// a relationship to target. An empty target, or one that points inside the
// document ("#abschnitt"), stays plain text - a relationship without a part
// behind it is worse than none.
func (b *builder) link(target string, fn func()) {
	if target == "" || strings.HasPrefix(target, "#") {
		fn()
		return
	}
	b.links = append(b.links, target)
	b.out.WriteString(`<w:hyperlink r:id="rId` + strconv.Itoa(linkBase+len(b.links)-1) + `">`)
	fn()
	b.out.WriteString("</w:hyperlink>")
}

// blocks writes the block children of a node.
func (b *builder) blocks(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		b.block(c)
	}
}

func (b *builder) block(n ast.Node) {
	switch v := n.(type) {
	case *ast.Heading:
		level := v.Level
		if level > 6 {
			level = 6
		}
		b.out.WriteString("<w:p><w:pPr><w:pStyle w:val=\"Heading" + strconv.Itoa(level) + "\"/></w:pPr>")
		b.inline(n)
		b.out.WriteString("</w:p>")
	case *ast.Paragraph:
		b.out.WriteString("<w:p>")
		b.inline(n)
		b.out.WriteString("</w:p>")
	case *ast.TextBlock:
		b.out.WriteString("<w:p>")
		b.inline(n)
		b.out.WriteString("</w:p>")
	case *ast.List:
		b.list(v)
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		b.code(n)
	case *ast.Blockquote:
		b.quote(n)
	case *ast.ThematicBreak:
		b.out.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="999999"/></w:pBdr></w:pPr></w:p>`)
	case *ast.HTMLBlock:
		// Raw HTML has no place in a Word document; its lines would read as
		// tags. Dropping it is the honest choice.
	default:
		// The GFM table is a type of the extension package, not of the core
		// node set - matching on the kind name keeps the import out.
		if n.Kind().String() == "Table" {
			b.table(n)
			return
		}
		b.blocks(n)
	}
}

// list writes a list: bullet or numbered, nested by ilvl.
func (b *builder) list(l *ast.List) {
	numID := 2
	if l.IsOrdered() {
		numID = 3
	}
	level := b.listDepth
	if level > 8 {
		level = 8
	}
	b.listDepth++
	for item := l.FirstChild(); item != nil; item = item.NextSibling() {
		if item.Kind() != ast.KindListItem {
			continue
		}
		// The first paragraph of an item carries the number, a nested list
		// follows as further paragraphs with the deeper level.
		first := true
		for c := item.FirstChild(); c != nil; c = c.NextSibling() {
			switch c.(type) {
			case *ast.List:
				b.list(c.(*ast.List))
				continue
			}
			if first {
				b.out.WriteString(`<w:p><w:pPr><w:numPr><w:ilvl w:val="` + strconv.Itoa(level) +
					`"/><w:numId w:val="` + strconv.Itoa(numID) + `"/></w:numPr><w:pStyle w:val="ListParagraph"/></w:pPr>`)
				first = false
			} else {
				b.out.WriteString(`<w:p><w:pPr><w:numPr><w:ilvl w:val="` + strconv.Itoa(level) +
					`"/><w:numId w:val="` + strconv.Itoa(numID) + `"/></w:numPr></w:pPr>`)
			}
			b.inline(c)
			b.out.WriteString("</w:p>")
		}
		if first {
			// An item without any paragraph: keep it, or the numbering of the
			// following items shifts.
			b.out.WriteString(`<w:p><w:pPr><w:numPr><w:ilvl w:val="` + strconv.Itoa(level) +
				`"/><w:numId w:val="` + strconv.Itoa(numID) + `"/></w:numPr></w:pPr></w:p>`)
		}
	}
	b.listDepth--
}

// code writes a fenced or indented code block: one paragraph per line, in the
// Code style, so the lines keep their own shape.
func (b *builder) code(n ast.Node) {
	var lines []string
	switch v := n.(type) {
	case *ast.FencedCodeBlock:
		lines = b.lines(v.Lines())
	case *ast.CodeBlock:
		lines = b.lines(v.Lines())
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	for _, line := range lines {
		b.out.WriteString(`<w:p><w:pPr><w:pStyle w:val="Code"/></w:pPr><w:r><w:t xml:space="preserve">`)
		b.out.WriteString(escape(line))
		b.out.WriteString("</w:t></w:r></w:p>")
	}
}

// lines returns the source lines of a segment list.
func (b *builder) lines(seg *text.Segments) []string {
	if seg == nil {
		return nil
	}
	var out []string
	for i := 0; i < seg.Len(); i++ {
		segment := seg.At(i)
		line := string(segment.Value(b.src))
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")
		out = append(out, line)
	}
	// A trailing blank line inside a fence is not content.
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return out
}

// quote writes a blockquote as an indented paragraph, which is what Word has.
func (b *builder) quote(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		b.out.WriteString(`<w:p><w:pPr><w:pStyle w:val="Quote"/><w:ind w:left="567"/></w:pPr>`)
		b.inline(c)
		b.out.WriteString("</w:p>")
	}
}

// table writes a GFM table as a Word table. The first row is the header, and a
// header row needs w:tblHeader only to repeat across pages; the look comes from
// the Header style on its runs.
func (b *builder) table(n ast.Node) {
	var rows []ast.Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		// The header row has a kind of its own, "TableHeader"; missing it drops
		// the first row of every table.
		switch c.Kind().String() {
		case "TableRow", "TableHeader":
			rows = append(rows, c)
		}
	}
	if len(rows) == 0 {
		return
	}
	cols := 0
	for _, r := range rows {
		if n := countCells(r); n > cols {
			cols = n
		}
	}
	b.out.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="0" w:type="auto"/>` +
		`<w:tblBorders><w:top w:val="single" w:sz="4" w:space="0" w:color="auto"/>` +
		`<w:left w:val="single" w:sz="4" w:space="0" w:color="auto"/>` +
		`<w:bottom w:val="single" w:sz="4" w:space="0" w:color="auto"/>` +
		`<w:right w:val="single" w:sz="4" w:space="0" w:color="auto"/>` +
		`<w:insideH w:val="single" w:sz="4" w:space="0" w:color="auto"/>` +
		`<w:insideV w:val="single" w:sz="4" w:space="0" w:color="auto"/></w:tblBorders></w:tblPr><w:tblGrid>`)
	for i := 0; i < cols; i++ {
		b.out.WriteString(`<w:gridCol/>`)
	}
	b.out.WriteString(`</w:tblGrid>`)
	for ri, r := range rows {
		b.out.WriteString("<w:tr>")
		if ri == 0 {
			b.out.WriteString("<w:trPr><w:tblHeader/></w:trPr>")
		}
		for c := r.FirstChild(); c != nil; c = c.NextSibling() {
			if c.Kind().String() != "TableCell" {
				continue
			}
			b.out.WriteString("<w:tc><w:tcPr/>")
			b.out.WriteString("<w:p>")
			if ri == 0 {
				b.out.WriteString(`<w:pPr><w:pStyle w:val="TableHeader"/></w:pPr>`)
			}
			b.inline(c)
			b.out.WriteString("</w:p></w:tc>")
		}
		b.out.WriteString("</w:tr>")
	}
	b.out.WriteString("</w:tbl>")
	// A table must be followed by a paragraph, or Word puts the next table
	// straight underneath it.
	b.out.WriteString("<w:p/>")
}

// countCells returns how many cells a row has.
func countCells(row ast.Node) int {
	n := 0
	for c := row.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind().String() == "TableCell" {
			n++
		}
	}
	return n
}

// inline writes the runs of a block: text with its bold, italic and monospace
// properties, and the text of a link rather than the tag around it.
func (b *builder) inline(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			b.run(string(v.Segment.Value(b.src)), runProps{})
			if v.HardLineBreak() || v.SoftLineBreak() {
				b.out.WriteString("<w:r><w:br/></w:r>")
			}
		case *ast.String:
			b.run(string(v.Value), runProps{})
		case *ast.Emphasis:
			b.emphasis(v)
		case *ast.CodeSpan:
			b.spanCode(v)
		case *ast.RawHTML:
			// Nothing of a tag belongs in the text.
		case *ast.AutoLink:
			// An autolink shows its own address and points at it.
			target := string(v.URL(b.src))
			b.link(target, func() { b.run(string(v.Label(b.src)), runProps{}) })
		case *ast.Link:
			// The address goes into the relationship, the text stays visible:
			// this is what makes the link clickable in Word.
			target := string(v.Destination)
			b.link(target, func() { b.inline(c) })
		case *ast.Image:
			// The picture when it can be embedded, its description otherwise:
			// a document without the image is still readable, one with a
			// broken reference is not.
			if !b.picture(string(v.Destination), altText(v, b.src)) {
				b.run("["+altText(v, b.src)+"]", runProps{})
			}

		default:
			b.inline(c)
		}
	}
}

// altText is the description of an image, which goldmark keeps as child text.
func altText(img *ast.Image, src []byte) string {
	var out strings.Builder
	for c := img.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			out.Write(t.Segment.Value(src))
		}
	}
	return out.String()
}

// emphasis writes bold and italic the way Word keeps them: a run property, not
// a character.
func (b *builder) emphasis(e *ast.Emphasis) {
	// Level 1 is italic, level 2 is bold; writing both for a strong run is how
	// **fett** turned into ***fett*** on the way back out.
	bold := e.Level >= 2
	for c := e.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			b.run(string(v.Segment.Value(b.src)), runProps{bold: bold, italic: !bold})
		case *ast.CodeSpan:
			b.spanCode(v)
		case *ast.Emphasis:
			b.emphasis(v)
		default:
			b.inline(c)
		}
	}
}

// spanCode writes inline code in the Code style.
func (b *builder) spanCode(n ast.Node) {
	var out strings.Builder
	var buf []byte
	switch v := n.(type) {
	case *ast.CodeSpan:
		for c := v.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				buf = append(buf, t.Segment.Value(b.src)...)
			}
		}
	}
	out.Write(buf)
	b.out.WriteString(`<w:r><w:rPr><w:rStyle w:val="CodeChar"/></w:rPr><w:t xml:space="preserve">`)
	b.out.WriteString(escape(out.String()))
	b.out.WriteString("</w:t></w:r>")
}

// runProps are the run properties a piece of text carries.
type runProps struct {
	bold   bool
	italic bool
	strike bool
}

// run writes one run.
func (b *builder) run(s string, p runProps) {
	if s == "" {
		return
	}
	b.out.WriteString("<w:r>")
	if p.bold || p.italic || p.strike {
		b.out.WriteString("<w:rPr>")
		// The order inside w:rPr is fixed by the schema: b, i, strike. A property
		// after a later one is ignored by Word without a word.
		if p.bold {
			b.out.WriteString("<w:b/>")
		}
		if p.italic {
			b.out.WriteString("<w:i/>")
		}
		if p.strike {
			b.out.WriteString("<w:strike/>")
		}
		b.out.WriteString("</w:rPr>")
	}
	b.out.WriteString(`<w:t xml:space="preserve">` + escape(s) + `</w:t></w:r>`)
}

// escape turns text into XML content. A stray ampersand or an angle bracket in
// the Markdown would otherwise end the part.
func escape(s string) string {
	var out strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			out.WriteString("&amp;")
		case '<':
			out.WriteString("&lt;")
		case '>':
			out.WriteString("&gt;")
		case '"':
			out.WriteString("&quot;")
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// documentXML wraps the body into the main part.
func documentXML(body string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"` +
		// r: is what w:hyperlink needs for its r:id; without the declaration the
		// part is invalid and the file is refused (measured with LibreOffice:
		// "source file could not be loaded" as soon as one link was inside).
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
		// wp: is what w:drawing needs; without it the part is invalid and
		// LibreOffice refuses the file as soon as one picture is inside.
		` xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">` +
		`<w:body>` + body +
		`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/>` +
		`<w:pgMar w:top="1417" w:right="1417" w:bottom="1417" w:left="1417"/>` +
		`</w:sectPr></w:body></w:document>`
}

func contentTypes(imageExts []string) string {
	// Every extension an image is stored under needs its content type, or Word
	// calls the file damaged and says nothing about which part was wrong.
	seen := map[string]bool{}
	var extra strings.Builder
	for _, e := range imageExts {
		if seen[e] {
			continue
		}
		seen[e] = true
		extra.WriteString(`<Default Extension="` + e + `" ContentType="image/` + e + `"/>`)
	}
	return contentTypesHead + extra.String() + contentTypesTail
}

const contentTypesHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
	`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
	`<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>`

const contentTypesTail = `</Types>`

const packageRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
	`</Relationships>`

// documentRels has to name the styles, the numbering part and every hyperlink:
// a part that is in the package but in no relationship is invisible to Word, and
// a w:hyperlink whose r:id is missing here is dead.
func documentRels(links []string, images []imagePart) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	b.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	b.WriteString(`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>`)
	for i, target := range links {
		b.WriteString(`<Relationship Id="rId` + strconv.Itoa(linkBase+i) +
			`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="` +
			escape(target) + `" TargetMode="External"/>`)
	}
	for i, img := range images {
		b.WriteString(`<Relationship Id="rId` + strconv.Itoa(linkBase+len(links)+i) +
			`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="` +
			img.name + `"/>`)
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

// stylesXML defines every style the writer names. A w:pStyle that points at a
// style which does not exist leaves the paragraph plain, silently.
const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
	`<w:docDefaults><w:rPrDefault><w:rPr>` +
	`<w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:cs="Calibri"/>` +
	`<w:sz w:val="22"/><w:szCs w:val="22"/><w:lang w:val="de-DE"/>` +
	`</w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="120"/></w:pPr></w:pPrDefault></w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>` +
	headingStyles +
	`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:pPr><w:contextualSpacing/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/><w:rPr><w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/><w:sz w:val="19"/></w:rPr><w:pPr><w:spacing w:after="0"/></w:pPr></w:style>` +
	`<w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:uiPriority w:val="99"/><w:unhideWhenUsed/><w:rPr><w:color w:val="0563C1"/><w:u w:val="single"/></w:rPr></w:style>` +
	`<w:style w:type="character" w:styleId="CodeChar"><w:name w:val="Code Char"/><w:rPr><w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:rPr><w:i/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="TableHeader"><w:name w:val="Table Header"/><w:basedOn w:val="Normal"/><w:rPr><w:b/></w:rPr><w:pPr><w:spacing w:after="0"/></w:pPr></w:style>` +
	`<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/></w:style>` +
	`</w:styles>`

// headingStyles writes Heading1 to Heading6, each one smaller than the last.
const headingStyles = `<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:rPr><w:b/><w:sz w:val="32"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:rPr><w:b/><w:sz w:val="28"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:rPr><w:b/><w:sz w:val="26"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading4"><w:name w:val="heading 4"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:rPr><w:b/><w:i/><w:sz w:val="24"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading5"><w:name w:val="heading 5"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:rPr><w:b/><w:sz w:val="22"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading6"><w:name w:val="heading 6"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:rPr><w:b/><w:i/><w:sz w:val="22"/></w:rPr></w:style>`

// numberingXML defines the two lists: numId 2 is the bullet list, numId 3 the
// numbered one. Every abstractNum has to come before the first w:num, or Word
// reports the numbering part as damaged.
var numberingXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
	`<w:abstractNum w:abstractNumId="0">` +
	bulletLevels +
	`</w:abstractNum>` +
	`<w:abstractNum w:abstractNumId="1">` +
	decimalLevels +
	`</w:abstractNum>` +
	`<w:num w:numId="2"><w:abstractNumId w:val="0"/></w:num>` +
	`<w:num w:numId="3"><w:abstractNumId w:val="1"/></w:num>` +
	`</w:numbering>`

// bulletLevels is nine levels of bullets. The bullet character comes from the
// Symbol font as U+F0B7, which is what Word itself writes.
var bulletLevels = levels(func(i int) string {
	chars := []string{"", "o", "\u25aa"}
	ch := chars[i%3]
	if ch == "" {
		ch = "\uf0b7"
	}
	return `<w:lvl w:ilvl="` + strconv.Itoa(i) + `"><w:start w:val="1"/><w:numFmt w:val="bullet"/>` +
		`<w:lvlText w:val="` + ch + `"/><w:lvlJc w:val="left"/>` +
		`<w:pPr><w:ind w:left="` + strconv.Itoa(720*(i+1)) + `" w:hanging="360"/></w:pPr>` +
		`<w:rPr><w:rFonts w:ascii="Symbol" w:hAnsi="Symbol" w:hint="default"/></w:rPr></w:lvl>`
})

// decimalLevels is nine levels of numbers: 1., then 1.1., then 1.1.1.
var decimalLevels = levels(func(i int) string {
	text := ""
	for k := 0; k <= i; k++ {
		text += "%" + strconv.Itoa(k+1) + "."
	}
	return `<w:lvl w:ilvl="` + strconv.Itoa(i) + `"><w:start w:val="1"/><w:numFmt w:val="decimal"/>` +
		`<w:lvlText w:val="` + text + `"/><w:lvlJc w:val="left"/>` +
		`<w:pPr><w:ind w:left="` + strconv.Itoa(720*(i+1)) + `" w:hanging="360"/></w:pPr></w:lvl>`
})

// levels builds nine level definitions.
func levels(f func(i int) string) string {
	var b strings.Builder
	for i := 0; i < 9; i++ {
		b.WriteString(f(i))
	}
	return b.String()
}
