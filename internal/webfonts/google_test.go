package webfonts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// googleCSS is a real answer of the CSS API for Open Sans in TTF form, cut down
// to two blocks. The shape is what matters: one block per subset, each with a
// unicode-range, and one url.
const googleCSS = `/* latin */
@font-face {
  font-family: 'Open Sans';
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.gstatic.com/s/opensans/v44/abc.ttf) format('truetype');
  unicode-range: U+0000-00FF, U+0131, U+0152-0153;
}
/* latin */
@font-face {
  font-family: 'Open Sans';
  font-style: normal;
  font-weight: 700;
  src: url(https://fonts.gstatic.com/s/opensans/v44/def.ttf) format('truetype');
  unicode-range: U+0000-00FF;
}
/* cyrillic */
@font-face {
  font-family: 'Open Sans';
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.gstatic.com/s/opensans/v44/cyr.ttf) format('truetype');
  unicode-range: U+0400-045F;
}`

// TestParseCSSTakesTheTTFAnswer is the regression that cost a real run: the TTF
// answer of the CSS API has one @font-face block and no unicode-range, and a
// filter that demanded the range threw the only font away.
const googleTTFCSS = `/* latin */
@font-face {
  font-family: 'Open Sans';
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.gstatic.com/s/opensans/v44/ttf.ttf) format('truetype');
}
/* latin */
@font-face {
  font-family: 'Open Sans';
  font-style: italic;
  font-weight: 400;
  src: url(https://fonts.gstatic.com/s/opensans/v44/ttf-i.ttf) format('truetype');
}`

func TestParseCSSTakesTheTTFAnswer(t *testing.T) {
	faces := parseCSS(googleTTFCSS)
	if len(faces) != 2 {
		t.Fatalf("faces: %d, want 2 from a TTF answer without unicode-range", len(faces))
	}
	if faces[1].style != "italic" {
		t.Errorf("second face is %q, want italic", faces[1].style)
	}
}

func TestParseCSSKeepsTheLatinFaces(t *testing.T) {
	faces := parseCSS(googleCSS)
	if len(faces) != 2 {
		t.Fatalf("faces: %d, want 2 (latin regular and bold)", len(faces))
	}
	if faces[0].family != "Open Sans" || faces[0].weight != "400" {
		t.Errorf("first face: %+v", faces[0])
	}
	if !strings.HasSuffix(faces[0].url, "abc.ttf") {
		t.Errorf("url: %q", faces[0].url)
	}
	for _, f := range faces {
		if strings.Contains(f.url, "cyr") {
			t.Errorf("a cyrillic block was taken: %+v", f)
		}
	}
}

// TestFetchRefusesWhatIsNoFont is the guard that matters: a 404 page, a woff2
// file or an unparseable body must not end up in the document as a font.
func TestFetchRefusesWhatIsNoFont(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   []byte
	}{
		{"404 mit HTML", http.StatusNotFound, []byte("<html>not found</html>")},
		{"HTML mit 200", http.StatusOK, []byte("<html><body>kein font</body></html>")},
		{"woff2-Kopf", http.StatusOK, []byte("wOF2\x00\x01\x00\x00rest")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "css") {
					w.WriteHeader(c.status)
					w.Write([]byte(googleCSS))
					return
				}
				w.WriteHeader(c.status)
				w.Write(c.body)
			}))
			defer srv.Close()
			g := Google{Client: srv.Client(), API: srv.URL + "/css2"}
			if _, err := g.Fetch(context.Background(), "Open Sans"); err == nil {
				t.Errorf("Fetch took an answer that holds no font")
			}
		})
	}
}

// TestFetchTakesARealFont: when the CSS and the bytes are sound, the family
// arrives with its faces.
func TestFetchTakesARealFont(t *testing.T) {
	ttf := Book()[0].Faces[0].TTF
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "css") {
			// The rule names a file; the test serves that file itself instead
			// of talking to gstatic.
			w.Write([]byte(strings.ReplaceAll(googleCSS, "https://fonts.gstatic.com", srv.URL)))
			return
		}
		w.Write(ttf)
	}))
	defer srv.Close()
	g := Google{Client: srv.Client(), API: srv.URL + "/css2"}
	fam, err := g.Fetch(context.Background(), "Open Sans")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if fam.Name != "Open Sans" {
		t.Errorf("family: %q", fam.Name)
	}
	if len(fam.Faces) != 2 {
		t.Fatalf("faces: %d, want regular and bold", len(fam.Faces))
	}
	if fam.Faces[1].Style != Bold {
		t.Errorf("second face is %v, want Bold", fam.Faces[1].Style)
	}
	// And the book keeps the code family.
	book := BookWith(fam)
	if book[0].Name != "Open Sans" || book[1].Name != MonoName {
		t.Errorf("book: %s + %s", book[0].Name, book[1].Name)
	}
}

