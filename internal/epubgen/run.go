package epubgen

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const usage = `usage: md2epub [-o FILE] [--title TITEL] [--author NAME] [FILE...]

Renders Markdown as an EPUB 3 book. Without FILE the Markdown is read from
standard input; without -o the book goes to standard output, which is only useful
when it is redirected into a file.

Every level-1 heading starts a chapter; a document without one becomes a single
chapter. The book gets a table of contents, the title in its metadata and a
stylesheet that a reader can render without guessing.
`

// Run is the command line of md2epub.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	out, title, author := "", "", ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stderr, usage)
			return 0
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "md2epub: %s needs a file name\n%s", a, usage)
				return 2
			}
			i++
			out = args[i]
		case a == "--title" || a == "-t":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "md2epub: %s needs a title\n%s", a, usage)
				return 2
			}
			i++
			title = args[i]
		case strings.HasPrefix(a, "--title="):
			title = strings.TrimPrefix(a, "--title=")
		case a == "--author" || a == "-a":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "md2epub: %s needs a name\n%s", a, usage)
				return 2
			}
			i++
			author = args[i]
		case strings.HasPrefix(a, "--author="):
			author = strings.TrimPrefix(a, "--author=")
		case len(a) > 0 && a[0] == '-':
			fmt.Fprintf(stderr, "md2epub: unknown option %s\n%s", a, usage)
			return 2
		default:
			files = append(files, a)
		}
	}

	source, code := readSource(files, stdin, stderr)
	if code != 0 {
		return code
	}
	// The title falls back to the file name, which is what a reader wants to see
	// in its library when nobody named the book.
	if title == "" && len(files) > 0 {
		base := filepath.Base(files[0])
		title = strings.TrimSuffix(base, filepath.Ext(base))
	}
	opts := Options{Title: title, Author: author}
	// Pictures are resolved against the folder of the first input file, or the
	// working directory when the Markdown comes from standard input.
	if baseDir, err := os.Getwd(); err == nil {
		opts.BaseDir = baseDir
	}
	if len(files) > 0 {
		if abs, err := filepath.Abs(files[0]); err == nil {
			opts.BaseDir = filepath.Dir(abs)
		}
	}

	if out == "" {
		w := bufio.NewWriter(stdout)
		if err := Convert(w, source, opts); err != nil {
			fmt.Fprintf(stderr, "md2epub: %v\n", err)
			return 1
		}
		if err := w.Flush(); err != nil {
			fmt.Fprintf(stderr, "md2epub: %v\n", err)
			return 1
		}
		return 0
	}
	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(stderr, "md2epub: %v\n", err)
		return 1
	}
	w := bufio.NewWriter(f)
	if err := Convert(w, source, opts); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "md2epub: %v\n", err)
		return 1
	}
	if err := w.Flush(); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "md2epub: %v\n", err)
		return 1
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(stderr, "md2epub: %v\n", err)
		return 1
	}
	return 0
}

// readSource reads the Markdown from the arguments or from standard input.
func readSource(files []string, stdin *os.File, stderr *os.File) ([]byte, int) {
	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "md2epub: %v\n", err)
			return nil, 1
		}
		return data, 0
	}
	var source []byte
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(stderr, "md2epub: %v\n", err)
			return nil, 1
		}
		source = append(source, data...)
		source = append(source, '\n')
	}
	return source, 0
}
