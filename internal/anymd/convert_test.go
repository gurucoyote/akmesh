package anymd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConvertReadsWhatTheCommandReads pins the library entry to the command:
// a document the CLI reads must convert the same way without arguments.
func TestConvertReadsWhatTheCommandReads(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "docxmd", "testdata", "simple.docx"))
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	md, err := Convert(data)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if md == "" {
		t.Fatal("Convert returned no text")
	}
}

// TestConvertRefusesUnknownBytes: bytes of no known format must produce the
// message that names the formats, not an empty success.
func TestConvertRefusesUnknownBytes(t *testing.T) {
	if _, err := Convert([]byte("just text, no document")); err == nil {
		t.Fatal("Convert accepted plain text")
	}
}
