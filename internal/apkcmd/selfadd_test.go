package apkcmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestAddChecksTheOwnBinaryBeforeReplacingIt runs the whole path: a package that
// carries a file named like our own binary must not replace it with something
// that does not start (acmesh-ieh3).
//
// The "binary" here is a shell script, because the check only asks whether the
// file starts and exits 0; that keeps the test independent of the real build.
func TestAddChecksTheOwnBinaryBeforeReplacingIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("die Prüfung startet ein Shellskript; unter Windows gibt es keins")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Skip("eigener Pfad nicht ermittelbar")
	}
	base := filepath.Base(exe)

	setup := func(t *testing.T, newBody string) (code int, target string) {
		t.Helper()
		root := newRoot(t)
		target = filepath.Join(root, "bin", base)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n# alt\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		repo := t.TempDir()
		writeRepo(t, repo, []pkgSpec{{name: "kern", version: "2.0.0-r0", files: map[string]string{
			"bin/" + base: newBody,
		}}})
		code, _, _ = runApk(t, root, "-X", repo, "--allow-untrusted", "add", "kern")
		return code, target
	}

	t.Run("gute Fassung wird eingesetzt", func(t *testing.T) {
		code, target := setup(t, "#!/bin/sh\nexit 0\n# neu\n")
		if code != 0 {
			t.Fatalf("add mit einer lauffaehigen Fassung endet mit %d, erwartet 0", code)
		}
		body, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "# neu") {
			t.Fatalf("die neue Fassung steht nicht an ihrem Platz: %q", body)
		}
		if _, err := os.Stat(target + ".apk-new"); !os.IsNotExist(err) {
			t.Fatalf("die Zwischendatei liegt noch da: %v", err)
		}
	})

	t.Run("kaputte Fassung laesst die alte stehen", func(t *testing.T) {
		code, target := setup(t, "#!/bin/sh\nexit 3\n# kaputt\n")
		if code != 1 {
			t.Fatalf("add mit einer kaputten Fassung endet mit %d, erwartet 1", code)
		}
		body, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "# alt") {
			t.Fatalf("die alte Fassung wurde ersetzt, obwohl die neue scheitert: %q", body)
		}
		if _, err := os.Stat(target + ".apk-new"); err != nil {
			t.Fatalf("die untestete Fassung muss zur Untersuchung liegen bleiben: %v", err)
		}
	})
}
