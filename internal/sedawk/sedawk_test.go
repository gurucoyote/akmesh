package sedawk

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runHere executes one of the two commands with stdin, returning stdout, stderr
// and the exit code.
func runHere(t *testing.T, cmd string, args []string, input string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in")
	if err := os.WriteFile(in, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	stdin, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
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
	code := Run(cmd, args, stdin, out, errFile)
	o, _ := os.ReadFile(out.Name())
	e, _ := os.ReadFile(errFile.Name())
	return string(o), string(e), code
}

// TestSedSubstitute is the everyday form: s/alt/neu/ on every line.
func TestSedSubstitute(t *testing.T) {
	got, _, code := runHere(t, "sed", []string{"s/zwei/ZWEI/"}, "eins\nzwei\ndrei\n")
	if code != 0 {
		t.Fatalf("exit: want 0, got %d", code)
	}
	if got != "eins\nZWEI\ndrei\n" {
		t.Errorf("got %q", got)
	}
}

// TestSedPrintRange checks the -n form an agent uses to look at a band of lines.
func TestSedPrintRange(t *testing.T) {
	got, _, code := runHere(t, "sed", []string{"-n", "2,3p"}, "eins\nzwei\ndrei\nvier\n")
	if code != 0 {
		t.Fatalf("exit: want 0, got %d", code)
	}
	if got != "zwei\ndrei\n" {
		t.Errorf("got %q", got)
	}
}

// TestSedInPlaceEditsFile pins -i, which the shell tests never reach.
func TestSedInPlaceEditsFile(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(name, []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, errText, code := runHere(t, "sed", []string{"-i", "s/a/A/", name}, "")
	if code != 0 {
		t.Fatalf("exit: want 0, got %d (%s)", code, errText)
	}
	got, _ := os.ReadFile(name)
	if string(got) != "A\nb\n" {
		t.Errorf("file after -i: %q", string(got))
	}
}

// TestAwkFieldsAndSum is the two things awk gets used for most: one field of a
// line and a sum over lines.
func TestAwkFieldsAndSum(t *testing.T) {
	got, _, code := runHere(t, "awk", []string{"{print $2}"}, "a 1\nb 2\n")
	if code != 0 {
		t.Fatalf("exit: want 0, got %d", code)
	}
	if got != "1\n2\n" {
		t.Errorf("field: got %q", got)
	}
	got, _, code = runHere(t, "awk", []string{"{s+=$1} END {print s}"}, "1\n2\n3\n")
	if code != 0 {
		t.Fatalf("exit: want 0, got %d", code)
	}
	if strings.TrimSpace(got) != "6" {
		t.Errorf("sum: got %q", got)
	}
}

// TestAwkProgramErrorExitsNonzero matters for scripts: a typo must not look
// like success.
func TestAwkProgramErrorExitsNonzero(t *testing.T) {
	_, errText, code := runHere(t, "awk", []string{"{print $"}, "x\n")
	if code == 0 {
		t.Fatalf("exit: want nonzero for a broken program, got 0")
	}
	if errText == "" {
		t.Errorf("no message on stderr")
	}
	_ = io.Discard
}
