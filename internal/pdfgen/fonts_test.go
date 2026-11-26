package pdfgen

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	pdf "github.com/stephenafamo/goldmark-pdf"

	"github.com/gurucoyote/akmesh/internal/webfonts"
)

// TestPDFCarriesOurFonts is the point of the port: the document is set in Lora
// and Go Mono and brings both programs with it. A PDF that merely names a font
// looks right on this machine and wrong on every other.
func TestPDFCarriesOurFonts(t *testing.T) {
	var buf bytes.Buffer
	src := "# Titel\n\nEin Absatz mit `Code` darin.\n\n```\nfunc main() {}\n```\n"
	if err := Convert(&buf, []byte(src)); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	doc := buf.String()
	if !strings.Contains(doc, "/FontFile") {
		t.Errorf("the PDF embeds no font program")
	}
	bases := regexp.MustCompile(`/BaseFont\s*/([A-Za-z0-9+\-, ]+)`).FindAllStringSubmatch(doc, -1)
	var names []string
	for _, m := range bases {
		names = append(names, m[1])
	}
	joined := strings.Join(names, " ")
	if !strings.Contains(joined, "lora") {
		t.Errorf("Lora is not in the document: %v", names)
	}
	if !strings.Contains(joined, "go") {
		t.Errorf("Go Mono is not in the document: %v", names)
	}
	// The faces the old build downloaded are gone with the Google path.
	for _, gone := range []string{"Roboto", "Helvetica"} {
		if strings.Contains(joined, gone) {
			t.Errorf("the document still names %s: %v", gone, names)
		}
	}
}

// TestEveryStyleUsesOurFonts guards the trap that cost a panic: a style left on
// the library's default asks for a font that is no longer in the build, and
// gofpdf then writes through an empty glyph table. No style may point elsewhere.
func TestEveryStyleUsesOurFonts(t *testing.T) {
	impl, styles, err := fontSetup(t.Context(), pdf.FpdfConfig{PaperSize: "A4"})
	if err != nil {
		t.Fatalf("fontSetup: %v", err)
	}
	_ = impl
	book := webfonts.Book()
	allowed := map[string]bool{}
	for _, fam := range book {
		allowed[fam.Name] = true
	}
	check := func(name string, f pdf.Font) {
		if !allowed[f.Family] {
			t.Errorf("style %s uses %q, which is not registered", name, f.Family)
		}
		if f.Type != pdf.FontTypeCustom {
			t.Errorf("style %s has type %q, so the library would load it itself", name, f.Type)
		}
	}
	check("H1", styles.H1.Font)
	check("H2", styles.H2.Font)
	check("H3", styles.H3.Font)
	check("H4", styles.H4.Font)
	check("H5", styles.H5.Font)
	check("H6", styles.H6.Font)
	check("Normal", styles.Normal.Font)
	check("Blockquote", styles.Blockquote.Font)
	check("THeader", styles.THeader.Font)
	check("TBody", styles.TBody.Font)
	check("CodeFont", styles.CodeFont)
	check("CodeBlock", styles.CodeBlock.Font)
}

// TestFontsComeFromTheBook: the faces handed to gofpdf are the ones we ship,
// with every style present, so no style can go missing in the middle of a page.
func TestFontsComeFromTheBook(t *testing.T) {
	impl, _, err := fontSetup(t.Context(), pdf.FpdfConfig{PaperSize: "A4"})
	if err != nil {
		t.Fatalf("fontSetup: %v", err)
	}
	if len(webfonts.Book()) < 2 {
		t.Fatalf("the book has %d families, want text and code", len(webfonts.Book()))
	}
	// gofpdf swallows a failed lookup and breaks later; a font that was
	// registered must be selectable without error.
	for _, fam := range webfonts.Book() {
		for _, style := range []string{pdf.FontStyleRegular, pdf.FontStyleBold, pdf.FontStyleItalic, pdf.FontStyleBoldItalic} {
			impl.Fpdf.SetFont(fam.Name, style, 11)
			if impl.Fpdf.Err() {
				t.Fatalf("SetFont(%s, %q): %v", fam.Name, style, impl.Fpdf.Error())
			}
		}
	}
}
