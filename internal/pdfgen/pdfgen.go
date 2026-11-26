// Package pdfgen turns Markdown into a PDF.
//
// The renderer is goldmark-pdf, but not the module as published: chroma and its
// whole lexer set come with it for syntax colours alone, and the scripts pull
// the Google font files over HTTP at run time. Both were removed; the changes
// are in docs/design/patches/, applied to the module cache by
// scripts/apply-goldmark-pdf-fork.sh.
//
// What that costs: a code block is set as a monospaced box without colours, and
// the fonts come from go-swiss as far as that stage is kept. Measured sizes:
// upstream 12.218.530 bytes, this fork 9.097.378.
//
// Header and footer follow what go-pdfkit/html2pdf does with the paged media
// margin boxes: the title runs above a hairline, the page count below, and the
// total comes from fpdf's {nb} alias, which is resolved when the last page is
// written. Both use a base fourteen font, so neither adds bytes to the file.
package pdfgen

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gurucoyote/akmesh/internal/webfonts"
	"html"
	"io"
	"strings"

	"github.com/phpdave11/gofpdf"
	pdf "github.com/stephenafamo/goldmark-pdf"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// bandHeight is the space the title band needs above the text column: the band
// itself plus a little air below the hairline.
const bandHeight = 42.0

// firstTitle returns the first level one heading of the document, which is what
// the running header shows. Empty when the document has none, and then there is
// no header at all: a header repeating a file name would be noise.
func firstTitle(source []byte) string {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := md.Parser().Parse(text.NewReader(source), parser.WithContext(parser.NewContext()))
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		h, ok := n.(*ast.Heading)
		if !ok || h.Level != 1 {
			continue
		}
		var b strings.Builder
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch t := c.(type) {
			case *ast.Text:
				b.Write(t.Segment.Value(source))
			case *ast.String:
				b.Write(t.Value)
			}
		}
		return strings.TrimSpace(html.UnescapeString(b.String()))
	}
	return ""
}

// Convert writes the PDF of a Markdown document.
//
// One renderer per document: goldmark-pdf carries per-document state (page
// breaks, fonts, running styles) and reusing one would bleed a previous
// document into the next.
func Convert(w io.Writer, source []byte) error {
	title := firstTitle(source)
	cfg := pdf.FpdfConfig{
		Title:     title,
		PaperSize: "A4",
		FooterFunc: func(impl pdf.Fpdf) func() {
			// Room for the page line below the text column.
			impl.Fpdf.SetAutoPageBreak(true, 42)
			return footer(impl.Fpdf)
		},
	}
	if title != "" {
		cfg.HeaderFunc = func(impl pdf.Fpdf) func() {
			return header(impl.Fpdf, title)
		}
	}

	ctx := context.Background()
	impl, styles, err := fontSetup(ctx, cfg)
	if err != nil {
		return err
	}
	goldcfg := pdf.DefaultConfig()
	goldcfg.Styles = styles
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRenderer(pdf.New(
			// Order matters: WithConfig replaces the whole configuration,
			// so it must come before WithPDF or it throws our PDF away and
			// a default one is made - the render then selects a core font
			// and gofpdf writes through an empty glyph table.
			pdf.WithConfig(goldcfg),
			pdf.WithPDF(impl),
			// A PDF is not HTML: without this every & becomes &amp; in the page
			// text, which the reader then reads out loud.
			pdf.WithEscapeHTML(false),
		)),
	)
	// Render into memory first: a half written file is worse than a clear error.
	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return err
	}
	if impl.Fpdf.Err() {
		// gofpdf records the error and carries on; a document written after
		// an error is broken, so it is reported as one.
		return impl.Fpdf.Error()
	}
	_, err = w.Write(buf.Bytes())
	return err
}

// pageFamily is the face the running title and the page number are set in: the
// same as the body, so the page has one look.
func pageFamily() string {
	book := webfonts.Book()
	if len(book) == 0 {
		return "Helvetica"
	}
	return book[0].Name
}

// header draws the title above a hairline.
func header(f *gofpdf.Fpdf, title string) func() {
	return func() {
		// Not on the first page: its title is already the opening heading, and
		// printing it twice on one page reads like a mistake.
		if f.PageNo() == 1 {
			return
		}
		f.SetFont(pageFamily(), "", 8)
		f.SetTextColor(120, 120, 120)
		// The header sits in the band above the text column, and its width is
		// the column width, so it lines up with the body.
		left, _, right, _ := f.GetMargins()
		pageW, _ := f.GetPageSize()
		width := pageW - left - right
		// The renderer starts its own text column at about 34pt from the top
		// regardless of the top margin, so the title band is placed above that
		// rather than below a margin it ignores.
		f.SetXY(left, 10)
		// The title goes in as UTF-8: the header is set in an embedded UTF-8
		// font, and translating it first would encode it twice - "Titel über
		// Öl" came out as "Titel 좥r ֬" when it was done (measured 2026-11-19).
		f.CellFormat(width, 10, title, "", 0, "L", false, 0, "")
		f.SetDrawColor(180, 180, 180)
		f.SetLineWidth(0.3)
		f.Line(left, 20, left+width, 20)
		// After a page break the renderer carries on from wherever y stands, so
		// the band has to end with y at the top of the text column.
		f.SetY(bandHeight)
		f.SetTextColor(0, 0, 0)
	}
}

// footer prints "Seite N von M" centred at the bottom of every page. {nb} is
// the total, known only once the last page exists, which is why it is written as
// an alias and resolved by fpdf at output time.
func footer(f *gofpdf.Fpdf) func() {
	f.AliasNbPages("{nb}")
	return func() {
		f.SetFont(pageFamily(), "", 8)
		f.SetTextColor(120, 120, 120)
		left, _, right, _ := f.GetMargins()
		pageW, _ := f.GetPageSize()
		f.SetXY(left, -24)
		f.CellFormat(pageW-left-right, 10, fmt.Sprintf("Seite %d von {nb}", f.PageNo()), "", 0, "C", false, 0, "")
		f.SetTextColor(0, 0, 0)
	}
}
