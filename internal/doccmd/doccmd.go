// Package doccmd is the command table of akmesh-docs, the reading binary.
//
// The table is the binary's face: what it answers to, and what it says when a
// name is unknown. It is a table of its own and not a view into the full
// command set, because exactly that is the point of the split - a binary that
// carries the document readers must not drag the file tools or the network
// stack along (docs/design/split.md, acmesh-h3ux).
//
// The writers (md2pdf, md2docx, md2epub, html2docx, html2epub) moved to
// akmesh-write on 25.11.2026: they cost 5.5 MiB in libraries, the readers in
// this binary only 61 KB on top of them, and a harness that reads documents has
// no use for the writing side (acmesh-1zzk).
package doccmd

import (
	"os"

	"akmesh/internal/anymd"
	"akmesh/internal/cmdrun"
	"akmesh/internal/epubmd"
	"akmesh/internal/md2txt"
	"akmesh/internal/multicall"
	"akmesh/internal/pdfmd"
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

// Prog is this binary: the document readers, md2txt-l and rg, which needs the
// document reader for --pre.
var Prog = multicall.Prog{
	Name: "akmesh-docs",
	Commands: map[string]cmdrun.Func{
		"any2md":   anymd.Run,
		"pdf2md":   pinned("pdf2md", anymd.PDF),
		"docx2md":  pinned("docx2md", anymd.DOCX),
		"epub2md":  pinned("epub2md", anymd.EPUB),
		"rtf2md":   pinned("rtf2md", anymd.RTF),
		"html2md":  pinned("html2md", anymd.HTML),
		"md2txt-l": md2txt.Run,
		"epub2html": func(args []string, stdin, stdout, stderr *os.File) int {
			return epubmd.RunHTML(args, stdin, stdout, stderr)
		},
		"pdf2html": func(args []string, stdin, stdout, stderr *os.File) int {
			return pdfmd.RunHTML(args, stdin, stdout, stderr)
		},
		"rg": runRG,
	},
	Others: []string{"akmesh", "akmesh-write"},
}

// Main is the entry point of the binary, called from cmd/akmesh-docs.
func Main() int { return Prog.Main() }

// Run is the dispatch without the policy, so a test sees what a caller sees.
func Run(argv []string, stdin, stdout, stderr *os.File) int {
	return Prog.Run(argv, stdin, stdout, stderr)
}

// Names lists this binary's commands.
func Names() []string { return Prog.Names() }
