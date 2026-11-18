package pdfmd

import (
	"fmt"
	"io"
	"os"
)

const usage = `usage: pdf2md [FILE...]

Prints the text of each PDF file, one page after another. Without a file the
document is read from standard input.

A PDF carries no structure, only positions, so what comes out is text: one
paragraph per line, no headings, no table. A scanned page has no text at all
(only images) and yields nothing; an encrypted document is refused with a
message. Page order follows the page tree, not the order of the objects.
`

// Run is the command line of pdf2md.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprint(stderr, usage)
			return 0
		default:
			if len(a) > 0 && a[0] == '-' {
				fmt.Fprintf(stderr, "pdf2md: unknown option %s\n%s", a, usage)
				return 2
			}
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "pdf2md: %v\n", err)
			return 1
		}
		return RunFile("-", data, stdout, stderr)
	}
	code := 0
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(stderr, "pdf2md: %v\n", err)
			code = 1
			continue
		}
		if c := RunFile(name, data, stdout, stderr); c != 0 {
			code = c
		}
	}
	return code
}

// RunFile converts one document and writes it out.
func RunFile(name string, data []byte, out, stderr io.Writer) int {
	text, err := Convert(data)
	if err != nil {
		fmt.Fprintf(stderr, "pdf2md: %s: %v\n", name, err)
		return 1
	}
	fmt.Fprint(out, text)
	return 0
}
