package docxgen

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Pictures in a .docx.
//
// The description is not the picture: a reader that can show images wants the
// image, and one that cannot wants the description. Both are written, so the
// same file serves both - Word shows the picture and reads the descr out when
// asked for the text alternative.
//
// What is embedded is decided by the bytes, not by the URL: the file is decoded
// first, and only PNG, JPEG and GIF go in. Everything else keeps its
// replacement text, which is what the caller writes.

// EMU is the unit Word measures lengths in: 914400 to the inch, so 9525 to the
// CSS pixel at 96 dpi.
const emuPerPx = 9525

// contentWidthEMU is how wide a picture may be: the A4 page minus its margins,
// the same numbers the section properties use.
const contentWidthEMU = (11906 - 1417*2) * 635 // 1 twip = 635 EMU

// imageExt returns the extension a picture is stored under, or "" when Word
// cannot show the format.
func imageExt(data []byte) string {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	switch format {
	case "png", "jpeg", "gif":
		return format
	default:
		return ""
	}
}

// loadImage reads the bytes an image node points at: a path relative to the
// document, a file: URL, or an inline data: URL. A remote URL is not fetched -
// a converter that silently reaches into the network is a surprise, and the
// description stays readable without it.
func loadImage(raw, baseDir string) ([]byte, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, false
	}
	if strings.HasPrefix(s, "data:") {
		comma := strings.Index(s, ",")
		if comma < 0 {
			return nil, false
		}
		meta, payload := s[:comma], s[comma+1:]
		var data []byte
		var err error
		if strings.Contains(meta, ";base64") {
			data, err = base64.StdEncoding.DecodeString(payload)
		} else {
			var unescaped string
			unescaped, err = url.PathUnescape(payload)
			data = []byte(unescaped)
		}
		if err != nil || len(data) == 0 {
			return nil, false
		}
		return data, true
	}
	if strings.HasPrefix(s, "file:") {
		u, err := url.Parse(s)
		if err != nil {
			return nil, false
		}
		s = u.Path
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return nil, false
	}
	if !filepath.IsAbs(s) {
		s = filepath.Join(baseDir, s)
	}
	data, err := os.ReadFile(s)
	if err != nil {
		return nil, false
	}
	return data, true
}

// picture writes an image as a run and reports whether it could. It writes a
// run and no paragraph: a w:drawing inside a w:p that is already open is a
// nested paragraph, which Word and LibreOffice refuse to show (measured in the
// joshua project, 2026-11-19: the pictures vanished after the export).
func (b *builder) picture(raw, alt string) bool {
	data, ok := loadImage(raw, b.baseDir)
	if !ok {
		return false
	}
	ext := imageExt(data)
	if ext == "" {
		return false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return false
	}
	name := "media/bild" + strconv.Itoa(len(b.images)+1) + "." + ext
	b.images = append(b.images, imagePart{name: name, data: data, ext: ext})
	id := "rId" + strconv.Itoa(b.imageBase()+len(b.images)-1)

	wEmu := int64(cfg.Width) * emuPerPx
	hEmu := int64(cfg.Height) * emuPerPx
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
	hey := strconv.FormatInt(hEmu, 10)
	b.out.WriteString(`<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">` +
		`<wp:extent cx="` + wStr + `" cy="` + hey + `"/>` +
		`<wp:docPr id="` + n + `" name="Bild` + n + `" descr="` + escape(alt) + `"/>` +
		`<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<pic:nvPicPr><pic:cNvPr id="` + n + `" name="Bild` + n + `" descr="` + escape(alt) + `"/>` +
		`<pic:cNvPicPr/></pic:nvPicPr>` +
		`<pic:blipFill><a:blip r:embed="` + id + `"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>` +
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + wStr + `" cy="` + hey + `"/>` +
		`</a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>` +
		`</pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`)
	return true
}
