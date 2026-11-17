package tools

import (
	"bytes"
	"strings"
	"testing"
)

// TestSingleByteRoundTrip is the property that matters for a converter: every
// byte the source code page defines must survive a trip to UTF-8 and back.
func TestSingleByteRoundTrip(t *testing.T) {
	for _, name := range []string{"ISO-8859-1", "ISO-8859-15", "WINDOWS-1252", "ASCII"} {
		cs, err := lookupCharset(name)
		if err != nil {
			t.Fatalf("lookup %s: %v", name, err)
		}
		checked := 0
		for b := 0; b < 256; b++ {
			in := []byte{byte(b)}
			text, err := cs.decode(in)
			if err != nil {
				continue // undefined byte in this code page
			}
			out, err := cs.encode(text)
			if err != nil {
				t.Fatalf("%s: byte %#x decoded to %q but cannot be encoded back: %v", name, b, text, err)
			}
			if !bytes.Equal(in, out) {
				t.Fatalf("%s: byte %#x came back as %#x", name, b, out)
			}
			checked++
		}
		if checked < 128 {
			t.Fatalf("%s: only %d bytes round tripped, the table looks broken", name, checked)
		}
	}
}

// TestRussianDollUmlaut pins the case that motivates the whole command: a
// German text from a Windows or Latin-1 source.
func TestUmlautFromLatin1(t *testing.T) {
	src, _ := lookupCharset("ISO-8859-1")
	dst, _ := lookupCharset("UTF-8")
	text, err := src.decode([]byte{0x47, 0x72, 0xFC, 0xDF, 0x65}) // "Grüße"
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if text != "Grüße" {
		t.Fatalf("decode gave %q", text)
	}
	out, _ := dst.encode(text)
	if string(out) != "Grüße" {
		t.Fatalf("encode gave %q", out)
	}
}

// TestWindows1252Euro covers the byte that differs between Latin-1 and CP1252.
func TestWindows1252Euro(t *testing.T) {
	cs, _ := lookupCharset("CP1252")
	text, err := cs.decode([]byte{0x80})
	if err != nil || text != "€" {
		t.Fatalf("0x80 in CP1252 = %q, %v", text, err)
	}
	if _, err := cs.encode("€"); err != nil {
		t.Fatalf("euro cannot be encoded: %v", err)
	}
	// The same byte in Latin-1 is a control character, not the euro sign.
	latin1, _ := lookupCharset("LATIN1")
	if text, _ := latin1.decode([]byte{0x80}); text == "€" {
		t.Fatalf("Latin-1 must not map 0x80 to the euro sign")
	}
}

// TestUTF16 covers both byte orders and the byte order mark.
func TestUTF16(t *testing.T) {
	text := "Grüße"
	le, _ := lookupCharset("UTF-16LE")
	be, _ := lookupCharset("UTF-16BE")
	auto, _ := lookupCharset("UTF-16")

	encodedLE, _ := le.encode(text)
	encodedBE, _ := be.encode(text)
	if bytes.Equal(encodedLE, encodedBE) {
		t.Fatalf("both byte orders produced the same bytes")
	}
	if got, err := le.decode(encodedLE); err != nil || got != text {
		t.Fatalf("LE round trip: %q, %v", got, err)
	}
	if got, err := be.decode(encodedBE); err != nil || got != text {
		t.Fatalf("BE round trip: %q, %v", got, err)
	}

	// BOM handling: a BOM overrides the assumed byte order.
	withBOM := append([]byte{0xFF, 0xFE}, encodedLE...)
	if got, err := auto.decode(withBOM); err != nil || got != text {
		t.Fatalf("auto with LE BOM: %q, %v", got, err)
	}
	if got, err := auto.decode(append([]byte{0xFE, 0xFF}, encodedBE...)); err != nil || got != text {
		t.Fatalf("auto with BE BOM: %q, %v", got, err)
	}
	if got, err := auto.decode(encodedBE); err != nil || got != text {
		t.Fatalf("auto without BOM should assume BE: %q, %v", got, err)
	}
	// An odd byte count cannot be UTF-16.
	if _, err := le.decode([]byte{0x41}); err == nil {
		t.Fatalf("odd length must fail")
	}
}

// TestUnrepresentable checks that a character the target cannot hold is an
// error, and that -c drops it instead.
func TestUnrepresentable(t *testing.T) {
	ascii, _ := lookupCharset("ASCII")
	if _, err := ascii.encode("Grüße"); err == nil {
		t.Fatalf("umlauts must not fit into ASCII")
	}
	dropped := dropUnrepresentable("Grüße", ascii)
	if dropped != "Gre" {
		t.Fatalf("dropUnrepresentable gave %q, want %q", dropped, "Gre")
	}
}

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

// TestEnvDedupeKeepsLast checks the rule that the last assignment wins, which
// is what makes "env A=1 A=2 cmd" print 2.
func TestEnvDedupeKeepsLast(t *testing.T) {
	env := []string{"A=1", "B=2", "A=3"}
	got := dedupeEnv(env, "A")
	want := []string{"B=2", "A=3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("dedupeEnv = %v, want %v", got, want)
	}
}
