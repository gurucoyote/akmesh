package webfonts

import (
	"strings"
	"testing"
)

// TestNoticesComeFromTheFonts: the terms are read out of the programs, not
// written by us. Lora says OFL, Go Mono says its own licence.
func TestNoticesComeFromTheFonts(t *testing.T) {
	notes := Notices(Book())
	if len(notes) != 2 {
		t.Fatalf("notices: %d, want one per family", len(notes))
	}
	lora := notes[0]
	if lora.Family != SansName {
		t.Fatalf("first notice is %q", lora.Family)
	}
	if !strings.Contains(lora.Copyright, "Lora") {
		t.Errorf("copyright: %q", lora.Copyright)
	}
	if got := lora.LicenseName(); !strings.Contains(got, "Open Font License") {
		t.Errorf("licence name: %q", got)
	}
	if lora.LicenseURL == "" || !strings.Contains(lora.LicenseURL, "sil.org") {
		t.Errorf("licence url: %q", lora.LicenseURL)
	}
	if notes[1].Family != MonoName {
		t.Errorf("second notice is %q", notes[1].Family)
	}
	if notes[1].Copyright == "" {
		t.Errorf("Go Mono has no copyright line")
	}
}

// TestNoticeFileIsReadable: the file names every family with its terms, and
// repeats a family only once.
func TestNoticeFileIsReadable(t *testing.T) {
	f := NoticeFile(Book())
	for _, want := range []string{"Schriften in diesem Dokument", SansName, MonoName, "Lizenz:"} {
		if !strings.Contains(f, want) {
			t.Errorf("notice file lacks %q", want)
		}
	}
	if n := strings.Count(f, "Lizenz:"); n != 2 {
		t.Errorf("licence lines: %d, want 2 (one per family)", n)
	}
}

// TestNoticeWithoutNames: a program whose name table cannot be read must not
// panic and must not invent terms.
func TestNoticeWithoutNames(t *testing.T) {
	n := NoticeOf(Face{Family: "Kaputt", TTF: []byte("kein font")})
	if n.Family != "Kaputt" || n.Copyright != "" || n.LicenseText != "" {
		t.Errorf("notice of junk: %+v", n)
	}
	if got := n.LicenseName(); got != "unbekannt" {
		t.Errorf("licence name of junk: %q", got)
	}
}
