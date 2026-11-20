package epubgen

import (
	"fmt"
	"regexp"
	"strings"

	"akmesh/internal/imgload"
	"akmesh/internal/webfonts"
)

// Pictures in a book.
//
// An EPUB reader that shows pictures wants the file inside the package; a
// reference to the author's disk is not a picture, it is a broken link. What is
// carried is decided in internal/imgload - the same loader md2docx uses, so the
// document and the book agree about which pictures exist.

var imgTag = regexp.MustCompile(`<img\b[^>]*>`)

var (
	attrSrc = regexp.MustCompile(`\bsrc="([^"]*)"`)
	attrAlt = regexp.MustCompile(`\balt="([^"]*)"`)
)

// embedImages copies every referenced picture into the package and rewrites the
// reference to point at that copy. A picture that cannot be read is replaced by
// its description, which is what a reader would show anyway.
func embedImages(body, baseDir string) (string, []imagePart) {
	var images []imagePart
	out := imgTag.ReplaceAllStringFunc(body, func(tag string) string {
		alt := ""
		if m := attrAlt.FindStringSubmatch(tag); m != nil {
			alt = m[1]
		}
		var src string
		if m := attrSrc.FindStringSubmatch(tag); m != nil {
			src = m[1]
		}
		img, ok := imgload.Load(htmlUnescape(src), baseDir)
		if !ok {
			return "[" + alt + "]"
		}
		name := "images/bild" + fmt.Sprint(len(images)+1) + "." + img.Format
		images = append(images, imagePart{name: name, data: img.Data})
		return `<img src="` + name + `" alt="` + alt + `"/>`
	})
	return out, images
}

// htmlUnescape is the small part of XML unescaping an attribute can carry.
func htmlUnescape(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">")
	return r.Replace(s)
}

// fontName is the file name a face travels under: family and style, lower case,
// spaces to hyphens.
func fontName(family string, style webfonts.Style) string {
	return "fonts/" + slug(family) + "-" + slug(string(style)) + ".ttf"
}

// slug turns a name into something a ZIP entry and a URL can carry.
func slug(s string) string {
	var b strings.Builder
	last := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			last = false
		case !last:
			b.WriteByte('-')
			last = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// styleCSS carries the fonts into the book: the @font-face rules name the files
// travelling next to it, so a reader that honours them sets the book in the
// faces it was made in, and one that does not falls back to serif and
// monospace rather than to nothing.
func styleCSS(families []webfonts.Family) string {
	var b strings.Builder
	b.WriteString("@charset \"UTF-8\";\n")
	for _, fam := range families {
		for _, face := range fam.Faces {
			b.WriteString("@font-face { font-family: \"" + fam.Name +
				"\"; font-style: " + italicOrNormal(face.Style) +
				"; font-weight: " + weightOf(face.Style) +
				"; src: url(\"" + fontName(fam.Name, face.Style) + "\"); }\n")
		}
	}
	body := `"Lora", serif`
	code := `"Go Mono", monospace`
	if len(families) > 0 {
		body = `"` + families[0].Name + `", serif`
	}
	if len(families) > 1 {
		code = `"` + families[1].Name + `", monospace`
	}
	b.WriteString(`
body { font-family: ` + body + `; line-height: 1.5; margin: 1em; }
h1, h2, h3 { line-height: 1.2; }
table { border-collapse: collapse; }
th, td { border: 1px solid #888; padding: 0.2em 0.4em; }
pre { background: #f4f4f4; padding: 0.6em; overflow-x: auto; }
code, kbd, samp { font-family: ` + code + `; }
blockquote { margin-left: 1em; font-style: italic; }
img { max-width: 100%; height: auto; }
`)
	return b.String()
}

func italicOrNormal(st webfonts.Style) string {
	if st == webfonts.Italic || st == webfonts.BoldItalic {
		return "italic"
	}
	return "normal"
}

func weightOf(st webfonts.Style) string {
	if st == webfonts.Bold || st == webfonts.BoldItalic {
		return "700"
	}
	return "400"
}

// colophonXHTML is the page that says under which terms the fonts travel. A
// licence the reader cannot find is no licence notice, and the OFL asks for the
// terms to go with the program.
func colophonXHTML(opts Options, families []webfonts.Family) string {
	body := "<h1>Schriften und Lizenzen</h1>" +
		"<p>Dieses Buch bringt seine Schriften mit, damit es überall gleich aussieht.</p>" +
		"<pre>" + esc(webfonts.NoticeFile(families)) + "</pre>"
	return xhtmlPage(opts.Language, "Schriften und Lizenzen", body)
}
