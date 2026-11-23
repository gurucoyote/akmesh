package wc

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMaxLineIsTheLongestLine: -L nimmt die laengste Zeile, Tabulatoren
// ruecken auf das naechste Vielfache von 8.
func TestMaxLineIsTheLongestLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\nbb\nccc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tab.txt"), []byte("x\ty\nab\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "noeol.txt"), []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ file, want string }{
		{"a.txt", "3 "},
		{"tab.txt", "9 "},
		{"noeol.txt", "6 "},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		if code := Run([]string{"-L", filepath.Join(dir, c.file)}, strings.NewReader(""), &out, &errb); code != 0 {
			t.Fatalf("%s: exit %d (%s)", c.file, code, errb.String())
		}
		if !strings.HasPrefix(out.String(), c.want) || !strings.HasSuffix(out.String(), c.file+"\n") {
			t.Fatalf("%s: -L = %q, want the leading %q and the name", c.file, out.String(), c.want)
		}
	}
}

// TestColumnsArePaddedLikeGNU: eine Breite fuer alle Zahlenspalten, genommen
// aus dem groessten Wert aller Zaehler einschliesslich der Summenzeile.
// Gemessen: "wc -l a.txt z5k.txt" richtet die Zeilenspalte an 4 aus, weil
// z5k.txt 5000 Bytes hat.
func TestColumnsArePaddedLikeGNU(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\nbb\nccc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("z", 5000)
	if err := os.WriteFile(filepath.Join(dir, "z5k.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := Run([]string{"-l", filepath.Join(dir, "a.txt"), filepath.Join(dir, "z5k.txt")}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("exit %d (%s)", code, errb.String())
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want three rows, got %q", out.String())
	}
	if !strings.HasPrefix(lines[0], "   3 ") {
		t.Fatalf("first row is not padded to four: %q", lines[0])
	}
	if !strings.HasSuffix(lines[2], " total") || !strings.HasPrefix(lines[2], "   3 ") {
		t.Fatalf("total row = %q", lines[2])
	}
	// Eine einzelne Datei wird nicht aufgeblaeht.
	out.Reset()
	if code := Run([]string{"-l", filepath.Join(dir, "a.txt")}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("single: exit %d", code)
	}
	if !strings.HasPrefix(out.String(), "3 ") {
		t.Fatalf("single file got padded: %q", out.String())
	}
}

// TestMaxLineFromStdin: ohne Dateinamen zaehlt -L den Strom.
func TestMaxLineFromStdin(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"-L"}, strings.NewReader("no newline"), &out, &errb); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out.String() != "10\n" {
		t.Fatalf("-L on stdin = %q, want 10", out.String())
	}
}