// TestFSAllowsEmbeddingOfTheBundledSet: the licence gate has to pass for what we
// ship, otherwise nothing may be embedded at all.
func TestFSAllowsEmbeddingOfTheBundledSet(t *testing.T) {
	for _, fam := range Book() {
		for _, f := range fam.Faces {
			fs, ok := FS(f.TTF)
			if !ok {
				t.Errorf("%s %s: no fsType found", fam.Name, f.Style)
				continue
			}
			if fs&0x0002 != 0 {
				t.Errorf("%s %s: fsType %d forbids embedding", fam.Name, f.Style, fs)
			}
			if !valid(f.TTF) {
				t.Errorf("%s %s: valid() refused a bundled font", fam.Name, f.Style)
			}
		}
	}
}

// TestDetectFindsThePageFamily: the family the page asks for, with generic and
// system names skipped, and two-word names kept whole.
func TestDetectFindsThePageFamily(t *testing.T) {
	cases := []struct{ name, html, want string }{
		{"im style-Block", `<html><head><style>body { font-family: "Open Sans", sans-serif; }</style></head></html>`, "Open Sans"},
		{"generisch zuerst", `<style>body{font-family:sans-serif} h1{font-family:Lato}</style>`, "Lato"},
		{"Systemschrift", `<style>body{font-family:Arial, Helvetica, sans-serif}</style>`, ""},
		{"keine Angabe", `<html><body><p>Text</p></body></html>`, ""},
		{"einzelnes Wort", `<style>p{font-family: Georgia, serif}</style>`, ""},
		{"Fira Code", `<style>pre{font-family:'Fira Code', monospace}</style>`, "Fira Code"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Detect([]byte(c.html)); got != c.want {
				t.Errorf("Detect = %q, want %q", got, c.want)
			}
		})
	}
}

// TestFetchReportsUnknownFamily: a family Google does not know stays an error,
// so the caller falls back instead of writing something empty.
func TestFetchReportsUnknownFamily(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad family"))
	}))
	defer srv.Close()
	g := Google{Client: srv.Client(), API: srv.URL + "/css2"}
	if _, err := g.Fetch(context.Background(), "Gibtsnicht"); err == nil {
		t.Fatalf("an error answer was accepted")
	}
}

// TestValidRefusesAFontThatForbidsEmbedding: the fsType bit is the licence gate.
// The bytes are a bundled font with that bit set - same program, one flag.
func TestValidRefusesAFontThatForbidsEmbedding(t *testing.T) {
	ttf := append([]byte(nil), Book()[0].Faces[0].TTF...)
	off := os2Offset(ttf)
	if off < 0 {
		t.Fatalf("no OS/2 table in the bundled font")
	}
	ttf[off+8], ttf[off+9] = 0x00, 0x02 // fsType 2: restricted, no embedding
	if fs, _ := FS(ttf); fs&0x0002 == 0 {
		t.Fatalf("the test did not set the bit, fsType is %d", fs)
	}
	if valid(ttf) {
		t.Errorf("valid() took a font whose licence forbids embedding")
	}
}

// os2Offset finds the OS/2 table in an sfnt file, or -1.
func os2Offset(v []byte) int {
	if len(v) < 12 {
		return -1
	}
	n := int(v[4])<<8 | int(v[5])
	for i := range n {
		rec := 12 + i*16
		if rec+16 > len(v) {
			return -1
		}
		if string(v[rec:rec+4]) == "OS/2" {
			return int(v[rec+8])<<24 | int(v[rec+9])<<16 | int(v[rec+10])<<8 | int(v[rec+11])
		}
	}
	return -1
}
