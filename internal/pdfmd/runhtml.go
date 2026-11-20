package pdfmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const usageHTML = `usage: pdf2html [FILE...]

Prints the HTML of each PDF. Without a file the document is read from standard
input.

A tagged PDF carries a structure tree, and that tree is followed: a heading
becomes a heading, a list a list, a table a table with its head row. A PDF
without tags has no such knowledge, so its text is guessed as in pdf2md and the
guessed Markdown is rendered to HTML - one rule set, one result.

Pictures: the tree names them, but their bytes are not part of what pdfmd reads,
so a figure stands with its description and no image data.
`

// RunHTML is the command line of pdf2html.
func RunHTML(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stderr, usageHTML)
			return 0
		case len(a) > 0 && a[0] == '-' && a != "-":
			fmt.Fprintf(stderr, "pdf2html: unknown option %s\n%s", a, usageHTML)
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
			fmt.Fprintf(stderr, "pdf2html: %v\n", err)
			return 1
		}
		return writeHTML(w, data, stderr)
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
			fmt.Fprintf(stderr, "pdf2html: %v\n", err)
			return 1
		}
		if code := writeHTML(w, data, stderr); code != 0 {
			return code
		}
	}
	return 0
}

// writeHTML converts one document and puts its HTML on the writer.
func writeHTML(w *bufio.Writer, data []byte, stderr *os.File) int {
	page, err := HTML(data)
	if err != nil {
		fmt.Fprintf(stderr, "pdf2html: %v\n", err)
		return 1
	}
	if _, err := w.WriteString(page); err != nil {
		fmt.Fprintf(stderr, "pdf2html: %v\n", err)
		return 1
	}
	return 0
}
