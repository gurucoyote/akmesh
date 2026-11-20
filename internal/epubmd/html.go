package epubmd

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"path"
	"strings"
)

// HTML returns the book as one HTML document.
//
// It is the counterpart of Markdown here: the chapters of an EPUB already are
// XHTML, so reading a book as HTML keeps what a trip through Markdown may
// flatten - a table with a head row, a nested list, inline code. The chapter
// bodies are joined in spine order, the way Convert joins their Markdown.
// Image is one picture taken out of the book.
type Image struct {
	Name string
	Data []byte
}

func htmlOf(data []byte, imagesDir string) (string, []Image, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, fmt.Errorf("not a zip archive: %w", err)
	}
	files := map[string]*zip.File{}
	for _, f := range r.File {
		files[f.Name] = f
	}
	rootfile, err := rootFilePath(files)
	if err != nil {
		return "", nil, err
	}
	opf, err := readFile(files, rootfile)
	if err != nil {
		return "", nil, err
	}
	var pkg packageDoc
	if err := xml.Unmarshal(opf, &pkg); err != nil {
		return "", nil, fmt.Errorf("package document %s: %w", rootfile, err)
	}
	base := path.Dir(rootfile)
	byID := map[string]item{}
	for _, it := range pkg.Manifest.Items {
		byID[it.ID] = it
	}

	title := pkg.Metadata.Title
	if title == "" {
		title = "Ohne Titel"
	}
	lang := pkg.Metadata.Language
	if lang == "" {
		lang = "de"
	}

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"" + escAttr(lang) + "\">\n<head>\n<meta charset=\"UTF-8\"/>\n<title>" +
		escText(title) + "</title>\n</head>\n<body>\n")
	var images []Image
	written := 0
	for _, ref := range pkg.Spine.ItemRefs {
		it, ok := byID[ref.IDRef]
		if !ok || strings.Contains(it.Properties, "nav") {
			continue
		}
		// The licence page was written by the converter, not by the author;
		// carrying it over would put it into the next book twice.
		if path.Base(it.Href) == "colophon.xhtml" {
			continue
		}
		body, err := readFile(files, path.Join(base, it.Href))
		if err != nil {
			return "", nil, err
		}
		inner := strings.TrimSpace(bodyOf(string(body)))
		if inner == "" {
			continue
		}
		inner, imgs := liftPictures(files, base, it.Href, inner, imagesDir, images)
		images = imgs
		b.WriteString(inner)
		b.WriteString("\n")
		written++
	}
	if written == 0 {
		return "", nil, fmt.Errorf("no readable chapter in the spine")
	}
	b.WriteString("</body>\n</html>\n")
	return b.String(), images, nil
}

// escText escapes text for HTML content.
func escText(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// escAttr escapes text for an HTML attribute.
func escAttr(s string) string {
	return strings.ReplaceAll(escText(s), `"`, "&quot;")
}

// HTML returns the book as one HTML page. A picture cannot travel inside a web
// page, so its bytes are handed back; without a folder to write them into, the
// description stands in its place.
func HTML(data []byte) (string, []Image, error) {
	return htmlOf(data, "")
}

// HTMLInto is HTML with a folder for the pictures: the page points at
// images/<name> and the bytes come back to be written there.
func HTMLInto(data []byte, imagesDir string) (string, []Image, error) {
	return htmlOf(data, imagesDir)
}

// liftPictures copies the pictures a chapter refers to into the package's own
// naming and rewrites the reference. Without a folder to write them into, the
// description stands in for the picture, as in docx2md: a reference to a file
// nobody wrote is a dead link.
func liftPictures(files map[string]*zip.File, base, chapterHref, body, imagesDir string, images []Image) (string, []Image) {
	var out strings.Builder
	rest := body
	for {
		i := strings.Index(rest, "<img")
		if i < 0 {
			break
		}
		end := strings.Index(rest[i:], ">")
		if end < 0 {
			break
		}
		end += i
		tag := rest[i : end+1]
		out.WriteString(rest[:i])
		rest = rest[end+1:]

		src := attrValue(tag, "src")
		alt := attrValue(tag, "alt")
		if alt == "" {
			alt = "Bild"
		}
		if src == "" || strings.HasPrefix(src, "http") {
			out.WriteString("[Bild]")
			continue
		}
		name := path.Base(src)
		data, err := readFile(files, path.Join(base, path.Dir(chapterHref), src))
		if err != nil {
			out.WriteString("[Bild]")
			continue
		}
		if imagesDir == "" {
			out.WriteString("[" + escText(alt) + "]")
			continue
		}
		images = append(images, Image{Name: name, Data: data})
		out.WriteString(`<img src="images/` + escAttr(name) + `" alt="` + escAttr(alt) + `"/>`)
	}
	out.WriteString(rest)
	return out.String(), images
}

// attrValue reads one attribute out of a tag.
func attrValue(tag, name string) string {
	i := strings.Index(tag, name+"=\"")
	if i < 0 {
		return ""
	}
	start := i + len(name) + 2
	end := strings.Index(tag[start:], "\"")
	if end < 0 {
		return ""
	}
	return tag[start : start+end]
}
