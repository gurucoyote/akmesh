package docxmd

import (
	"fmt"
	"io"
	"os"
)

const usage = `usage: docx2md [FILE...]

Prints the Markdown for each .docx file, one after another. Without a file the
document is read from standard input.

Headings come from the Word style (Heading 1, berschnitt 1, Title), lists from
the numbering part (bullet becomes -, decimal becomes 1.), tables as a GFM table
with the first row as header, links as [text](url) from the relationship part,
and bold and italic from the run properties. Text that looks like Markdown is
escaped.
`

// Run is the command line of docx2md.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprint(stderr, usage)
			return 0
		default:
			if len(a) > 0 && a[0] == '-' {
				fmt.Fprintf(stderr, "docx2md: unknown option %s\n%s", a, usage)
				return 2
			}
			files = append(files, a)
		}
	}

	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "docx2md: %v\n", err)
			return 1
		}
		return RunFile("-", data, stdout, stderr)
	}
	code := 0
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(stderr, "docx2md: %v\n", err)
			code = 1
			continue
		}
		if c := RunFile(name, data, stdout, stderr); c != 0 {
			code = c
		}
	}
	return code
}

// RunFile converts one document and writes it out. It is separate from Run so
// that the error path can be tested without a file on disk.
func RunFile(name string, data []byte, out, stderr io.Writer) int {
	md, err := Convert(data)
	if err != nil {
		fmt.Fprintf(stderr, "docx2md: %s: %v\n", name, err)
		return 1
	}
	fmt.Fprint(out, md)
	return 0
}
