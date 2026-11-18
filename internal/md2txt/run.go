package md2txt

import (
	"fmt"
	"io"
	"os"
)

const usage = `usage: md2txt-l [FILE...]

Prints the plain text of Markdown: the way pandoc -t plain writes it. Headings
keep their text but lose the marks, emphasis and code spans lose theirs, a link
keeps its text, an image becomes its description in brackets, lists are indented
by two spaces a level, code blocks by four, tables keep their columns, and a
paragraph is wrapped at 72 characters.
`

// Run is the command line of md2txt-l, the lightweight variant: it reads Markdown
// itself and produces plain text byte for byte like pandoc -t plain.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprint(stderr, usage)
			return 0
		default:
			if len(a) > 0 && a[0] == '-' {
				fmt.Fprintf(stderr, "md2txt-l: unknown option %s\n%s", a, usage)
				return 2
			}
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "md2txt-l: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, Convert(string(data)))
		return 0
	}
	code := 0
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(stderr, "md2txt-l: %v\n", err)
			code = 1
			continue
		}
		fmt.Fprint(stdout, Convert(string(data)))
	}
	return code
}
