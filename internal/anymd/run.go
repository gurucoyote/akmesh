// Package anymd reads a document and prints it as Markdown, whatever the
// document is. It is the one place the five readers meet: pdf2md, docx2md,
// epub2md, rtf2md and html2md are aliases that pin their format and call this,
// so a fix in the reading path reaches every name at once.
//
// The format is decided by the bytes, not by the file name (see Detect), because
// an agent that has a document in hand does not always know what it holds, and
// the flag it would have to guess is exactly the choice this command takes away.
package anymd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gurucoyote/akmesh/internal/docxmd"
	"github.com/gurucoyote/akmesh/internal/epubmd"
	"github.com/gurucoyote/akmesh/internal/htmlmd"
	"github.com/gurucoyote/akmesh/internal/pdfmd"
	"github.com/gurucoyote/akmesh/internal/rtfmd"
)

const usage = `usage: any2md [-o FILE.md] [-f FORMAT] [FILE...]

Prints the Markdown of each document, one after another, whatever the document
is: PDF, docx, EPUB, RTF or HTML. The format is read from the bytes, so the file
name does not matter and a document without its suffix still works. Without FILE
the document is read from standard input. A single "-" stands for standard input.

-f, --from FORMAT
      read the input as this format instead of detecting it; one of
      pdf, docx, epub, rtf, html

-o, --output FILE.md
      write the Markdown to that file instead of standard output, and the
      pictures of a docx next to it. Printing to standard output cannot carry a
      picture: there is no folder to put it in, so the description stands in for
      it. -o takes one document; for several, redirect instead.

What comes out is what the Markdown reader of that format can say; for a PDF
that is text without structure, because a PDF carries positions, not headings.
The named commands (pdf2md, docx2md, epub2md, rtf2md, html2md) do the same and
pin their format: a script that means "this must be a PDF" still says pdf2md.
`

// aliasUsage is the help of the five names that pin a format. They live here
// rather than in their packages because the alias is only a name for this
// command: a second help text beside a second implementation is how five readers
// drift apart again. The texts keep the detail of the old commands, so an agent
// that had learned one of them finds the same words.
var aliasUsage = map[Format]string{
	PDF: `usage: pdf2md [FILE...]

The reader of any2md with the format fixed to pdf: whatever the bytes say, they
are read as a PDF, and a file that is recognisably another format is refused
instead of being guessed at.

Prints the text of each PDF file, one page after another. Without a file the
document is read from standard input.

A PDF carries no structure, only positions, so what comes out is text: one
paragraph per line, no headings, no table. A scanned page has no text at all
(only images) and yields nothing; an encrypted document is refused with a
message. Page order follows the page tree, not the order of the objects.
`,
	DOCX: `usage: docx2md [-o FILE.md] [FILE...]

The reader of any2md with the format fixed to docx.

Prints the Markdown for each .docx file, one after another. Without a file the
document is read from standard input.

With -o the text goes to that file and the pictures are written next to it, with
the Markdown pointing at them. Printing to standard output cannot carry a
picture: there is no folder to put it in, so the description stands in for it.

Headings come from the Word style (Heading 1, Überschrift 1, Title), lists from
the numbering part (bullet becomes -, decimal becomes 1.), tables as a GFM table
with the first row as header, links as [text](url) from the relationship part,
and bold and italic from the run properties, code as backticks (a code
character style or a monospace face). Text that looks like Markdown is escaped.

Pictures come out of word/media and keep the description Word stored as the
text alternative; a drawing without one is called Bild.
`,
	EPUB: `usage: epub2md [FILE...]

The reader of any2md with the format fixed to epub.

Reads EPUB books and writes their text as Markdown. Without FILE the book is read
from standard input, so that a book can be piped in.

Chapters are written in the order the book declares them, each starting with its
heading. A table of contents is not a chapter and is skipped.
`,
	RTF: `usage: rtf2md [FILE...]

The reader of any2md with the format fixed to rtf.

Prints the Markdown for each RTF file, one after another. Without a file the
document is read from standard input.

Paragraphs come from \par (and from a backslash at the end of a line), \line is
a break inside the paragraph, \tab a tab; \b and \i become ** and *; \'hh is one
byte in the document's code page and \uN a character by number; headings come
from the style of the paragraph and its name in the stylesheet. The font table,
the colour table, the document info and pictures are skipped.
`,
	HTML: `usage: html2md [FILE...]

The reader of any2md with the format fixed to html.

Prints the Markdown for each HTML fragment, one after another. Without a file
the fragment is read from standard input. A fragment is what it says: a whole
page works, and so does a single paragraph.

The rules follow Turndown and JohannesKaufmann/html-to-markdown: headings become
#, strong/em become ** and *, links [text](url), images ![alt](src), lists - and
1., blockquote >, pre a fenced block, hr ---, and Markdown's own characters in
the text are escaped so the wording survives.
`,
}

// Run is the command line of any2md: the format is detected.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return run("any2md", "", args, stdin, stdout, stderr)
}

