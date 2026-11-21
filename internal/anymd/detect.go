package anymd

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

// Format is a document format any2md can read. The strings are the words the
// help and the error messages use, so they are spelled the way an agent would
// say them.
type Format string

// The formats any2md reads. Each one has a command of its own as well, and that
// command pins its format instead of detecting it: pdf2md refuses anything that
// is not a PDF, which is what a script wants.
const (
	PDF  Format = "pdf"
	DOCX Format = "docx"
	EPUB Format = "epub"
	RTF  Format = "rtf"
	HTML Format = "html"
)

// Formats lists the formats in the order the help mentions them.
var Formats = []Format{PDF, DOCX, EPUB, RTF, HTML}

// names maps a format to the words Detect accepts for it, so that "-f" and the
// messages cannot drift from the exported constants.
func valid(f Format) bool {
	for _, x := range Formats {
		if x == f {
			return true
		}
	}
	return false
}

// ParseFormat turns the word an agent typed into a format. The empty string is
// not a format: the caller decides what an empty flag means.
func ParseFormat(s string) (Format, bool) {
	f := Format(strings.ToLower(strings.TrimSpace(s)))
	return f, valid(f)
}

// Detect names the format of data from its bytes, never from a file name: a
// document mailed without its suffix still has to be read, and a .txt that holds
// HTML is HTML. The second result is false when the bytes are not one of the
// formats any2md reads - guessing there would send the PDF reader a PNG and
// print its error at the caller, which is a worse message than this one.
func Detect(data []byte) (Format, bool) {
	b := bytes.TrimLeft(data, " \t\r\n\ufeff")
	switch {
	case bytes.HasPrefix(b, []byte("%PDF-")):
		return PDF, true
	case bytes.HasPrefix(b, []byte("{\\rtf")):
		return RTF, true
	case bytes.HasPrefix(b, []byte("PK\x03\x04")):
		return detectZip(data)
	case len(b) > 2 && b[0] == '<':
		return HTML, true
	}
	return "", false
}

// detectZip tells Word from an EPUB. Both are zips; only their entries say
// which is which, and a zip with neither marker stays unknown on purpose.
func detectZip(data []byte) (Format, bool) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", false
	}
	var word, epub bool
	for _, f := range r.File {
		switch f.Name {
		case "word/document.xml":
			word = true
		case "META-INF/container.xml":
			epub = true
		}
	}
	switch {
	case epub:
		return EPUB, true
	case word:
		return DOCX, true
	}
	return "", false
}

// UnknownError is the message a file of no known format produces. It names the
// formats, because an agent that reads it knows what to do next, while
// "unsupported" leaves it guessing.
func UnknownError(name string) error {
	words := make([]string, len(Formats))
	for i, f := range Formats {
		words[i] = string(f)
	}
	return fmt.Errorf("%s: unknown document format (any2md reads %s)",
		name, strings.Join(words, ", "))
}
