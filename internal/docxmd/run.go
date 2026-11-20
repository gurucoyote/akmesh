package docxmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const usage = `usage: docx2md [-o FILE.md] [FILE...]

Prints the Markdown for each .docx file, one after another. Without a file the
document is read from standard input.

With -o the text goes to that file and the pictures are written next to it, with
the Markdown pointing at them. Printing to standard output cannot carry a
picture: there is no folder to put it in, so the description stands in for it.

Headings come from the Word style (Heading 1, berschnitt 1, Title), lists from
the numbering part (bullet becomes -, decimal becomes 1.), tables as a GFM table
with the first row as header, links as [text](url) from the relationship part,
and bold and italic from the run properties, code as backticks (a code
character style or a monospace face). Text that looks like Markdown is escaped.

Pictures come out of word/media and keep the description Word stored as the
text alternative; a drawing without one is called Bild.
`

// Run is the command line of docx2md.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	out := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-h", "--help":
			fmt.Fprint(stderr, usage)
			return 0
		case "-o", "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "docx2md: %s needs a file name\n%s", a, usage)
				return 2
			}
			i++
			out = args[i]
			continue
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
		return RunFileTo("-", data, out, stdout, stderr)
	}
	code := 0
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(stderr, "docx2md: %v\n", err)
			code = 1
			continue
		}
		if c := RunFileTo(name, data, out, stdout, stderr); c != 0 {
			code = c
		}
	}
	return code
}

// RunFile converts one document and writes it out. It is separate from Run so
// that the error path can be tested without a file on disk.
func RunFile(name string, data []byte, out, stderr io.Writer) int {
	return RunFileTo(name, data, "", out, stderr)
}

// RunFileTo converts one document. With outFile the Markdown goes there and the
// pictures are written beside it; without it the text goes to the writer.
func RunFileTo(name string, data []byte, outFile string, out, stderr io.Writer) int {
	if outFile == "" {
		md, err := Convert(data)
		if err != nil {
			fmt.Fprintf(stderr, "docx2md: %s: %v\n", name, err)
			return 1
		}
		fmt.Fprint(out, md)
		return 0
	}
	dir := filepath.Dir(outFile)
	md, images, err := ConvertTo(data, dir)
	if err != nil {
		fmt.Fprintf(stderr, "docx2md: %s: %v\n", name, err)
		return 1
	}
	for _, img := range images {
		p := filepath.Join(dir, img.Name)
		if err := os.WriteFile(p, img.Data, 0o644); err != nil {
			fmt.Fprintf(stderr, "docx2md: %s: %v\n", p, err)
			return 1
		}
	}
	if err := os.WriteFile(outFile, []byte(md), 0o644); err != nil {
		fmt.Fprintf(stderr, "docx2md: %v\n", err)
		return 1
	}
	return 0
}
