package epubmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const usage = `usage: epub2md [FILE...]

Reads EPUB books and writes their text as Markdown. Without FILE the book is read
from standard input, so that a book can be piped in.

Chapters are written in the order the book declares them, each starting with its
heading. A table of contents is not a chapter and is skipped.
`

// Run is the command line of epub2md.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stderr, usage)
			return 0
		case len(a) > 0 && a[0] == '-' && a != "-":
			fmt.Fprintf(stderr, "epub2md: unknown option %s\n%s", a, usage)
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
			fmt.Fprintf(stderr, "epub2md: %v\n", err)
			return 1
		}
		return write(w, data, stderr)
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
			fmt.Fprintf(stderr, "epub2md: %v\n", err)
			return 1
		}
		if code := write(w, data, stderr); code != 0 {
			return code
		}
	}
	return 0
}

// write converts one book and puts its Markdown on the writer.
func write(w *bufio.Writer, data []byte, stderr *os.File) int {
	md, err := Convert(data)
	if err != nil {
		fmt.Fprintf(stderr, "epub2md: %v\n", err)
		return 1
	}
	if _, err := w.WriteString(md); err != nil {
		fmt.Fprintf(stderr, "epub2md: %v\n", err)
		return 1
	}
	return 0
}
