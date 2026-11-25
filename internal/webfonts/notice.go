package webfonts

import (
	"fmt"
	"strings"
)

// Licence notice. Every TrueType font carries its terms in the name table: the
// copyright line (name ID 0), the licence text (13) and a licence URL (14).
// Reading them is the only honest way to say what a document ships - and
// shipping the terms next to the program is what the OFL asks for anything that
// embeds it.
//
// Not every font fills every field: Open Sans has copyright and URL but no
// licence text, Lora and Go Mono have all three (measured 2026-11-19).

// Notice is what a font says about itself.
type Notice struct {
	Family      string
	Copyright   string
	LicenseText string
	LicenseURL  string
}

// LicenseName is the short name of the licence, guessed from the URL and the
// text: enough for a notice, not a legal opinion.
func (n Notice) LicenseName() string {
	u := strings.ToLower(n.LicenseURL)
	t := strings.ToLower(n.LicenseText)
	switch {
	case strings.Contains(u, "/ofl") || strings.Contains(t, "open font license"):
		return "SIL Open Font License 1.1"
	case strings.Contains(u, "apache") || strings.Contains(t, "apache license"):
		return "Apache License 2.0"
	case strings.Contains(u, "creativecommons") || strings.Contains(t, "creative commons"):
		return "Creative Commons"
	case strings.Contains(t, "bitstream vera") || strings.Contains(u, "gnome.org/fonts"):
		return "Bitstream Vera / GNOME"
	case strings.Contains(t, "redistribution and use in source and binary forms"):
		// A BSD licence: clause three is what makes it the 3-clause variant,
		// and the no-endorsement sentence is in the text for Go Mono.
		if strings.Contains(t, "neither the name") || strings.Contains(t, "endorse") {
			return "BSD 3-Clause"
		}
		return "BSD 2-Clause"
	}
	return "unbekannt"
}

// String is one line for a notice file.
func (n Notice) String() string {
	var b strings.Builder
	b.WriteString(n.Family)
	if n.Copyright != "" {
		b.WriteString(" - " + n.Copyright)
	}
	b.WriteString(" - Lizenz: " + n.LicenseName())
	if n.LicenseURL != "" {
		b.WriteString(" <" + n.LicenseURL + ">")
	}
	return b.String()
}

// NoticeOf reads the notice of one font program.
func NoticeOf(f Face) Notice {
	n := Notice{Family: f.Family}
	d := f.TTF
	off, length, ok := table(d, "name")
	if !ok {
		return n
	}
	tab := d[off : off+length]
	if len(tab) < 6 {
		return n
	}
	count := int(be16(tab[2:4]))
	strOff := int(be16(tab[4:6]))
	if strOff > len(tab) {
		return n
	}
	for i := range count {
		rec := 6 + i*12
		if rec+12 > len(tab) {
			break
		}
		pid := int(be16(tab[rec : rec+2]))
		nid := int(be16(tab[rec+6 : rec+8]))
		ln := int(be16(tab[rec+8 : rec+10]))
		so := int(be16(tab[rec+10 : rec+12]))
		if nid != 0 && nid != 13 && nid != 14 {
			continue
		}
		start := strOff + so
		if start < 0 || start+ln > len(tab) {
			continue
		}
		raw := tab[start : start+ln]
		s := decodeName(pid, raw)
		if s == "" {
			continue
		}
		switch nid {
		case 0:
			if n.Copyright == "" {
				n.Copyright = s
			}
		case 13:
			if n.LicenseText == "" {
				n.LicenseText = s
			}
		case 14:
			if n.LicenseURL == "" {
				n.LicenseURL = s
			}
		}
	}
	return n
}

// decodeName reads a name table entry: Windows (platform 3) is UTF-16, the
// rest is a single byte per character on the platforms that matter here.
func decodeName(pid int, raw []byte) string {
	var s string
	if pid == 3 || pid == 0 {
		if len(raw)%2 == 1 {
			raw = raw[:len(raw)-1]
		}
		var b strings.Builder
		for i := 0; i+1 < len(raw); i += 2 {
			b.WriteRune(rune(be16(raw[i : i+2])))
		}
		s = b.String()
	} else {
		s = string(raw)
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.TrimSpace(s)
}

// table finds a table in an sfnt file and returns its offset and length.
func table(d []byte, tag string) (int, int, bool) {
	if len(d) < 12 {
		return 0, 0, false
	}
	n := int(be16(d[4:6]))
	for i := range n {
		rec := 12 + i*16
		if rec+16 > len(d) {
			return 0, 0, false
		}
		if string(d[rec:rec+4]) != tag {
			continue
		}
		off := int(be32(d[rec+8 : rec+12]))
		length := int(be32(d[rec+12 : rec+16]))
		if off < 0 || length < 0 || off+length > len(d) {
			return 0, 0, false
		}
		return off, length, true
	}
	return 0, 0, false
}

func be16(b []byte) uint16 { return uint16(b[0])<<8 | uint16(b[1]) }

func be32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// Notices writes one line per face, deduplicated by family: three faces of the
// same family repeat the same licence, and the file should say it once.
func Notices(families []Family) []Notice {
	var out []Notice
	seen := map[string]bool{}
	for _, fam := range families {
		for _, f := range fam.Faces {
			n := NoticeOf(f)
			if seen[n.Family] {
				continue
			}
			seen[n.Family] = true
			out = append(out, n)
		}
	}
	return out
}

// NoticeFile is the text a document carries next to its fonts.
func NoticeFile(families []Family) string {
	var b strings.Builder
	b.WriteString("Schriften in diesem Dokument\n")
	b.WriteString("==========================\n\n")
	b.WriteString("Die folgenden Schriftprogramme sind mitgeliefert, damit das Dokument\n")
	b.WriteString("überall gleich aussieht. Die Bedingungen stammen aus der Schrift selbst\n")
	b.WriteString("(Namenstabelle) und gelten für die Weitergabe dieses Dokuments.\n\n")
	for _, n := range Notices(families) {
		b.WriteString(n.String() + "\n")
		if n.LicenseText != "" {
			b.WriteString("\n")
			for line := range strings.SplitSeq(n.LicenseText, "\n") {
				b.WriteString("  " + strings.TrimRight(line, " ") + "\n")
			}
		}
		b.WriteString("\n")
	}
	if len(b.String()) < 100 {
		b.WriteString(fmt.Sprintf("%d Familien ohne Angaben in der Schrift.\n", len(families)))
	}
	return b.String()
}
