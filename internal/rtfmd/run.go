package rtfmd

import (
	"fmt"
	"io"
	"os"
)

const usage = `usage: rtf2md [FILE...]

Prints the Markdown for each RTF file, one after another. Without a file the
document is read from standard input.

Paragraphs come from \par (and from a backslash at the end of a line), \line is
a break inside the paragraph, \tab a tab; \b and \i become ** and *; \'hh is one
byte in the document's code page and \uN a character by number; headings come
from the style of the paragraph and its name in the stylesheet. The font table,
the colour table, the document info and pictures are skipped.
`

// Run is the command line of rtf2md.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprint(stderr, usage)
			return 0
		default:
			if len(a) > 0 && a[0] == '-' {
				fmt.Fprintf(stderr, "rtf2md: unknown option %s\n%s", a, usage)
				return 2
			}
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "rtf2md: %v\n", err)
			return 1
		}
		return RunFile("-", data, stdout, stderr)
	}
	code := 0
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(stderr, "rtf2md: %v\n", err)
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
	md, err := Convert(data)
	if err != nil {
		fmt.Fprintf(stderr, "rtf2md: %s: %v\n", name, err)
		return 1
	}
	fmt.Fprint(out, md)
	return 0
}
