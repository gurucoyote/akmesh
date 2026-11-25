// Package writecmd is the command table of akmesh-write, the writing binary:
// Markdown and HTML go out as PDF, docx or EPUB.
//
// It is separate from akmesh-docs because the writers are what costs: goldmark-pdf,
// gofpdf, the fonts, go-epub and the docx library make 5.5 MiB, while the
// readers in akmesh-docs add 61 KB on top of the same libraries. A harness that
// reads documents should not pay for the writing side (acmesh-1zzk).
package writecmd

import (
	"os"

	"akmesh/internal/cmdrun"
	"akmesh/internal/docxgen"
	"akmesh/internal/epubgen"
	"akmesh/internal/multicall"
	"akmesh/internal/pdfgen"
)

// Prog is this binary.
var Prog = multicall.Prog{
	Name: "akmesh-write",
	Commands: map[string]cmdrun.Func{
		"md2pdf":    pdfgen.Run,
		"md2docx":   docxgen.Run,
		"md2epub":   epubgen.Run,
		"html2docx": docxgen.RunHTML,
		"html2epub": epubgen.RunHTML,
	},
	Others: []string{"akmesh", "akmesh-docs"},
}

// Main is the entry point of the binary, called from cmd/akmesh-write.
func Main() int { return Prog.Main() }

// Run is the dispatch without the policy, so a test sees what a caller sees.
func Run(argv []string, stdin, stdout, stderr *os.File) int {
	return Prog.Run(argv, stdin, stdout, stderr)
}

// Names lists this binary's commands.
func Names() []string { return Prog.Names() }
