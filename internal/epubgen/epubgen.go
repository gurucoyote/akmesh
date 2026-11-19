// Package epubgen writes an EPUB 3 package from Markdown.
//
// The pipeline is goldmark and nothing else: the Markdown is rendered to HTML
// once, the HTML is cut into chapters at its top-level headings, and the parts of
// an EPUB are written around them. No library for the packaging, because an EPUB
// is a ZIP with a fixed first entry and a handful of XML files - and because a
// reader that opens the result (internal/epubmd) has to understand the same
// layout, so one file of packaging code serves both.
//
// The rules that make a file an EPUB instead of a zip:
//
//   - "mimetype" is the first entry and is stored, never deflated; a reader
//     checks it before it reads anything else and refuses the book otherwise;
//   - META-INF/container.xml names the package document;
//   - the package document lists every chapter in the spine, and the spine is the
//     reading order - file names mean nothing;
//   - the nav document is declared with properties="nav" and is not a chapter.
package epubgen

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// Options carries what a book needs beyond its text.
type Options struct {
	// Title is the book title; "Ohne Titel" is used when it is empty, because
	// an EPUB without a title is rejected by some readers.
	Title string
	// Author is optional and stays out of the metadata when empty.
	Author string
	// Language is the book language; de is the default.
	Language string
}

// chapter is one section of the book.
type chapter struct {
	title string
	body  string
}

// Convert writes the EPUB of a Markdown document.
func Convert(w io.Writer, markdown []byte, opts Options) error {
	if opts.Title == "" {
		opts.Title = "Ohne Titel"
	}
	if opts.Language == "" {
		opts.Language = "de"
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Strikethrough),
		goldmark.WithRendererOptions(html.WithXHTML(), html.WithUnsafe()),
	)
	var rendered bytes.Buffer
	if err := md.Convert(markdown, &rendered); err != nil {
		return err
	}

	chapters := splitChapters(rendered.String())

	zw := zip.NewWriter(w)
	// The mimetype entry has to come first and uncompressed.
	mime, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		return err
	}
	if _, err := io.WriteString(mime, "application/epub+zip"); err != nil {
		return err
	}

	parts := []struct{ name, body string }{
		{"META-INF/container.xml", containerXML},
		{"OEBPS/package.opf", packageOPF(opts, chapters)},
		{"OEBPS/nav.xhtml", navXHTML(opts, chapters)},
	}
	for i, c := range chapters {
		parts = append(parts, struct{ name, body string }{
			fmt.Sprintf("OEBPS/chapter%d.xhtml", i+1),
			chapterXHTML(opts, c),
		})
	}
	for _, p := range parts {
		f, err := zw.Create(p.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(f, p.body); err != nil {
			return err
		}
	}
	return zw.Close()
}

// splitChapters cuts the rendered HTML at its top-level headings. A document
// without a level-1 heading is one chapter, which is the honest reading of a
// text that never says where it divides.
func splitChapters(htmlText string) []chapter {
	starts := h1Starts(htmlText)
	if len(starts) == 0 {
		return []chapter{{title: "", body: ensureXHTML(htmlText)}}
	}
	var out []chapter
	// Text before the first heading belongs to the first chapter.
	if pre := strings.TrimSpace(htmlText[:starts[0]]); pre != "" {
		out = append(out, chapter{title: "", body: ensureXHTML(pre)})
	}
	for i, at := range starts {
		end := len(htmlText)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		part := htmlText[at:end]
		out = append(out, chapter{title: headingText(part), body: ensureXHTML(part)})
	}
	return out
}

// h1Starts returns the offsets of the top-level headings.
func h1Starts(s string) []int {
	var out []int
	for off := 0; ; {
		i := strings.Index(s[off:], "<h1")
		if i < 0 {
			return out
		}
		i += off
		// A heading must stand at the start of a line; anything else is text
		// inside a code block or an attribute.
		if i == 0 || s[i-1] == '\n' {
			out = append(out, i)
		}
		off = i + 3
	}
}

// headingText returns the text of the first <h1> in a fragment.
func headingText(fragment string) string {
	open := strings.Index(fragment, ">")
	if open < 0 {
		return ""
	}
	end := strings.Index(fragment[open:], "</h1>")
	if end < 0 {
		return ""
	}
	text := fragment[open+1 : open+end]
	text = stripTags(text)
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	text = strings.ReplaceAll(text, "&#39;", "'")
	return strings.TrimSpace(text)
}

