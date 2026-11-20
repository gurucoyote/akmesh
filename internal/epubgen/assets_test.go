package epubgen

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bookParts opens a finished book and returns its entries as text. Binary
// entries come back as a string too; only their length matters here.
func bookParts(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		var buf bytes.Buffer
		buf.ReadFrom(rc)
		rc.Close()
		out[f.Name] = buf.String()
	}
	return out
}

func makeBook(t *testing.T, markdown, baseDir string) map[string]string {
	t.Helper()
	var buf bytes.Buffer
	if err := Convert(&buf, []byte(markdown), Options{Title: "Probe", BaseDir: baseDir}); err != nil {
		t.Fatalf("convert: %v", err)
	}
	return bookParts(t, buf.Bytes())
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 10, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png: %v", err)
	}
	return buf.Bytes()
}

// TestBookCarriesItsFonts: the programs travel in the package, the stylesheet
// names them by their file names, and the reader is not asked to have them.
func TestBookCarriesItsFonts(t *testing.T) {
	parts := makeBook(t, "# Kapitel\n\nText.\n", "")
	var fonts []string
	for name := range parts {
		if strings.HasPrefix(name, "OEBPS/fonts/") {
			fonts = append(fonts, name)
		}
	}
	if len(fonts) == 0 {
		t.Fatalf("no fonts in the book: %v", keysOf(parts))
	}
	css := parts["OEBPS/style.css"]
	for _, name := range fonts {
		// The rule has to point at the file that is actually there, or the
		// reader silently falls back to a font it happens to have.
		if !strings.Contains(css, `url("`+strings.TrimPrefix(name, "OEBPS/")+`")`) {
			t.Errorf("style.css does not reference %s", name)
		}
		if !strings.Contains(parts["OEBPS/package.opf"], `href="`+strings.TrimPrefix(name, "OEBPS/")+`"`) {
			t.Errorf("%s is not in the manifest", name)
		}
	}
	if !strings.Contains(parts["OEBPS/package.opf"], `media-type="font/ttf"`) {
		t.Errorf("the fonts have no media type in the manifest")
	}
	// The body asks for the embedded family, not for a generic serif.
	if !strings.Contains(css, `font-family: "Lora", serif`) {
		t.Errorf("body font is not the embedded one: %s", css)
	}
	if !strings.Contains(css, `font-family: "Go Mono", monospace`) {
		t.Errorf("code font is not the embedded one")
	}
}

// TestLicenseTravelsWithTheFonts: the terms the fonts carry are in the book, on
// a page the reading order reaches.
func TestLicenseTravelsWithTheFonts(t *testing.T) {
	parts := makeBook(t, "# Kapitel\n\nText.\n", "")
	colophon := parts["OEBPS/colophon.xhtml"]
	if colophon == "" {
		t.Fatalf("no colophon in the book")
	}
	for _, want := range []string{"Schriften und Lizenzen", "Lora", "Go Mono", "Lizenz:"} {
		if !strings.Contains(colophon, want) {
			t.Errorf("colophon lacks %q", want)
		}
	}
	opf := parts["OEBPS/package.opf"]
	if !strings.Contains(opf, `href="colophon.xhtml"`) {
		t.Errorf("the colophon is not in the manifest")
	}
	if !strings.Contains(opf, `<itemref idref="colophon"/>`) {
		t.Errorf("the colophon is not in the reading order")
	}
}

// TestPicturesAreCopiedIntoTheBook: a reference to the author's disk is not a
// picture. The bytes belong in the package, and the chapters point at that copy.
func TestPicturesAreCopiedIntoTheBook(t *testing.T) {
	dir := t.TempDir()
	png := testPNG(t, 40, 20)
	if err := os.WriteFile(filepath.Join(dir, "bild.png"), png, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	parts := makeBook(t, "# Kapitel\n\n![Ein Quadrat](bild.png)\n", dir)

	media := parts["OEBPS/images/bild1.png"]
	if media == "" {
		t.Fatalf("no picture in the book: %v", keysOf(parts))
	}
	if len(media) != len(string(png)) {
		t.Errorf("picture is %d bytes, want %d", len(media), len(png))
	}
	chapter := parts["OEBPS/chapter1.xhtml"]
	if !strings.Contains(chapter, `<img src="images/bild1.png" alt="Ein Quadrat"/>`) {
		t.Errorf("the chapter does not point at the copy: %s", chapter)
	}
	opf := parts["OEBPS/package.opf"]
	if !strings.Contains(opf, `href="images/bild1.png" media-type="image/png"`) {
		t.Errorf("the picture has no manifest entry: %s", opf)
	}
}

// TestMissingPictureKeepsTheDescription: without the file the book still says
// what was there, and nothing in the package points at a hole.
func TestMissingPictureKeepsTheDescription(t *testing.T) {
	parts := makeBook(t, "# Kapitel\n\n![Fehlt](gibtsnicht.png)\n", t.TempDir())
	if got := parts["OEBPS/images/bild1.png"]; got != "" {
		t.Errorf("a missing picture was packed anyway")
	}
	chapter := parts["OEBPS/chapter1.xhtml"]
	if !strings.Contains(chapter, "[Fehlt]") {
		t.Errorf("the description is gone: %s", chapter)
	}
	if strings.Contains(chapter, "<img") {
		t.Errorf("a broken image tag survived: %s", chapter)
	}
}

// TestMimetypeStaysFirst: a reader checks the first entry before it reads
// anything else, and refuses the book when it is not stored uncompressed.
func TestMimetypeStaysFirst(t *testing.T) {
	var buf bytes.Buffer
	if err := Convert(&buf, []byte("# Kapitel\n"), Options{Title: "Probe"}); err != nil {
		t.Fatalf("convert: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	if len(zr.File) == 0 || zr.File[0].Name != "mimetype" {
		t.Fatalf("first entry is %q", zr.File[0].Name)
	}
	if zr.File[0].Method != zip.Store {
		t.Errorf("mimetype is compressed")
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
