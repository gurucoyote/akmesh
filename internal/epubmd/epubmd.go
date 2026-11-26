// Package epubmd reads an EPUB into Markdown.
//
// An EPUB is a ZIP whose parts point at each other: META-INF/container.xml names
// the package document, the package document lists the chapters in its manifest
// and orders them in its spine. The spine is the book; a reader that sorts file
// names instead gets the chapters in whatever order the author's tool happened to
// save them.
//
// The chapters themselves are XHTML. They are handed to the same converter that
// reads an HTML page, so an EPUB chapter and its HTML source produce the same
// Markdown - one piece of code, one set of rules about links, lists and tables.
package epubmd

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/gurucoyote/akmesh/internal/htmlmd"
)

// Convert returns the Markdown of an EPUB book. Chapters are joined by a blank
// line, in the order the package declares them.
func Convert(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("not a zip archive: %w", err)
	}
	files := map[string]*zip.File{}
	for _, f := range r.File {
		files[f.Name] = f
	}

	rootfile, err := rootFilePath(files)
	if err != nil {
		return "", err
	}
	opf, err := readFile(files, rootfile)
	if err != nil {
		return "", err
	}
	var pkg packageDoc
	if err := xml.Unmarshal(opf, &pkg); err != nil {
		return "", fmt.Errorf("package document %s: %w", rootfile, err)
	}

	base := path.Dir(rootfile)
	// id -> item, so the spine can name chapters by id.
	byID := map[string]item{}
	for _, it := range pkg.Manifest.Items {
		byID[it.ID] = it
	}

	var chapters []string
	for _, ref := range pkg.Spine.ItemRefs {
		it, ok := byID[ref.IDRef]
		if !ok {
			continue
		}
		// The nav document is the table of contents, not a chapter; printing it
		// puts the contents into the book twice.
		if strings.Contains(it.Properties, "nav") {
			continue
		}
		// The licence page this tool writes itself (internal/epubgen) is not
		// the author's text. Read back, it must not become a chapter, or a book
		// made from a book carries the licence twice.
		if path.Base(it.Href) == "colophon.xhtml" {
			continue
		}
		body, err := readFile(files, path.Join(base, it.Href))
		if err != nil {
			// A missing chapter must not silently shorten the book.
			return "", err
		}
		md := strings.TrimSpace(normalizeTables(htmlmd.Convert(collapseTableWhitespace(bodyOf(string(body))))))
		if md == "" {
			continue
		}
		chapters = append(chapters, md)
	}
	if len(chapters) == 0 {
		return "", errors.New("no readable chapter in the spine")
	}
	return strings.Join(chapters, "\n\n") + "\n", nil
}

// bodyOf returns what stands between <body> and </body>.
//
// A chapter file is a whole XHTML document: XML declaration, doctype, head with a
// title and the stylesheet. Only the body is text. Handing the file to the HTML
// converter unchanged puts the declaration, the title and every CSS rule into the
// book - readable-looking output and wrong.
func bodyOf(xhtml string) string {
	start := strings.Index(strings.ToLower(xhtml), "<body")
	if start < 0 {
		// A fragment without <body> is already what we want.
		return xhtml
	}
	open := strings.Index(xhtml[start:], ">")
	if open < 0 {
		return xhtml
	}
	rest := xhtml[start+open+1:]
	if end := strings.Index(strings.ToLower(rest), "</body>"); end >= 0 {
		return rest[:end]
	}
	return rest
}

// collapseTableWhitespace removes the source whitespace between cells.
//
// goldmark writes every cell of a table on a line of its own. The HTML converter
// keeps that whitespace, and a table row then arrives as "Name \n  | Ort" - one
// cell per line with a hanging pipe. Inside a table the whitespace is not text,
// so it is removed before the conversion; everywhere else it stays, because a
// source line break between two words is a space a reader may want.
func collapseTableWhitespace(s string) string {
	lower := strings.ToLower(s)
	var b strings.Builder
	for {
		start := strings.Index(lower, "<table")
		if start < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := strings.Index(lower[start:], "</table>")
		if end < 0 {
			b.WriteString(s)
			return b.String()
		}
		end += start + len("</table>")
		b.WriteString(s[:start])
		b.WriteString(collapseBetweenTags(s[start:end]))
		s = s[end:]
		lower = strings.ToLower(s)
	}
}

// collapseBetweenTags turns ">  \n <" into "><".
func collapseBetweenTags(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '>' {
			b.WriteByte(c)
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j < len(s) && s[j] == '<' {
				i = j - 1
				continue
			}
			// Whitespace before text stays as it is.
			b.WriteString(s[i+1 : j])
			i = j - 1
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// normalizeTables turns the pipe-separated table lines of the HTML converter
// into GFM tables.
//
// html2md has its own contract: it writes a table as cells separated by pipes
// and deliberately does not claim to be a GFM table (see its test), because a
// terminal reader only wants the cells. A book is different - its Markdown is
// read on, and a table without the dash line parses as a paragraph. So the
// shape is restored here, where the requirement exists, instead of changing the
// meaning of html2md for everyone.
func normalizeTables(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.Contains(line, " | ") || strings.HasPrefix(strings.TrimSpace(line), "- ") {
			out = append(out, line)
			continue
		}
		// A table needs at least two rows with the same number of cells; a
		// single line with a pipe is prose.
		cells := strings.Count(line, " | ") + 1
		j := i
		var rows []string
		for j < len(lines) && strings.Count(lines[j], " | ")+1 == cells && strings.Contains(lines[j], " | ") {
			rows = append(rows, lines[j])
			j++
		}
		if len(rows) < 2 {
			out = append(out, line)
			continue
		}
		// A table needs a blank line in front of it, or the line above swallows
		// the first row as a paragraph continuation.
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		for k, r := range rows {
			out = append(out, "| "+strings.TrimSpace(r)+" |")
			if k == 0 {
				out = append(out, "|"+strings.Repeat(" --- |", cells))
			}
		}
		i = j - 1
	}
	return strings.Join(out, "\n")
}

// rootFilePath follows META-INF/container.xml to the package document.
func rootFilePath(files map[string]*zip.File) (string, error) {
	body, err := readFile(files, "META-INF/container.xml")
	if err != nil {
		return "", errors.New("no META-INF/container.xml: this zip is not an EPUB")
	}
	var c container
	if err := xml.Unmarshal(body, &c); err != nil {
		return "", fmt.Errorf("container.xml: %w", err)
	}
	for _, rf := range c.Rootfiles {
		if rf.FullPath != "" {
			return rf.FullPath, nil
		}
	}
	return "", errors.New("container.xml names no package document")
}

// readFile returns the content of one part.
func readFile(files map[string]*zip.File, name string) ([]byte, error) {
	f, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("part %s is missing", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

type container struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type item struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type packageDoc struct {
	Metadata struct {
		Title    string `xml:"title"`
		Language string `xml:"language"`
	} `xml:"metadata"`
	Manifest struct {
		Items []item `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		ItemRefs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}
