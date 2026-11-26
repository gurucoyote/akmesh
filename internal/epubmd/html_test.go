package epubmd

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/gurucoyote/akmesh/internal/epubgen"
)

// bookOf writes an EPUB through our own writer, so the reader is tested against
// the package this repository produces.
func bookOf(t *testing.T, markdown, baseDir string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := epubgen.Convert(&buf, []byte(markdown), epubgen.Options{Title: "Probe", BaseDir: baseDir}); err != nil {
		t.Fatalf("epubgen: %v", err)
	}
	return buf.Bytes()
}

func testImage(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 8))
	for x := range 16 {
		for y := range 8 {
			img.Set(x, y, color.RGBA{R: 90, G: 20, B: 20, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png: %v", err)
	}
	return buf.Bytes()
}

// TestHTMLKeepsTheChaptersAndLeavesTheColophon: reading a book as HTML gives the
// chapters the author wrote, not the licence page our writer adds.
func TestHTMLKeepsTheChaptersAndLeavesTheColophon(t *testing.T) {
	page, _, err := HTML(bookOf(t, "# Eins\n\nText eins.\n\n# Zwei\n\nText zwei.\n", ""))
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{"<h1>Eins</h1>", "<p>Text eins.</p>", "<h1>Zwei</h1>", "<p>Text zwei.</p>"} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "Schriften und Lizenzen") {
		t.Errorf("the licence page travelled along:\n%s", page)
	}
	if strings.Index(page, "Text eins.") > strings.Index(page, "Text zwei.") {
		t.Errorf("the chapters are out of order")
	}
	// A web page, not a book with a table of contents.
	if strings.Contains(page, `epub:type="toc"`) {
		t.Errorf("the nav document was read as a chapter")
	}
}

// TestHTMLCarriesPicturesTheWayOut: a picture inside a book has to be written
// beside the page, or the page points at nothing.
func TestHTMLCarriesPicturesTheWayOut(t *testing.T) {
	dir := t.TempDir()
	png := testImage(t)
	if err := os.WriteFile(dir+"/bild.png", png, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	book := bookOf(t, "# Titel\n\n![Punkt](bild.png)\n", dir)

	// Without a folder the description stands in for the picture.
	plain, images, err := HTML(book)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("pictures were handed back without a folder")
	}
	if !strings.Contains(plain, "[Punkt]") || strings.Contains(plain, "<img") {
		t.Errorf("a dead picture reference survived:\n%s", plain)
	}

	// With a folder the page points at the copy and the bytes come along.
	out, images, err := HTMLInto(book, dir)
	if err != nil {
		t.Fatalf("HTMLInto: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("got %d pictures, want 1", len(images))
	}
	if string(images[0].Data) != string(png) {
		t.Errorf("the picture changed on the way out")
	}
	if !strings.Contains(out, `<img src="images/`+images[0].Name+`" alt="Punkt"/>`) {
		t.Errorf("the page does not point at the copy:\n%s", out)
	}
}

// TestHTMLOfAnEmptySpineIsAnError: a book without a readable chapter is not an
// empty page, it is a broken book.
func TestHTMLOfAnEmptySpineIsAnError(t *testing.T) {
	if _, _, err := HTML([]byte("not a zip")); err == nil {
		t.Errorf("a non-book was read without complaint")
	}
}
