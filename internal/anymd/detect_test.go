package anymd

import (
	"archive/zip"
	"bytes"
	"testing"
)

// zipOf builds an in-memory archive with the named entries, which is what a
// docx and an epub look like from the outside: both are zips, only their
// entries tell them apart.
func zipOf(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// TestDetect pins the formats any2md reads. Detection goes by the bytes, not by
// the file name: a document that was mailed without its suffix still has to be
// read, and a .txt that holds HTML is HTML.
func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want Format
	}{
		{"pdf", []byte("%PDF-1.7\n1 0 obj\n"), PDF},
		{"pdf with leading space", []byte("\n  %PDF-1.4\n"), PDF},
		{"rtf", []byte("{\\rtf1\\ansi\\deff0 hello}"), RTF},
		{"html doctype", []byte("<!DOCTYPE html>\n<html><body>x</body></html>"), HTML},
		{"html upper", []byte("<!doctype HTML><html></html>"), HTML},
		{"html fragment", []byte("<p>hello</p>"), HTML},
		{"html with leading blank", []byte("\n\n  <div>x</div>"), HTML},
		{"docx", zipOf(t, "[Content_Types].xml", "word/document.xml"), DOCX},
		{"epub", zipOf(t, "mimetype", "META-INF/container.xml"), EPUB},
	}
	for _, c := range cases {
		got, ok := Detect(c.data)
		if !ok || got != c.want {
			t.Errorf("%s: Detect = %q, %v; want %q, true", c.name, got, ok, c.want)
		}
	}
}

// TestDetectRefusesWhatItCannotRead is the other half: a zip that is neither
// Word nor a book, plain text and an empty file must be refused rather than
// guessed. A wrong guess here would send the PDF reader a PNG and print its
// error, which is a worse message than "unknown format".
func TestDetectRefusesWhatItCannotRead(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"text", []byte("hello world\n")},
		{"markdown", []byte("# title\n")},
		{"zip without the markers", zipOf(t, "readme.txt")},
		{"broken zip", []byte("PK\x03\x04 not really a zip")},
	}
	for _, c := range cases {
		if got, ok := Detect(c.data); ok {
			t.Errorf("%s: Detect = %q, true; want false", c.name, got)
		}
	}
}
