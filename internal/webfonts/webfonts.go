// Package webfonts hands out the font programs a book carries with it.
//
// Why fonts at all: the page was laid out in the browser's own faces, and a
// document that asks the reader's machine for "Calibri" or "serif" gives the
// page to whoever happens to be installed - a different width, a different
// look, sometimes a different script coverage. Embedding the program means the
// book looks on every machine the way it looked here.
//
// The set is deliberately small: Lora for running text (with bold and italic)
// and Go Mono for code. Each face is a static instance from go-opentype/fonts,
// the same programs the layout engine measures and the PDF writer embeds, so
// the PDF, the .docx and the .epub agree about what the page looks like.
//
// Licence: Lora and Go Mono ship under the SIL Open Font License 1.1, which
// allows embedding in documents; the fonts are used unmodified. The embedding
// bit in the OS/2 table (fsType) is checked at build time by the tests.
package webfonts

import (
	"github.com/go-opentype/fonts/gomono"
	"github.com/go-opentype/fonts/lora"
)

// Style names a face: what Word calls Regular, Bold, Italic.
type Style string

const (
	Regular    Style = "Regular"
	Bold       Style = "Bold"
	Italic     Style = "Italic"
	BoldItalic Style = "BoldItalic"
)

// Face is one font program with the names it is asked by.
type Face struct {
	Family string // "Lora"
	Style  Style  // Regular, Bold, Italic
	TTF    []byte // the program itself
}

// Family is one font family with its faces.
type Family struct {
	Name  string
	Faces []Face
}

// Sans is the family for running text, Mono for code.
const (
	SansName = "Lora"
	MonoName = "Go Mono"
)

// Book is the full set a document embeds.
func Book() []Family {
	return []Family{
		{Name: SansName, Faces: []Face{
			{Family: SansName, Style: Regular, TTF: lora.TTF},
			{Family: SansName, Style: Bold, TTF: lora.BoldTTF},
			{Family: SansName, Style: Italic, TTF: lora.ItalicTTF},
		}},
		{Name: MonoName, Faces: []Face{
			{Family: MonoName, Style: Regular, TTF: gomono.TTF},
		}},
	}
}

// BookWith is Book() with another family for running text: what the page asked
// for, when it could be fetched. Code keeps Go Mono.
func BookWith(body ...Family) []Family {
	out := Book()
	for _, f := range body {
		if f.Name == "" || len(f.Faces) == 0 {
			continue
		}
		faces := make([]Face, 0, len(f.Faces))
		for _, face := range f.Faces {
			face.Family = f.Name
			faces = append(faces, face)
		}
		out[0] = Family{Name: f.Name, Faces: faces}
		break
	}
	return out
}

// FSType reports the OS/2 embedding permission of a font program: 0 means the
// font may be embedded freely, 2 forbids it, 4 allows printing and previewing
// only, 8 allows editable embedding. Anything with bit 1 set (value 2) must not
// be embedded.
func FS(v []byte) (uint16, bool) {
	if len(v) < 12 {
		return 0, false
	}
	numTables := int(v[4])<<8 | int(v[5])
	const os2Tag = "OS/2"
	for i := range numTables {
		rec := 12 + i*16
		if rec+16 > len(v) {
			return 0, false
		}
		if string(v[rec:rec+4]) != os2Tag {
			continue
		}
		off := int(v[rec+8])<<24 | int(v[rec+9])<<16 | int(v[rec+10])<<8 | int(v[rec+11])
		// fsType sits at offset 8 of the OS/2 table.
		if off+10 > len(v) {
			return 0, false
		}
		return uint16(v[off+8])<<8 | uint16(v[off+9]), true
	}
	return 0, false
}
