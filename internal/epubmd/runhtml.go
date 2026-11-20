package epubmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const usageHTML = `usage: epub2html [-o FILE.html] [FILE...]

Prints the HTML of each EPUB book. Without a file the book is read from standard
input.

A chapter of a book is XHTML already, so this way keeps what Markdown can
flatten: a table with a head row, a nested list, inline code. The chapters are
joined in the order the package declares them, and the navigation document -
the table of contents - is left out, because it is not a chapter.

The result is a web page, not a book: the fonts and the licence page stay
behind. Without -o the pictures cannot be carried along - there is no folder to
put them in - so their description stands in for them; with -o the page points at
images/<name> and the files are written beside it.
`

// RunHTML is the command line of epub2html.
func RunHTML(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	out := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "epub2html: %s needs a file name\n%s", a, usageHTML)
				return 2
			}
			i++
			out = args[i]
		case a == "-h" || a == "--help":
			fmt.Fprint(stderr, usageHTML)
			return 0
		case len(a) > 0 && a[0] == '-' && a != "-":
			fmt.Fprintf(stderr, "epub2html: unknown option %s\n%s", a, usageHTML)
			return 2
		case a == "-":
			files = append(files, "")
		default:
			files = append(files, a)
		}
	}

	w := bufio.NewWriter(stdout)
	defer w.Flush()
	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "epub2html: %v\n", err)
			return 1
		}
		return writeHTML(w, data, out, stderr)
	}
	for _, name := range files {
		var data []byte
		var err error
		if name == "" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(name)
		}
		if err != nil {
			fmt.Fprintf(stderr, "epub2html: %v\n", err)
			return 1
		}
		if code := writeHTML(w, data, out, stderr); code != 0 {
			return code
		}
	}
	return 0
}

// writeHTML converts one book and puts its HTML on the writer.
func writeHTML(w *bufio.Writer, data []byte, out string, stderr *os.File) int {
	var page string
	var images []Image
	var err error
	if out == "" {
		page, images, err = HTML(data)
	} else {
		page, images, err = HTMLInto(data, filepath.Dir(out))
	}
	if err != nil {
		fmt.Fprintf(stderr, "epub2html: %v\n", err)
		return 1
	}
	if out != "" {
		if err := os.WriteFile(out, []byte(page), 0o644); err != nil {
			fmt.Fprintf(stderr, "epub2html: %v\n", err)
			return 1
		}
	} else if _, err := w.WriteString(page); err != nil {
		fmt.Fprintf(stderr, "epub2html: %v\n", err)
		return 1
	}
	for _, img := range images {
		if out == "" {
			break
		}
		p := filepath.Join(filepath.Dir(out), "images", img.Name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			fmt.Fprintf(stderr, "epub2html: %v\n", err)
			return 1
		}
		if err := os.WriteFile(p, img.Data, 0o644); err != nil {
			fmt.Fprintf(stderr, "epub2html: %v\n", err)
			return 1
		}
	}
	return 0
}
