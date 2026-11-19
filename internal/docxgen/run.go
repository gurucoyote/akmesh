package docxgen

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const usage = `usage: md2docx [-o FILE] [FILE...]

Renders Markdown as a Word document (.docx). Without FILE the Markdown is read
from standard input; without -o the package goes to standard output, which is
only useful when it is redirected into a file.

Rendered: headings (Word heading styles), paragraphs, bold and italic, inline
code, lists (bullet and numbered, nested), blockquotes, fenced code blocks,
tables, and horizontal rules. Not rendered: images (the description is written
instead), footnotes, comments and tracked changes.
`

// Run is the command line of md2docx.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	out := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stderr, usage)
			return 0
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "md2docx: %s needs a file name\n%s", a, usage)
				return 2
			}
			i++
			out = args[i]
		case len(a) > 0 && a[0] == '-':
			fmt.Fprintf(stderr, "md2docx: unknown option %s\n%s", a, usage)
			return 2
		default:
			files = append(files, a)
		}
	}

	var source []byte
	// baseDir is where relative image paths are resolved: the folder of the
	// first input file, or the working directory when reading standard input.
	baseDir, _ := os.Getwd()
	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "md2docx: %v\n", err)
			return 1
		}
		source = data
	} else {
		for i, name := range files {
			if i == 0 {
				if abs, err := filepath.Abs(name); err == nil {
					baseDir = filepath.Dir(abs)
				}
			}
			data, err := os.ReadFile(name)
			if err != nil {
				fmt.Fprintf(stderr, "md2docx: %v\n", err)
				return 1
			}
			source = append(source, data...)
			source = append(source, '\n')
		}
	}

	if out == "" {
		w := bufio.NewWriter(stdout)
		if err := ConvertWithBase(w, source, baseDir); err != nil {
			fmt.Fprintf(stderr, "md2docx: %v\n", err)
			return 1
		}
		if err := w.Flush(); err != nil {
			fmt.Fprintf(stderr, "md2docx: %v\n", err)
			return 1
		}
		return 0
	}

	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(stderr, "md2docx: %v\n", err)
		return 1
	}
	w := bufio.NewWriter(f)
	if err := ConvertWithBase(w, source, baseDir); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "md2docx: %v\n", err)
		return 1
	}
	if err := w.Flush(); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "md2docx: %v\n", err)
		return 1
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(stderr, "md2docx: %v\n", err)
		return 1
	}
	return 0
}
