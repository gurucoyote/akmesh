package file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDescribeBytes pins the signatures an agent meets most often.
func TestDescribeBytes(t *testing.T) {
	cases := []struct {
		head []byte
		want string
	}{
		{[]byte{0x7F, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 62, 0}, "ELF 64-bit LSB executable, x86-64"},
		{[]byte("%PDF-1.7\n"), "PDF document"},
		{[]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A}, "PNG image data"},
		{[]byte("GIF89a..."), "GIF image data, version 89a"},
		{[]byte{0x1F, 0x8B, 0x08}, "gzip compressed data"},
		{[]byte("PK\x03\x04zip"), "Zip archive data"},
		{[]byte("SQLite format 3\x00"), "SQLite 3.x database"},
		{[]byte("hallo welt\n"), "ASCII text"},
		{[]byte("Grüße\n"), "UTF-8 Unicode text"},
		{[]byte("a\r\nb\r\n"), "ASCII text, with CRLF line terminators"},
		{[]byte{0xEF, 0xBB, 0xBF, 'h', 'i'}, "UTF-8 Unicode (with BOM) text"},
		{[]byte("Gr\xfc\xdfe\n"), "ISO-8859 text, non-UTF-8"},
		{[]byte{0x00, 0x01, 0x02, 0x03}, "data"},
	}
	for _, c := range cases {
		got, _, err := describeBytes(c.head)
		if err != nil {
			t.Fatalf("describeBytes(%q): %v", c.head, err)
		}
		if got != c.want {
			t.Fatalf("describeBytes(%q) = %q, want %q", c.head, got, c.want)
		}
	}
}

// TestDescribePESignature checks the Windows executable branch without needing
// a real PE file.
func TestDescribePESignature(t *testing.T) {
	head := make([]byte, 0x100)
	copy(head, "MZ")
	head[0x3C] = 0x40 // e_lfanew
	copy(head[0x40:], "PE\x00\x00")
	head[0x44] = 0x64
	head[0x45] = 0x86 // x86-64
	head[0x58] = 0x0B
	head[0x59] = 0x02 // 0x20B, PE32+
	got, _, err := describeBytes(head)
	if err != nil {
		t.Fatalf("describeBytes: %v", err)
	}
	if !strings.Contains(got, "PE32+") || !strings.Contains(got, "x86-64") {
		t.Fatalf("PE type came out as %q", got)
	}
}

// TestRunTellsTextFromBinary is the part an agent relies on before opening a
// file: is this text or bytes?
func TestRunTellsTextFromBinary(t *testing.T) {
	dir := t.TempDir()
	text := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(text, []byte("hallo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "b.bin")
	if err := os.WriteFile(bin, []byte{0x00, 0x01, 0x02, 0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	got := describe(t, text)
	if !strings.Contains(got, "text") && !strings.Contains(got, "ASCII") {
		t.Errorf("text file described as %q", got)
	}
	got = describe(t, bin)
	if !strings.Contains(got, "data") {
		t.Errorf("binary file described as %q", got)
	}
}

// TestRunMissingFileReportsIt: a missing path is an error message and a nonzero
// exit, not an empty answer.
func TestRunMissingFileReportsIt(t *testing.T) {
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errFile, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errFile.Close()
	code := Run([]string{"no-such-file-xyz"}, nil, out, errFile)
	if code == 0 {
		t.Fatalf("exit: want nonzero for a missing file, got 0")
	}
	msg, _ := os.ReadFile(errFile.Name())
	if len(msg) == 0 {
		t.Errorf("no message on stderr")
	}
}

// describe runs file on one path and returns its output as a plain string.
func describe(t *testing.T, path string) string {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errFile, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errFile.Close()
	if code := Run([]string{path}, nil, out, errFile); code != 0 {
		t.Fatalf("file %s: exit %d", path, code)
	}
	data, _ := os.ReadFile(out.Name())
	return string(data)
}
