package htmlmd

import (
	"fmt"
	"io"
	"os"
)

const usage = `usage: html2md [FILE...]

Prints the Markdown for each HTML fragment, one after another. Without a file
the fragment is read from standard input. A fragment is what it says: a whole
page works, and so does a single paragraph.

The rules follow Turndown and JohannesKaufmann/html-to-markdown: headings become
#, strong/em become ** and *, links [text](url), images ![alt](src), lists - and
1., blockquote >, pre a fenced block, hr ---, and Markdown's own characters in
the text are escaped so the wording survives.
`

// Run is the command line of html2md.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprint(stderr, usage)
			return 0
		default:
			if len(a) > 0 && a[0] == '-' {
				fmt.Fprintf(stderr, "html2md: unknown option %s\n%s", a, usage)
				return 2
			}
			files = append(files, a)
		}
	}

	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "html2md: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, Convert(string(data)))
		return 0
	}

	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(stderr, "html2md: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, Convert(string(data)))
	}
	return 0
}
