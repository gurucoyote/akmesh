package epubgen

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const usageHTML = `usage: html2epub [-o FILE] [--title TITLE] [--author NAME] [FILE...]

Writes an EPUB 3 book from HTML. Without a file the HTML is read from standard
input.

HTML is what a book is made of - the chapters of an EPUB are XHTML - so this way
keeps the structure that Markdown can flatten: a table with a head row, a nested
list, inline code. Sections are cut at the top-level headings the way md2epub
does it, and the book carries its fonts, its pictures and the font licence like
any other book from this tool.

--title falls back to the file name, as in md2epub. Pictures are resolved
against the folder of the first input file.
`

// RunHTML is the command line of html2epub.
func RunHTML(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	out, title, author := "", "", ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stderr, usageHTML)
			return 0
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "html2epub: %s needs a file name\n%s", a, usageHTML)
				return 2
			}
			i++
			out = args[i]
		case a == "--title":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "html2epub: %s needs a value\n", a)
				return 2
			}
			i++
			title = args[i]
		case strings.HasPrefix(a, "--title="):
			title = strings.TrimPrefix(a, "--title=")
		case a == "--author":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "html2epub: %s needs a value\n", a)
				return 2
			}
			i++
			author = args[i]
		case strings.HasPrefix(a, "--author="):
			author = strings.TrimPrefix(a, "--author=")
		case len(a) > 0 && a[0] == '-':
			fmt.Fprintf(stderr, "html2epub: unknown option %s\n%s", a, usageHTML)
			return 2
		default:
			files = append(files, a)
		}
	}

	var source []byte
	opts := Options{Title: title, Author: author}
	if wd, err := os.Getwd(); err == nil {
		opts.BaseDir = wd
	}
	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "html2epub: %v\n", err)
			return 1
		}
		source = data
	} else {
		for i, name := range files {
			data, err := os.ReadFile(name)
			if err != nil {
				fmt.Fprintf(stderr, "html2epub: %v\n", err)
				return 1
			}
			if i == 0 {
				if abs, err := filepath.Abs(name); err == nil {
					opts.BaseDir = filepath.Dir(abs)
				}
				if title == "" {
					base := filepath.Base(name)
					opts.Title = strings.TrimSuffix(base, filepath.Ext(base))
				}
			}
			source = append(source, data...)
			source = append(source, '\n')
		}
	}

	if out == "" {
		w := bufio.NewWriter(stdout)
		if err := ConvertHTML(w, source, opts); err != nil {
			fmt.Fprintf(stderr, "html2epub: %v\n", err)
			return 1
		}
		if err := w.Flush(); err != nil {
			fmt.Fprintf(stderr, "html2epub: %v\n", err)
			return 1
		}
		return 0
	}
	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(stderr, "html2epub: %v\n", err)
		return 1
	}
	w := bufio.NewWriter(f)
	if err := ConvertHTML(w, source, opts); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "html2epub: %v\n", err)
		return 1
	}
	if err := w.Flush(); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "html2epub: %v\n", err)
		return 1
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(stderr, "html2epub: %v\n", err)
		return 1
	}
	return 0
}
