package apkcore

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteInstalledKeepsOneEntryPerName is the regression for 24.11.2026: a
// downgrade appended the new entry and left the old one, so the database held
// three P: lines for one package and every report counted it three times.
func TestWriteInstalledKeepsOneEntryPerName(t *testing.T) {
	root := t.TempDir()
	pkgs := []*Installed{
		{Package: Package{Name: "rotest", Version: "1.0.0-r0"}},
		{Package: Package{Name: "anderes", Version: "1.0.0-r0"}},
		{Package: Package{Name: "rotest", Version: "2.0.0-r0"}},
	}
	if err := WriteInstalled(root, pkgs); err != nil {
		t.Fatal(err)
	}
	got, err := ReadInstalled(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%d Eintraege, erwartet 2: %+v", len(got), got)
	}
	if got[0].Name != "rotest" || got[0].Version != "2.0.0-r0" {
		t.Fatalf("der letzte Stand muss gewinnen: %+v", got[0])
	}
	body, _ := os.ReadFile(filepath.Join(root, "lib/apk/db/installed"))
	if n := len(splitLines(body, "P:rotest")); n != 1 {
		t.Fatalf("P:rotest kommt %d mal vor", n)
	}
}

func splitLines(b []byte, want string) []string {
	var out []string
	for _, l := range splitBytes(b) {
		if string(l) == want {
			out = append(out, want)
		}
	}
	return out
}

func splitBytes(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			out = append(out, b[start:i])
			start = i + 1
		}
	}
	return out
}
