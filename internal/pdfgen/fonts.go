// Package pdfgen turns Markdown into a PDF.
//
// The fonts are the document's own: Lora for text, Go Mono for code, taken from
// internal/webfonts and handed to gofpdf as bytes. Nothing is downloaded while
// rendering - the goldmark-pdf fork has no Google Fonts path left in it (see
// scripts/apply-goldmark-pdf-fork.sh), so a document renders the same on a
// machine without a network and without installed fonts.
package pdfgen

import (
	"context"

	pdf "github.com/stephenafamo/goldmark-pdf"

	"github.com/gurucoyote/akmesh/internal/webfonts"
)

// fontSetup registers the embedded programs and returns the styles that use
// them. The names it registers are the ones the styles ask for; a mismatch
// shows up as gofpdf's "undefined font".
func fontSetup(ctx context.Context, cfg pdf.FpdfConfig) (*pdf.Fpdf, pdf.Styles, error) {
	impl := pdf.NewFpdf(ctx, cfg)
	families := webfonts.Book()
	for _, fam := range families {
		// Every style a document can ask for must exist, or gofpdf fails in
		// the middle of a page. A missing bold face falls back to the regular
		// one: a bolder-looking code sample is better than a broken save.
		byStyle := map[string][]byte{}
		var regular []byte
		for _, face := range fam.Faces {
			switch face.Style {
			case webfonts.Regular:
				regular = face.TTF
				byStyle[pdf.FontStyleRegular] = face.TTF
			case webfonts.Bold:
				byStyle[pdf.FontStyleBold] = face.TTF
			case webfonts.Italic:
				byStyle[pdf.FontStyleItalic] = face.TTF
			case webfonts.BoldItalic:
				byStyle[pdf.FontStyleBoldItalic] = face.TTF
			}
		}
		if regular == nil {
			continue
		}
		for _, style := range []string{
			pdf.FontStyleRegular, pdf.FontStyleBold,
			pdf.FontStyleItalic, pdf.FontStyleBoldItalic,
		} {
			data := byStyle[style]
			if data == nil {
				data = regular
			}
			if err := impl.AddFont(fam.Name, style, data); err != nil {
				return nil, pdf.Styles{}, err
			}
		}
	}

	st := pdf.DefaultStyles()
	if len(families) > 0 {
		body := customFont(families[0].Name)
		for _, s := range []*pdf.Style{st.H1, st.H2, st.H3, st.H4, st.H5, st.H6, st.Normal, st.Blockquote, st.THeader, st.TBody} {
			if s != nil {
				s.Font = body
			}
		}
		if len(families) > 1 {
			mono := customFont(families[1].Name)
			st.CodeFont = mono
			// The code block has a style of its own: leaving it on the
			// default would ask for a font that is no longer in the build,
			// and gofpdf then writes through an empty glyph table.
			if st.CodeBlock != nil {
				st.CodeBlock.Font = mono
			}
		}
	}
	return impl, st, nil
}

// customFont is a font the caller registered itself; goldmark-pdf then skips
// its own loading for it.
func customFont(family string) pdf.Font {
	return pdf.Font{
		CanUseForText: true,
		CanUseForCode: true,
		Category:      "sans-serif",
		Family:        family,
		Type:          pdf.FontTypeCustom,
	}
}
