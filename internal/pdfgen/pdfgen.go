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
package pdfgen

import (
	"io"

	"github.com/stephenafamo/goldmark-pdf"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// Convert writes the PDF of a Markdown document.
//
// One renderer per document: goldmark-pdf carries per-document state (page
// breaks, fonts, running styles) and reusing one would bleed a previous
// document into the next.
func Convert(w io.Writer, source []byte) error {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRenderer(pdf.New()),
	)
	return md.Convert(source, w)
}
