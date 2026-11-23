package grep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBinaryFileMessage: ohne -a meldet grep "binary file matches" auf stderr
// und gibt keine Zeilen aus, mit -a die Rohzeile; -I zaehlt die Datei als
// Nicht-Treffer. Werte gegen GNU grep 3.11 gemessen (acmesh-bde).
func TestBinaryFileMessage(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin1")
	if err := os.WriteFile(bin, []byte("X\x00ABC\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, string, int) {
		var out, errb strings.Builder
		code := Run(args, os.Stdin, &nopFile{&out}, &nopFile{&errb})
		return out.String(), errb.String(), code
	}
	out, errOut, code := run("ABC", bin)
	if out != "" || !strings.Contains(errOut, "binary file matches") || code != 0 {
		t.Fatalf("plain: out %q, err %q, code %d", out, errOut, code)
	}
	if out, _, code := run("-a", "ABC", bin); out != "X\x00ABC\n" || code != 0 {
		t.Fatalf("-a: out %q, code %d", out, code)
	}
	if out, errOut, code := run("-I", "ABC", bin); out != "" || errOut != "" || code != 1 {
		t.Fatalf("-I: out %q, err %q, code %d", out, errOut, code)
	}
	if out, _, code := run("-c", "ABC", bin); out != "1\n" || code != 0 {
		t.Fatalf("-c: out %q, code %d", out, code)
	}
	if out, _, code := run("--binary-files=text", "ABC", bin); out != "X\x00ABC\n" || code != 0 {
		t.Fatalf("--binary-files=text: out %q, code %d", out, code)
	}
	if out, _, code := run("ZZZ", bin); out != "" || code != 1 {
		t.Fatalf("no match: out %q, code %d", out, code)
	}
}

// TestFilesWithoutMatch: -L nennt die Dateien ohne Treffer und endet mit 0,
// auch wenn es nichts zu nennen gibt (GNU-Messung 23.11.2026).
func TestFilesWithoutMatch(t *testing.T) {
	dir := t.TempDir()
	hit := filepath.Join(dir, "hit")
	miss := filepath.Join(dir, "miss")
	os.WriteFile(hit, []byte("needle\n"), 0o644)
	os.WriteFile(miss, []byte("hay\n"), 0o644)
	var out, errb strings.Builder
	if code := Run([]string{"-L", "needle", hit, miss}, os.Stdin, &nopFile{&out}, &nopFile{&errb}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := out.String(); got != miss+"\n" {
		t.Fatalf("out %q", got)
	}
	var o2, e2 strings.Builder
	if code := Run([]string{"-L", "needle", hit}, os.Stdin, &nopFile{&o2}, &nopFile{&e2}); code != 0 || o2.String() != "" {
		t.Fatalf("all match: out %q, code %d", o2.String(), code)
	}
}

// nopFile makes a strings.Builder usable as an os.File for Run.
type nopFile struct{ b *strings.Builder }

func (n *nopFile) Write(p []byte) (int, error) { return n.b.Write(p) }