// RunAs is the entry point of an alias. pinned names the format the command
// stands for and is the format used when the bytes cannot be detected; a
// document detected as a different format is refused, so that pdf2md keeps
// refusing an HTML file the way it always did.
func RunAs(prog string, pinned Format, args []string, stdin *os.File, stdout *os.File, stderr *os.File) int {
	return run(prog, pinned, args, stdin, stdout, stderr)
}

func run(prog string, pinned Format, args []string, stdin *os.File, stdout *os.File, stderr *os.File) int {
	// An alias answers --help with its own text: it is the name the caller
	// typed, and the paragraph that explains the format is the same one.
	help := usage
	if t, ok := aliasUsage[pinned]; ok {
		help = t
	}
	var files []string
	outFile := ""
	forced := pinned
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stderr, help)
			return 0
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "%s: %s needs a file name\n%s", prog, a, help)
				return 2
			}
			i++
			outFile = args[i]
		case a == "-f" || a == "--from":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "%s: %s needs a format\n%s", prog, a, help)
				return 2
			}
			i++
			f, ok := ParseFormat(args[i])
			if !ok {
				fmt.Fprintf(stderr, "%s: %s: unknown format %q\n%s", prog, a, args[i], help)
				return 2
			}
			forced = f
		case strings.HasPrefix(a, "--from="):
			f, ok := ParseFormat(strings.TrimPrefix(a, "--from="))
			if !ok {
				fmt.Fprintf(stderr, "%s: unknown format %q\n%s", prog, strings.TrimPrefix(a, "--from="), help)
				return 2
			}
			forced = f
		case a == "-":
			files = append(files, "")
		case len(a) > 0 && a[0] == '-':
			fmt.Fprintf(stderr, "%s: unknown option %s\n%s", prog, a, help)
			return 2
		default:
			files = append(files, a)
		}
	}

	if pinned != "" && forced != pinned {
		fmt.Fprintf(stderr, "%s: the format is fixed to %s; any2md detects it\n", prog, pinned)
		return 2
	}
	if outFile != "" && len(files) > 1 {
		fmt.Fprintf(stderr, "%s: -o takes one document, but %d were named; redirect instead\n", prog, len(files))
		return 2
	}

	w := bufio.NewWriter(stdout)
	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", prog, err)
			return 1
		}
		code := conv(prog, "-", data, forced, outFile, w, stderr)
		w.Flush()
		return code
	}

	code := 0
	for _, name := range files {
		var data []byte
		var err error
		if name == "" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(name)
		}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", prog, err)
			code = 1
			continue
		}
		label := name
		if label == "" {
			label = "-"
		}
		if c := conv(prog, label, data, forced, outFile, w, stderr); c != 0 {
			code = c
		}
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", prog, err)
		code = 1
	}
	return code
}

// conv converts one document. forced skips detection; otherwise the bytes decide
// and a document of no known format is refused with the list of what any2md
// reads.
func conv(prog, name string, data []byte, forced Format, outFile string, out io.Writer, stderr io.Writer) int {
	f := forced
	if f == "" {
		detected, ok := Detect(data)
		if !ok {
			fmt.Fprintf(stderr, "%s: %v\n", prog, UnknownError(name))
			return 1
		}
		f = detected
	} else if detected, ok := Detect(data); ok && detected != f {
		fmt.Fprintf(stderr, "%s: %s: not a %s, the bytes look like %s\n", prog, name, f, detected)
		return 1
	}

	md, images, err := convertAs(f, data, outFile)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s: %v\n", prog, name, err)
		return 1
	}
	if outFile != "" {
		dir := filepath.Dir(outFile)
		for _, img := range images {
			p := filepath.Join(dir, img.Name)
			if err := os.WriteFile(p, img.Data, 0o644); err != nil {
				fmt.Fprintf(stderr, "%s: %s: %v\n", prog, p, err)
				return 1
			}
		}
		if err := os.WriteFile(outFile, []byte(md), 0o644); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", prog, err)
			return 1
		}
		return 0
	}
	if _, err := io.WriteString(out, md); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", prog, err)
		return 1
	}
	return 0
}

// convertAs hands the bytes to the reader of that format. Only docx carries
// pictures; the others take no image folder, so outFile only decides where the
// text goes.
func convertAs(f Format, data []byte, outFile string) (string, []docxmd.Image, error) {
	switch f {
	case PDF:
		md, err := pdfmd.Convert(data)
		return md, nil, err
	case RTF:
		md, err := rtfmd.Convert(data)
		return md, nil, err
	case EPUB:
		md, err := epubmd.Convert(data)
		return md, nil, err
	case HTML:
		return htmlmd.Convert(string(data)), nil, nil
	case DOCX:
		if outFile != "" {
			return docxmd.ConvertTo(data, filepath.Dir(outFile))
		}
		md, err := docxmd.Convert(data)
		return md, nil, err
	}
	return "", nil, fmt.Errorf("unknown document format %q", f)
}
