package apkcmd

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// script writes a small executable shell script, which stands in for a freshly
// extracted binary: the check only needs something that starts and exits.
func script(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestInstallCheckedBinaryKeepsTheOldOneWhenTheCheckFails is the reason this
// path exists: a package that ships a binary that does not run must not take
// away the binary that can repair it.
func TestInstallCheckedBinaryKeepsTheOldOneWhenTheCheckFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("die Prüfung startet ein Shellskript; unter Windows gibt es keins")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "akmesh")
	script(t, target, "exit 0")
	deferred := filepath.Join(dir, "akmesh.apk-new")
	script(t, deferred, "exit 3")

	if err := installCheckedBinary("apk", deferred, target, io.Discard, io.Discard); err == nil {
		t.Fatal("installCheckedBinary meldet Erfolg, obwohl die neue Fassung scheitert")
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("die alte Fassung wurde angefasst: %q", body)
	}
	if _, err := os.Stat(deferred); err != nil {
		t.Fatalf("die geprüfte Datei muss zur Untersuchung liegen bleiben: %v", err)
	}
}

// TestInstallCheckedBinaryReplacesAfterAGoodCheck: a binary that runs replaces
// the old one, and the deferred name is gone afterwards.
func TestInstallCheckedBinaryReplacesAfterAGoodCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("die Prüfung startet ein Shellskript; unter Windows gibt es keins")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "akmesh")
	script(t, target, "exit 7")
	deferred := filepath.Join(dir, "akmesh.apk-new")
	script(t, deferred, "exit 0")

	if err := installCheckedBinary("apk", deferred, target, io.Discard, io.Discard); err != nil {
		t.Fatalf("installCheckedBinary: %v", err)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("die neue Fassung steht nicht an ihrem Platz: %q", body)
	}
	if _, err := os.Stat(deferred); !os.IsNotExist(err) {
		t.Fatalf("die Zwischendatei liegt noch da: %v", err)
	}
}

// TestOwnBinaryTargetPointsAtBinName: the protected path is /bin/<name> of the
// running binary, inside the root that is being installed into.
func TestOwnBinaryTargetPointsAtBinName(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("eigener Pfad nicht ermittelbar")
	}
	base := filepath.Base(exe)
	if got, want := ownBinaryTarget("/"), filepath.Join("/", "bin", base); got != want {
		t.Fatalf("ohne root ist das Ziel %q, erwartet %q", got, want)
	}
	if got, want := ownBinaryTarget("/tmp/wurzel"), filepath.Join("/tmp/wurzel", "bin", base); got != want {
		t.Fatalf("mit root ist das Ziel %q, erwartet %q", got, want)
	}
}
