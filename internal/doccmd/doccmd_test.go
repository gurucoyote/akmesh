package doccmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// streams gives a test the same *os.File a caller would have, and lets it read
// what the dispatcher wrote.
func streams(t *testing.T) (in, out, errOut *os.File, read func() (string, string)) {
	t.Helper()
	dir := t.TempDir()
	in, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { in.Close() })
	out, err = os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	errOut, err = os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	read = func() (string, string) {
		o, _ := os.ReadFile(out.Name())
		e, _ := os.ReadFile(errOut.Name())
		return string(o), string(e)
	}
	t.Cleanup(func() { out.Close(); errOut.Close() })
	return in, out, errOut, read
}

// TestDispatchByName: a symlink called "rg" is the command, without a subcommand
// word - that is how an agent keeps typing "rg" after the split.
func TestDispatchByName(t *testing.T) {
	in, out, errOut, read := streams(t)
	code := Run([]string{"/bin/rg", "--help"}, in, out, errOut)
	if code != 0 {
		t.Fatalf("rg --help endet mit %d", code)
	}
	stdout, stderr := read()
	if !strings.Contains(stdout+stderr, "rg [-i]") {
		t.Fatalf("die Hilfe von rg fehlt: %q", stdout+stderr)
	}
}

// TestDispatchByFirstArgument: started as akmesh-docs, the first word is the
// command.
func TestDispatchByFirstArgument(t *testing.T) {
	in, out, errOut, read := streams(t)
	code := Run([]string{"/bin/akmesh-docs", "any2md", "--help"}, in, out, errOut)
	if code != 0 {
		t.Fatalf("any2md --help endet mit %d", code)
	}
	stdout, stderr := read()
	// any2md writes its help to stderr, rg to stdout: a caller reads both.
	if !strings.Contains(stdout+stderr, "any2md [-o") {
		t.Fatalf("die Hilfe von any2md fehlt: %q", stdout+stderr)
	}
}

// TestUnknownCommandNamesTheList: a command this binary does not have must be an
// error with the list, never a silent success - the same rule the single binary
// follows.
func TestUnknownCommandNamesTheList(t *testing.T) {
	in, out, errOut, read := streams(t)
	code := Run([]string{"/bin/akmesh-docs", "gibtsnicht"}, in, out, errOut)
	if code != 2 {
		t.Fatalf("unbekannter Befehl endet mit %d, erwartet 2", code)
	}
	_, stderr := read()
	if !strings.Contains(stderr, `unknown command "gibtsnicht"`) {
		t.Fatalf("die Meldung nennt den Namen nicht: %q", stderr)
	}
	if !strings.Contains(stderr, "any2md") || !strings.Contains(stderr, "rg") {
		t.Fatalf("die Meldung nennt die Befehle nicht: %q", stderr)
	}
}

// TestNoArgumentPrintsTheList: the binary alone says what it can do.
func TestNoArgumentPrintsTheList(t *testing.T) {
	in, out, errOut, read := streams(t)
	if code := Run([]string{"/bin/akmesh-docs"}, in, out, errOut); code != 2 {
		t.Fatalf("ohne Argument endet es mit %d, erwartet 2", code)
	}
	_, stderr := read()
	if !strings.Contains(stderr, "usage: akmesh-docs COMMAND") {
		t.Fatalf("keine Aufrufzeile: %q", stderr)
	}
	if !strings.Contains(stderr, "md2pdf") || !strings.Contains(stderr, "rg") {
		t.Fatalf("die Befehlsliste fehlt: %q", stderr)
	}
}

// TestEveryCommandAnswersHelp: every name in the table reaches its own help.
// The document commands carry their text in their package, not in the shared
// table, so the check is that the call answers at all - a name whose help is
// empty is a command that was never wired up.
func TestEveryCommandAnswersHelp(t *testing.T) {
	for _, name := range Names() {
		in, out, errOut, read := streams(t)
		if code := Run([]string{"/bin/akmesh-docs", name, "--help"}, in, out, errOut); code != 0 {
			t.Errorf("%s --help endet mit %d", name, code)
			continue
		}
		stdout, stderr := read()
		if strings.TrimSpace(stdout+stderr) == "" {
			t.Errorf("%s --help sagt nichts", name)
		}
	}
}

// TestTableIsTheDocumentJob pins what this binary is: the readers and writers
// for the five document formats plus rg. A name that appears here by accident
// costs a library, so the list is written down.
func TestTableIsTheDocumentJob(t *testing.T) {
	want := "any2md docx2md epub2html epub2md html2docx html2epub html2md md2docx md2epub md2pdf md2txt-l pdf2html pdf2md rg rtf2md"
	if got := strings.Join(Names(), " "); got != want {
		t.Fatalf("Tabelle ist %q,\nerwartet     %q", got, want)
	}
}
