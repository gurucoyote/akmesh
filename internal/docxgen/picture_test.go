package docxgen

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// pngData is a real PNG of the given size: the converter decides by decoding,
// so a fake header would not do.
func pngData(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png: %v", err)
	}
	return buf.Bytes()
}

// convertWithBase writes a document whose images live in dir.
func convertWithBase(t *testing.T, markdown, dir string) map[string]string {
	t.Helper()
	var buf bytes.Buffer
	if err := ConvertWithBase(&buf, []byte(markdown), dir); err != nil {
		t.Fatalf("convert: %v", err)
	}
	return unzip(t, buf.Bytes())
}

// TestImagesAreEmbedded is the point of the feature: the picture travels in the
// package, with its relationship, its content type and its size in EMU.
func TestImagesAreEmbedded(t *testing.T) {
	dir := t.TempDir()
	png := pngData(t, 60, 30)
	if err := os.WriteFile(filepath.Join(dir, "bild.png"), png, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	parts := convertWithBase(t, "# Titel\n\nText.\n\n![Ein rotes Rechteck](bild.png)\n", dir)

	media := parts["word/media/bild1.png"]
	if media == "" {
		t.Fatalf("no picture in the package: %v", keysOf(parts))
	}
	if len(media) != len(string(png)) {
		t.Errorf("picture is %d bytes, want %d", len(media), len(png))
	}
	if got := parts["word/_rels/document.xml.rels"]; !strings.Contains(got, "relationships/image") ||
		!strings.Contains(got, `Target="media/bild1.png"`) {
		t.Errorf("no image relationship: %s", got)
	}
	if got := parts["[Content_Types].xml"]; !strings.Contains(got, `Extension="png" ContentType="image/png"`) {
		t.Errorf("no content type for png: %s", got)
	}
	doc := parts["word/document.xml"]
	// 60 x 30 pixels at 9525 EMU per pixel.
	for _, want := range []string{`cx="571500"`, `cy="285750"`, `descr="Ein rotes Rechteck"`, "<w:drawing>", `r:embed="rId3"`} {
		if !strings.Contains(doc, want) {
			t.Errorf("document lacks %q", want)
		}
	}
	// A drawing inside an already open paragraph is a nested paragraph, which
	// LibreOffice then refuses to show.
	if strings.Contains(doc, "<w:p><w:p>") {
		t.Errorf("nested paragraph around the drawing")
	}
}

// TestMissingImageKeepsItsDescription: a picture that cannot be read leaves the
// replacement text, so the document still says what was there.
func TestMissingImageKeepsItsDescription(t *testing.T) {
	dir := t.TempDir()
	parts := convertWithBase(t, "![Fehlt](gibtsnicht.png)\n", dir)
	if got := parts["word/media/bild1.png"]; got != "" {
		t.Errorf("a missing picture was embedded anyway")
	}
	if !strings.Contains(parts["word/document.xml"], "[Fehlt]") {
		t.Errorf("the description is gone: %s", parts["word/document.xml"])
	}
}

// TestSVGIsNotEmbedded: Word cannot show an SVG planted in a media part, so it
// stays text instead of becoming a broken picture.
func TestSVGIsNotEmbedded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bild.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	parts := convertWithBase(t, "![Kreis](bild.svg)\n", dir)
	if got := parts["word/media/bild1.svg"]; got != "" {
		t.Errorf("an SVG was embedded")
	}
	if !strings.Contains(parts["word/document.xml"], "[Kreis]") {
		t.Errorf("the description is gone")
	}
}

// TestWideImageIsScaledToThePage: a photo wider than the text column is scaled
// down, and the height follows the aspect ratio.
func TestWideImageIsScaledToThePage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "breit.png"), pngData(t, 2000, 1000), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	parts := convertWithBase(t, "![Breit](breit.png)\n", dir)
	doc := parts["word/document.xml"]
	if !strings.Contains(doc, `cx="`+strconv.Itoa(contentWidthEMU)+`"`) {
		t.Errorf("the picture was not capped to the text width")
	}
	// Half the aspect ratio: 1000/2000 of the width.
	if !strings.Contains(doc, `cy="`+strconv.Itoa(contentWidthEMU/2)+`"`) {
		t.Errorf("the height does not follow the aspect ratio")
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
