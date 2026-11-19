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
	"html"
	"io"
	"strings"

	"github.com/go-swiss/fonts"
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
			if c.Kind().String() == "Text" || c.Kind().String() == "String" {
				b.Write(c.Text(source))
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
		FooterFunc: func(impl pdf.Fpdf, cache fonts.Cache) func() {
			// Room for the page line below the text column.
			impl.Fpdf.SetAutoPageBreak(true, 42)
			return footer(impl.Fpdf)
		},
	}
	if title != "" {
		cfg.HeaderFunc = func(impl pdf.Fpdf, cache fonts.Cache) func() {
			return header(impl.Fpdf, title)
		}
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRenderer(pdf.New(
			pdf.WithFpdf(context.Background(), cfg),
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
	_, err := w.Write(buf.Bytes())
	return err
}

// header draws the title above a hairline. The font is one of the fourteen that
// every PDF reader must have, so the header costs nothing in file size.
func header(f *gofpdf.Fpdf, title string) func() {
	return func() {
		// Not on the first page: its title is already the opening heading, and
		// printing it twice on one page reads like a mistake.
		if f.PageNo() == 1 {
			return
		}
		f.SetFont("Helvetica", "", 8)
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
		// A base fourteen font is written in its own encoding, so the UTF-8
		// title has to be translated or the umlauts come out as two latin-1
		// bytes each ("MÃ¤rchen").
		f.CellFormat(width, 10, f.UnicodeTranslatorFromDescriptor("")(title), "", 0, "L", false, 0, "")
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
		f.SetFont("Helvetica", "", 8)
		f.SetTextColor(120, 120, 120)
		left, _, right, _ := f.GetMargins()
		pageW, _ := f.GetPageSize()
		f.SetXY(left, -24)
		f.CellFormat(pageW-left-right, 10, fmt.Sprintf("Seite %d von {nb}", f.PageNo()), "", 0, "C", false, 0, "")
		f.SetTextColor(0, 0, 0)
	}
}
