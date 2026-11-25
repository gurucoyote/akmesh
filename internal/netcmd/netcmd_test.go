package netcmd

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
	if code := Run([]string{"/bin/akmesh-net", name, "--help"}, in, out, errOut); code != 0 {
		t.Errorf("%s --help endet mit %d", name, code)
	}
	o, _ := os.ReadFile(out.Name())
	e, _ := os.ReadFile(errOut.Name())
	return string(o) + string(e)
}

// TestTableIsTheNetworkJob pins what this binary is. A name that appears here by
// accident costs a library; a name that disappears breaks a caller silently.
func TestTableIsTheNetworkJob(t *testing.T) {
	want := "certinfo curl httpd mkcert scp ssh verify wget"
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
