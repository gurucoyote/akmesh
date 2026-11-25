package iconv

import (
	"bytes"
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
		for b := range 256 {
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
