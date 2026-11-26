package docxgen

import (
	"strconv"

	"github.com/gurucoyote/akmesh/internal/imgload"
)

// Pictures in a .docx.
//
// The description is not the picture: a reader that can show images wants the
// image, and one that cannot wants the description. Both are written, so the
// same file serves both - Word shows the picture and reads the descr out when
// asked for the text alternative.
//
// Which bytes are embedded, and whether there are any at all, is decided in
// internal/imgload: the same loader feeds md2epub, so a document and a book
// either both carry the picture or both keep the replacement text.

// EMU is the unit Word measures lengths in: 914400 to the inch, so 9525 to the
// CSS pixel at 96 dpi.
const emuPerPx = 9525

// contentWidthEMU is how wide a picture may be: the A4 page minus its margins,
// the same numbers the section properties use.
const contentWidthEMU = (11906 - 1417*2) * 635 // 1 twip = 635 EMU

// picture writes an image as a run and reports whether it could. It writes a
// run and no paragraph: a w:drawing inside a w:p that is already open is a
// nested paragraph, which Word and LibreOffice refuse to show (measured in the
// joshua project, 2026-11-19: the pictures vanished after the export).
func (b *builder) picture(raw, alt string) bool {
	img, ok := imgload.Load(raw, b.baseDir)
	if !ok {
		return false
	}
	name := "media/bild" + strconv.Itoa(len(b.images)+1) + "." + img.Format
	b.images = append(b.images, imagePart{name: name, data: img.Data, ext: img.Format})
	id := "rId" + strconv.Itoa(b.imageBase()+len(b.images)-1)

	wEmu := int64(img.Width) * emuPerPx
	hEmu := int64(img.Height) * emuPerPx
	if wEmu > contentWidthEMU {
		hEmu = hEmu * contentWidthEMU / wEmu
		wEmu = contentWidthEMU
	}
	if alt == "" {
		// A picture without a description gets a neutral one rather than an
		// empty descr, which a screen reader would read as nothing.
		alt = "Bild"
	}
	n := strconv.Itoa(len(b.images))
	wStr := strconv.FormatInt(wEmu, 10)
	hStr := strconv.FormatInt(hEmu, 10)
	b.out.WriteString(`<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">` +
		`<wp:extent cx="` + wStr + `" cy="` + hStr + `"/>` +
		`<wp:docPr id="` + n + `" name="Bild` + n + `" descr="` + escape(alt) + `"/>` +
		`<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<pic:nvPicPr><pic:cNvPr id="` + n + `" name="Bild` + n + `" descr="` + escape(alt) + `"/>` +
		`<pic:cNvPicPr/></pic:nvPicPr>` +
		`<pic:blipFill><a:blip r:embed="` + id + `"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>` +
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + wStr + `" cy="` + hStr + `"/>` +
		`</a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>` +
		`</pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`)
	return true
}
