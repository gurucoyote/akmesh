package docxgen

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"akmesh/internal/htmlmd"
)

const usageHTML = `usage: html2docx [-o FILE.docx] [FILE...]

Writes a Word document from HTML. Without a file the HTML is read from standard
input.

The way there is HTML -> Markdown -> document, and that is on purpose: the
Markdown reader is the one this tool already trusts, and a second document
builder would drift from it. What survives is what Markdown can say - headings,
paragraphs, bold and italic, inline and block code, links, pictures, lists with
their nesting, and a table with its head row (the head row becomes a Word head
row).

What does not survive, and no route through Markdown can keep: column widths,
merged cells, colours, fonts and text boxes. If a document needs those, the docx
has to be built from the original, not from a converted page.

Pictures are resolved against the folder of the first input file, and their
bytes are embedded - a docx that points at a file next to it is a broken
document as soon as it is mailed.
`

// pageBody drops the head of a whole HTML page. Its title is not a paragraph of
// the document - it is the name of the page, and it walked in as the first line
// before a document reached the reader with the title twice.
func pageBody(doc string) string {
	lower := strings.ToLower(doc)
	if start := strings.Index(lower, "<head"); start >= 0 {
		if end := strings.Index(lower[start:], "</head>"); end >= 0 {
			doc = doc[:start] + doc[start+end+len("</head>"):]
			lower = strings.ToLower(doc)
		}
	}
	if start := strings.Index(lower, "<body"); start >= 0 {
		if gt := strings.Index(doc[start:], ">"); gt >= 0 {
			start += gt + 1
			if end := strings.LastIndex(lower, "</body>"); end > start {
				return doc[start:end]
			}
			return doc[start:]
		}
	}
	return doc
}

// convertPageForTest is the step html2docx takes before the writer, kept in one
// place so the test and the command cannot drift apart.
func convertPageForTest(page string) string {
	return htmlmd.ConvertGFM(pageBody(page))
}

// RunHTML is the command line of html2docx.
func RunHTML(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	out := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stderr, usageHTML)
			return 0
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "html2docx: %s needs a file name\n%s", a, usageHTML)
				return 2
			}
			i++
			out = args[i]
		case len(a) > 0 && a[0] == '-' && a != "-":
			fmt.Fprintf(stderr, "html2docx: unknown option %s\n%s", a, usageHTML)
			return 2
		case a == "-":
			files = append(files, "")
		default:
			files = append(files, a)
		}
	}

	baseDir := ""
	var source []byte
	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "html2docx: %v\n", err)
			return 1
		}
		source = data
		if wd, err := os.Getwd(); err == nil {
			baseDir = wd
		}
	} else {
		for i, name := range files {
			var data []byte
			var err error
			if name == "" {
				data, err = io.ReadAll(stdin)
			} else {
				data, err = os.ReadFile(name)
			}
			if err != nil {
				fmt.Fprintf(stderr, "html2docx: %v\n", err)
				return 1
			}
			if i == 0 && name != "" {
				if abs, err := filepath.Abs(name); err == nil {
					baseDir = filepath.Dir(abs)
				}
			}
			source = append(source, data...)
			source = append(source, '\n')
		}
	}
	if baseDir == "" {
		if wd, err := os.Getwd(); err == nil {
			baseDir = wd
		}
	}

	markdown := convertPageForTest(string(source))
	// A page whose markup produced no text is not a document; refusing is
	// kinder than writing an empty file.
	if strings.TrimSpace(markdown) == "" {
		fmt.Fprintf(stderr, "html2docx: no text in the input\n")
		return 1
	}

	if out == "" {
		w := bufio.NewWriter(stdout)
		if err := ConvertWithBase(w, []byte(markdown), baseDir); err != nil {
			fmt.Fprintf(stderr, "html2docx: %v\n", err)
			return 1
		}
		if err := w.Flush(); err != nil {
			fmt.Fprintf(stderr, "html2docx: %v\n", err)
			return 1
		}
		return 0
	}
	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(stderr, "html2docx: %v\n", err)
		return 1
	}
	w := bufio.NewWriter(f)
	if err := ConvertWithBase(w, []byte(markdown), baseDir); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "html2docx: %v\n", err)
		return 1
	}
	if err := w.Flush(); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "html2docx: %v\n", err)
		return 1
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(stderr, "html2docx: %v\n", err)
		return 1
	}
	return 0
}
