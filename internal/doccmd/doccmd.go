// Package doccmd is the command table of akmesh-docs, the document binary.
//
// The table is the binary's face: what it answers to, and what it says when a
// name is unknown. It is a table of its own and not a view into the full
// command set, because exactly that is the point of the split - a binary that
// carries the document readers must not drag the file tools or the network
// stack along (docs/design/split.md, acmesh-h3ux).
package doccmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"akmesh/internal/anymd"
	"akmesh/internal/cmdrun"
	"akmesh/internal/docxgen"
	"akmesh/internal/epubgen"
	"akmesh/internal/epubmd"
	"akmesh/internal/md2txt"
	"akmesh/internal/pdfgen"
	"akmesh/internal/pdfmd"
	"akmesh/internal/policy"
	"akmesh/internal/rgcmd"
	"akmesh/internal/usage"
)

// pinned builds the reader with its format fixed, the way the *2md mains do.
func pinned(prog string, f anymd.Format) cmdrun.Func {
	return func(args []string, stdin, stdout, stderr *os.File) int {
		return anymd.RunAs(prog, f, args, stdin, stdout, stderr)
	}
}

// runRG goes through the shared help gate; rg takes io.Reader and io.Writer
// instead of files, which is why it needs the IO variant.
func runRG(args []string, stdin, stdout, stderr *os.File) int {
	return usage.RunIO("rg", args, stdin, stdout, stderr, rgcmd.Run)
}

// Commands is what akmesh-docs can do: the document readers and writers plus
// rg, which needs the document reader for --pre.
var Commands = map[string]cmdrun.Func{
	"any2md":    anymd.Run,
	"pdf2md":    pinned("pdf2md", anymd.PDF),
	"docx2md":   pinned("docx2md", anymd.DOCX),
	"epub2md":   pinned("epub2md", anymd.EPUB),
	"rtf2md":    pinned("rtf2md", anymd.RTF),
	"html2md":   pinned("html2md", anymd.HTML),
	"md2txt-l":  md2txt.Run,
	"md2pdf":    pdfgen.Run,
	"md2docx":   docxgen.Run,
	"md2epub":   epubgen.Run,
	"html2docx": docxgen.RunHTML,
	"html2epub": epubgen.RunHTML,
	"epub2html": epubmd.RunHTML,
	"pdf2html":  pdfmd.RunHTML,
	"rg":        runRG,
}

// Names lists the commands in the order the help prints them.
func Names() []string {
	out := make([]string, 0, len(Commands))
	for name := range Commands {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Main is the entry point of the binary. It is called from cmd/akmesh-docs and
// from the tests, so that both exercise the same dispatch.
//
// Two ways in, as for every multicall binary: the name the program was started
// as (a symlink named "rg"), or the first argument (akmesh-docs rg). A name it
// does not have is an error with the list, never a silent success.
func Main() int {
	// The policy comes first: a rule that only holds in some binaries holds
	// nowhere.
	policy.Enter()
	return Run(os.Args, os.Stdin, os.Stdout, os.Stderr)
}

// Run is the dispatch without the policy: argv is the whole argument vector,
// and the streams are passed in so that a test sees what a caller sees.
func Run(argv []string, stdin, stdout, stderr *os.File) int {
	name := "akmesh-docs"
	if len(argv) > 0 && argv[0] != "" {
		name = filepath.Base(argv[0])
	}
	var args []string
	if len(argv) > 1 {
		args = argv[1:]
	}
	if _, ok := Commands[name]; !ok {
		if len(args) == 0 {
			usageList(name, stderr)
			return 2
		}
		if _, ok := Commands[args[0]]; ok {
			name, args = args[0], args[1:]
		} else {
			fmt.Fprintf(stderr, "%s: unknown command %q\n", name, args[0])
			usageList(name, stderr)
			return 2
		}
	}
	return cmdrun.RunArgs(Commands, name, args, stdin, stdout, stderr)
}

// usageList names this binary's commands and points at the other packages: a
// command that is not here must not look as if it were, and the caller has to
// learn where it does live (docs/design/split.md).
func usageList(prog string, w io.Writer) {
	fmt.Fprintf(w, "usage: %s COMMAND [ARG...]\n", prog)
	fmt.Fprintf(w, "commands: %s\n", strings.Join(Names(), " "))
}
