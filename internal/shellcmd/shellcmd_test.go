package shellcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpOf runs "<name> --help" through the dispatcher and returns both streams.
func helpOf(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	in, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errOut, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errOut.Close()
	if code := Run([]string{"/bin/akmesh-shell", name, "--help"}, in, out, errOut); code != 0 {
		t.Errorf("%s --help endet mit %d", name, code)
	}
	o, _ := os.ReadFile(out.Name())
	e, _ := os.ReadFile(errOut.Name())
	return string(o) + string(e)
}

// TestTableIsTheShellJob pins what this binary is: the shell and the package
// manager that runs its scripts. Both need mvdan.cc/sh, which is why they share
// a binary.
func TestTableIsTheShellJob(t *testing.T) {
	want := "apk apkbuild apkguard apkreport gosh sh"
	if got := strings.Join(Names(), " "); got != want {
		t.Fatalf("Tabelle ist %q,\nerwartet     %q", got, want)
	}
}

// TestEveryCommandAnswersHelp: a name whose help is empty was never wired up.
func TestEveryCommandAnswersHelp(t *testing.T) {
	for _, name := range Names() {
		if strings.TrimSpace(helpOf(t, name)) == "" {
			t.Errorf("%s --help sagt nichts", name)
		}
	}
}
