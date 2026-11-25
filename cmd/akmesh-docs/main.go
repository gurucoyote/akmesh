// Command akmesh-docs is the document binary: the readers and writers for PDF,
// docx, EPUB, RTF and HTML, Markdown as the hub, and rg for searching them.
//
// It is a dispatcher of its own, not a mode of akmesh. Whoever needs only
// "document to Markdown" installs this instead of the full tool kit
// (docs/design/split.md, acmesh-h3ux).
package main

import (
	"os"

	"akmesh/internal/doccmd"
)

func main() {
	os.Exit(doccmd.Main())
}
