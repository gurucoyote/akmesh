package sedawk

import (
	"os"
	"path/filepath"
	"testing"
)

// runSedText runs sed with args, stdin and returns stdout.
func runSedText(t *testing.T, stdin string, args ...string) (string, int) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in")
	if err := os.WriteFile(in, []byte(stdin), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	of, err := os.CreateTemp(dir, "out")
	if err != nil {
		t.Fatal(err)
	}
	defer of.Close()
	code := runSed(args, f, of, os.Stderr)
	if _, err := of.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(of.Name())
	return string(data), code
}

// TestZeroTerminatedRecords: -z liest NUL-getrennte Saetze, wendet das Skript
// auf jeden an uns schreibt sie wieder NUL-getrennt - die Form, mit der ein
// Agent "find -print0" weiterverarbeitet (acmesh-d9e).
func TestZeroTerminatedRecords(t *testing.T) {
	in := "a.txt\x00b c.txt\x00d.txt\x00"
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"start of every record", []string{"-z", "s/^/X/"}, "Xa.txt\x00Xb c.txt\x00Xd.txt\x00"},
		{"all matches", []string{"-z", "s/txt/TXT/g"}, "a.TXT\x00b c.TXT\x00d.TXT\x00"},
		{"line number", []string{"-z", "2d"}, "a.txt\x00d.txt\x00"},
		{"last record", []string{"-z", "$s/txt/END/"}, "a.txt\x00b c.txt\x00d.END\x00"},
		{"glued flags", []string{"-zn", "p"}, in},
	}
	for _, c := range cases {
		got, code := runSedText(t, in, c.args...)
		if code != 0 {
			t.Fatalf("%s: exit %d", c.name, code)
		}
		if got != c.want {
			t.Fatalf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// TestZeroTerminatedKeepsLineBreakInRecord: ein Zeilenumbruch innerhalb eines
// NUL-Satzes bleibt Zeilenumbruch (im Umschlag codiert), das Byte 0x01 ebenso.
func TestZeroTerminatedKeepsLineBreakInRecord(t *testing.T) {
	got, code := runSedText(t, "a\nb\x00c\x00", "-z", "s/^/X/")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got != "Xa\nb\x00Xc\x00" {
		t.Fatalf("got %q", got)
	}
	got, code = runSedText(t, "a\x01b\x00", "-z", "s/^/X/")
	if code != 0 || got != "Xa\x01b\x00" {
		t.Fatalf("0x01: %q (exit %d)", got, code)
	}
}
