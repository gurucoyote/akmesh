// Command akmesh-write is the writing binary: Markdown or HTML out as PDF, docx
// or EPUB.
//
// It is separate from akmesh-docs because these are the commands that cost
// libraries; a harness that only reads documents installs the reader instead
// (acmesh-1zzk).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/writecmd"
)

func main() {
	os.Exit(writecmd.Main())
}
