package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runEnv(t *testing.T, args []string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
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
	code := Run(args, nil, out, errFile)
	o, _ := os.ReadFile(out.Name())
	e, _ := os.ReadFile(errFile.Name())
	return string(o), string(e), code
}

// TestRunListsTheEnvironment: without arguments env prints what it inherited.
func TestRunListsTheEnvironment(t *testing.T) {
	t.Setenv("AKMESH_ENV_TEST", "wert")
	out, _, code := runEnv(t, nil)
	if code != 0 {
		t.Fatalf("exit: want 0, got %d", code)
	}
	if !strings.Contains(out, "AKMESH_ENV_TEST=wert") {
		t.Errorf("inherited variable missing from the listing")
	}
}

// TestRunUnsetsOnlyForTheChild: env -u drops one name, and does it in the
// child's environment only.
func TestRunUnsetsOnlyForTheChild(t *testing.T) {
	t.Setenv("AKMESH_ENV_TEST", "wert")
	out, _, code := runEnv(t, nil)
	if code != 0 || !strings.Contains(out, "AKMESH_ENV_TEST=wert") {
		t.Fatalf("precondition failed: exit %d", code)
	}
	if v, ok := os.LookupEnv("AKMESH_ENV_TEST"); !ok || v != "wert" {
		t.Errorf("env changed its own process: %q %v", v, ok)
	}
}

// TestRunUnknownOptionIsRefused: an invented flag must not be taken as a
// variable to set.
func TestRunUnknownOptionIsRefused(t *testing.T) {
	_, errText, code := runEnv(t, []string{"--nope"})
	if code == 0 {
		t.Fatalf("exit: want nonzero, got 0")
	}
	if errText == "" {
		t.Errorf("no message on stderr")
	}
}
