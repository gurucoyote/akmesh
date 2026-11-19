package pdfgen

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const usage = `usage: md2pdf [-o FILE] [FILE...]

Renders Markdown as a PDF. Without FILE the Markdown is read from standard
input; without -o the PDF goes to standard output, which is only useful when it
is redirected into a file.

The renderer is a slimmed goldmark-pdf: headings, paragraphs, emphasis, lists,
blockquotes, tables, links and images are rendered; a code block is set as a
monospaced box without syntax colours.
`

// Run is the command line of md2pdf.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	out := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			fmt.Fprint(stderr, usage)
			return 0
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "md2pdf: %s needs a file name\n%s", a, usage)
				return 2
			}
			i++
			out = args[i]
		case len(a) > 1 && a[0] == '-' && a[1] != '-':
			fmt.Fprintf(stderr, "md2pdf: unknown option %s\n%s", a, usage)
			return 2
		case len(a) > 0 && a[0] == '-':
			fmt.Fprintf(stderr, "md2pdf: unknown option %s\n%s", a, usage)
			return 2
		default:
			files = append(files, a)
		}
	}

	var source []byte
	var err error
	if len(files) == 0 {
		source, err = io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "md2pdf: %v\n", err)
			return 1
		}
	} else {
		for _, name := range files {
			data, err := os.ReadFile(name)
			if err != nil {
				fmt.Fprintf(stderr, "md2pdf: %v\n", err)
				return 1
			}
			source = append(source, data...)
			source = append(source, '\n')
		}
	}

	if out == "" {
		w := bufio.NewWriter(stdout)
		if err := Convert(w, source); err != nil {
			fmt.Fprintf(stderr, "md2pdf: %v\n", err)
			return 1
		}
		if err := w.Flush(); err != nil {
			fmt.Fprintf(stderr, "md2pdf: %v\n", err)
			return 1
		}
		return 0
	}

	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(stderr, "md2pdf: %v\n", err)
		return 1
	}
	w := bufio.NewWriter(f)
	if err := Convert(w, source); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "md2pdf: %v\n", err)
		return 1
	}
	if err := w.Flush(); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "md2pdf: %v\n", err)
		return 1
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(stderr, "md2pdf: %v\n", err)
		return 1
	}
	return 0
}