// stripTags removes inline tags, keeping their text.
func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ensureXHTML closes the void elements. goldmark writes HTML5 (<br>, <hr>, <img
// ...>) and an XHTML reader stops at the first unclosed tag.
func ensureXHTML(s string) string {
	for _, tag := range []string{"br", "hr", "img", "col", "input", "meta", "link"} {
		s = closeVoid(s, tag)
	}
	return s
}

// closeVoid turns <tag> and <tag ...> into self-closing form, leaving the
// already-closed ones alone.
func closeVoid(s, tag string) string {
	var b strings.Builder
	rest := s
	for {
		i := strings.Index(rest, "<"+tag)
		if i < 0 {
			break
		}
		after := i + 1 + len(tag)
		if after >= len(rest) {
			break
		}
		c := rest[after]
		if c != '>' && c != ' ' && c != '\t' && c != '\n' {
			b.WriteString(rest[:after])
			rest = rest[after:]
			continue
		}
		end := strings.Index(rest[after:], ">")
		if end < 0 {
			break
		}
		end += after
		b.WriteString(rest[:end])
		if end > 0 && rest[end-1] != '/' {
			b.WriteString("/")
		}
		b.WriteString(">")
		rest = rest[end+1:]
	}
	b.WriteString(rest)
	return b.String()
}

// xhtmlPage wraps a chapter body into a complete XHTML document.
func xhtmlPage(lang, title, body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<!DOCTYPE html>` + "\n" +
		`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + esc(lang) + `" lang="` + esc(lang) + `">` + "\n" +
		`<head><meta charset="UTF-8"/><title>` + esc(title) + `</title>` + chapterCSS + `</head>` + "\n" +
		`<body>` + "\n" + body + "\n" + `</body></html>`
}

// chapterCSS keeps the book readable without depending on a reader's defaults.
const chapterCSS = `<style>
body { font-family: serif; line-height: 1.5; margin: 1em; }
h1, h2, h3 { line-height: 1.2; }
table { border-collapse: collapse; }
th, td { border: 1px solid #888; padding: 0.2em 0.4em; }
pre { background: #f4f4f4; padding: 0.6em; overflow-x: auto; }
code { font-family: monospace; }
blockquote { margin-left: 1em; font-style: italic; }
</style>`

// chapterXHTML is one chapter file.
func chapterXHTML(opts Options, c chapter) string {
	return xhtmlPage(opts.Language, c.title, c.body)
}

// navXHTML is the table of contents, declared as the book's nav document.
func navXHTML(opts Options, chapters []chapter) string {
	var b strings.Builder
	b.WriteString(`<nav epub:type="toc" id="toc"><h1>` + esc(opts.Title) + `</h1><ol>`)
	for i, c := range chapters {
		title := c.title
		if title == "" {
			title = fmt.Sprintf("Kapitel %d", i+1)
		}
		b.WriteString(`<li><a href="chapter` + fmt.Sprint(i+1) + `.xhtml">` + esc(title) + `</a></li>`)
	}
	b.WriteString(`</ol></nav>`)
	return xhtmlPage(opts.Language, opts.Title, b.String())
}

// packageOPF is the package document: metadata, manifest, spine.
func packageOPF(opts Options, chapters []chapter) string {
	var manifest, spine strings.Builder
	manifest.WriteString(`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`)
	for i := range chapters {
		manifest.WriteString(`<item id="c` + fmt.Sprint(i+1) + `" href="chapter` +
			fmt.Sprint(i+1) + `.xhtml" media-type="application/xhtml+xml"/>`)
		spine.WriteString(`<itemref idref="c` + fmt.Sprint(i+1) + `"/>`)
	}
	author := ""
	if opts.Author != "" {
		author = `<dc:creator>` + esc(opts.Author) + `</dc:creator>`
	}
	return `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid">` + "\n" +
		`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
		`<dc:identifier id="bookid">urn:uuid:acmesh-` + bookID(opts.Title) + `</dc:identifier>` +
		`<dc:title>` + esc(opts.Title) + `</dc:title>` +
		`<dc:language>` + esc(opts.Language) + `</dc:language>` + author +
		`<meta property="dcterms:modified">` + modified + `</meta>` +
		`</metadata>` + "\n" +
		`<manifest>` + manifest.String() + `</manifest>` + "\n" +
		`<spine>` + spine.String() + `</spine>` + "\n" +
		`</package>`
}

// modified is a fixed timestamp. A changing value would make two builds of the
// same book differ, and a diff would then report a change that never happened.
const modified = "2026-01-01T00:00:00Z"

// bookID derives a stable identifier from the title, so that two books with
// different titles are not the same book to a reader's library.
func bookID(title string) string {
	var sum uint32 = 2166136261
	for _, r := range title {
		sum ^= uint32(r)
		sum *= 16777619
	}
	return fmt.Sprintf("%08x", sum)
}

const containerXML = `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
	`<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">` + "\n" +
	`<rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles>` + "\n" +
	`</container>`

// esc escapes text for XML content and attributes.
func esc(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&#39;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
