package multicall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gurucoyote/akmesh/internal/cmdrun"
)

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

// echo is a command that writes its arguments, so a test can see which arguments
// the dispatcher handed over.
func echo(want string) cmdrun.Func {
	return func(args []string, stdin, stdout, stderr *os.File) int {
		stdout.WriteString(want + ":" + strings.Join(args, " ") + "\n")
		return 0
	}
}

func prog() Prog {
	return Prog{
		Name:     "akmesh-probe",
		Commands: map[string]cmdrun.Func{"eins": echo("eins")},
		Others:   []string{"akmesh"},
	}
}

// TestDispatchDropsTheSubcommandWord: started as akmesh-probe, the first word
// names the command and must not reach the command as an argument.
func TestDispatchDropsTheSubcommandWord(t *testing.T) {
	in, out, errOut, read := streams(t)
	if code := prog().Run([]string{"/bin/akmesh-probe", "eins", "a", "b"}, in, out, errOut); code != 0 {
		t.Fatalf("Aufruf endet mit %d", code)
	}
	stdout, _ := read()
	if stdout != "eins:a b\n" {
		t.Fatalf("die Argumente kamen falsch an: %q", stdout)
	}
}

// TestDispatchByProgramName: a symlink named like the command needs no word.
func TestDispatchByProgramName(t *testing.T) {
	in, out, errOut, read := streams(t)
	if code := prog().Run([]string{"/bin/eins", "x"}, in, out, errOut); code != 0 {
		t.Fatalf("Aufruf endet mit %d", code)
	}
	stdout, _ := read()
	if stdout != "eins:x\n" {
		t.Fatalf("die Argumente kamen falsch an: %q", stdout)
	}
}

// TestCommandSeesItsOwnName: after the dispatch, argv[0] is the command name,
// not the binary's path. A command that restarts itself (httpd -f detach) reads
// the name from there, and a dispatcher that leaves the path in place starts the
// child with no command at all.
func TestCommandSeesItsOwnName(t *testing.T) {
	defer func(old []string) { os.Args = old }(os.Args)
	in, out, errOut, _ := streams(t)
	prog := Prog{
		Name: "akmesh-probe",
		Commands: map[string]cmdrun.Func{"eins": func(args []string, stdin, stdout, stderr *os.File) int {
			if len(os.Args) == 0 || os.Args[0] != "eins" {
				stdout.WriteString("argv0=" + strings.Join(os.Args, " ") + "\n")
			}
			return 0
		}},
	}
	if code := prog.Run([]string{"/bin/akmesh-probe", "eins", "x"}, in, out, errOut); code != 0 {
		t.Fatalf("Aufruf endet mit %d", code)
	}
	if len(os.Args) != 2 || os.Args[0] != "eins" || os.Args[1] != "x" {
		t.Fatalf("os.Args ist %q, erwartet [eins x]", os.Args)
	}
}

// TestUnknownNameListsAndPointsAtTheOthers: a name this binary does not have is
// an error with the list and with the other packages, never a silent success.
func TestUnknownNameListsAndPointsAtTheOthers(t *testing.T) {
	in, out, errOut, read := streams(t)
	if code := prog().Run([]string{"/bin/akmesh-probe", "zwei"}, in, out, errOut); code != 2 {
		t.Fatalf("unbekannter Name endet mit %d, erwartet 2", code)
	}
	_, stderr := read()
	for _, want := range []string{`unknown command "zwei"`, "commands: eins", "other packages: akmesh"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("%q fehlt in %q", want, stderr)
		}
	}
}

// TestEmptyArgumentVectorUsesTheName pins the fallback: with no argv at all the
// program is still named, so the list does not start with a bare colon.
func TestEmptyArgumentVectorUsesTheName(t *testing.T) {
	in, out, errOut, read := streams(t)
	if code := prog().Run(nil, in, out, errOut); code != 2 {
		t.Fatalf("ohne Argumente endet es mit %d, erwartet 2", code)
	}
	_, stderr := read()
	if !strings.Contains(stderr, "usage: akmesh-probe COMMAND") {
		t.Fatalf("der Programmname fehlt: %q", stderr)
	}
}
