package docxmd

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"

	"akmesh/internal/docxgen"
)

// docxWithPicture builds a document the way our own writer does, so the reader
// is tested against the markup that actually leaves this repository.
func docxWithPicture(t *testing.T, markdown string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := docxgen.Convert(&buf, []byte(markdown)); err != nil {
		t.Fatalf("docxgen: %v", err)
	}
	return buf.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 30, G: 60, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png: %v", err)
	}
	return buf.Bytes()
}

// pictureDocx is a document with one picture of known size.
func pictureDocx(t *testing.T) (docx []byte, pngLen int) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/bild.png", pngBytes(t, 24, 12), 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}
	var buf bytes.Buffer
	if err := docxgen.ConvertWithBase(&buf, []byte("Text davor.\n\n![Ein blaues Feld](bild.png)\n\nText danach.\n"), dir); err != nil {
		t.Fatalf("docxgen: %v", err)
	}
	return buf.Bytes(), len(pngBytes(t, 24, 12))
}

// TestPictureComesOutOfThePackage is the point of the feature: the picture the
// document carries is handed back, and the Markdown points at it.
func TestPictureComesOutOfThePackage(t *testing.T) {
	docx, wantLen := pictureDocx(t)
	md, images, err := ConvertTo(docx, t.TempDir())
	if err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("got %d pictures, want 1: %v", len(images), md)
	}
	if len(images[0].Data) != wantLen {
		t.Errorf("picture is %d bytes, want %d", len(images[0].Data), wantLen)
	}
	if images[0].Name != "bild1.png" {
		t.Errorf("picture is called %q", images[0].Name)
	}
	for _, want := range []string{"![Ein blaues Feld](bild1.png)", "Text davor.", "Text danach."} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
}

// TestPictureWithoutFolderStaysADescription: printing to standard output has no
// folder to put the file in, so the reference would be a link to nothing.
func TestPictureWithoutFolderStaysADescription(t *testing.T) {
	docx, _ := pictureDocx(t)
	md, err := Convert(docx)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(md, "[Ein blaues Feld]") {
		t.Errorf("the description is gone:\n%s", md)
	}
	if strings.Contains(md, "![") {
		t.Errorf("a picture reference was written without a picture:\n%s", md)
	}
}

// TestTwoPicturesKeepTheirOwnFiles: two drawings must not land on one name.
func TestTwoPicturesKeepTheirOwnFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/a.png", pngBytes(t, 10, 10), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var buf bytes.Buffer
	if err := docxgen.ConvertWithBase(&buf, []byte("![eins](a.png)\n\n![zwei](a.png)\n"), dir); err != nil {
		t.Fatalf("docxgen: %v", err)
	}
	md, images, err := ConvertTo(buf.Bytes(), t.TempDir())
	if err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("got %d pictures, want 2", len(images))
	}
	if images[0].Name == images[1].Name {
		t.Errorf("both pictures are called %q:\n%s", images[0].Name, md)
	}
}

// TestInlineCodeBecomesBackticks: a run set in the code character style is code
// on the way out as well, or the meaning of the text is lost.
func TestInlineCodeBecomesBackticks(t *testing.T) {
	docx := docxWithPicture(t, "Ein `inline` Code und **fett**.\n")
	md, err := Convert(docx)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(md, "`inline`") {
		t.Errorf("inline code is not marked:\n%s", md)
	}
	if !strings.Contains(md, "**fett**") {
		t.Errorf("bold is gone:\n%s", md)
	}
	if !strings.Contains(md, "Ein ") {
		t.Errorf("the prose around the code is gone:\n%s", md)
	}
}

// TestMonospaceFaceIsCode: Word puts the face on the run when the style is
// applied directly, so a face alone has to count.
func TestMonospaceFaceIsCode(t *testing.T) {
	body := `<w:p><w:r><w:t>vor </w:t></w:r><w:r><w:rPr><w:rFonts w:ascii="Consolas"/></w:rPr><w:t>x := 1</w:t></w:r></w:p>`
	md, err := Convert(makeDocx(t, body, ""))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(md, "`x := 1`") {
		t.Errorf("a monospace run is not code:\n%s", md)
	}
}

// TestProseIsNotCode keeps the rule from firing on ordinary text: a font that
// merely sounds technical is still prose.
func TestProseIsNotCode(t *testing.T) {
	body := `<w:p><w:r><w:rPr><w:rFonts w:ascii="Georgia"/></w:rPr><w:t>Ein Satz.</w:t></w:r></w:p>`
	md, err := Convert(makeDocx(t, body, ""))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if strings.Contains(md, "`") {
		t.Errorf("prose was marked as code:\n%s", md)
	}
}
