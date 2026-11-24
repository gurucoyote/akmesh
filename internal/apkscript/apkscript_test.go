package apkscript

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, body string, root string) (string, string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	err := Run(Script{Kind: ".post-install", Body: []byte(body), Package: "scprog", Version: "1.0.0-r0"}, root, &out, &errb)
	return out.String(), errb.String(), err
}

// TestPathsLandInTheRoot is the core promise: a script may use "/etc/foo" and
// the file appears in the target root, not on the host.
func TestPathsLandInTheRoot(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "etc/scprog"), 0o755)
	if _, _, err := run(t, "cd /etc/scprog\necho hallo > /etc/scprog/marke.txt\necho zweite > marke2.txt\n", root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/scprog/marke2.txt")); err != nil {
		t.Fatalf("relative Datei nicht im Zielbaum: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "etc/scprog/marke.txt"))
	if err != nil {
		t.Fatalf("Datei liegt nicht im Zielbaum: %v", err)
	}
	if strings.TrimSpace(string(got)) != "hallo" {
		t.Fatalf("Inhalt: %q", got)
	}
	if _, err := os.Stat("/etc/scprog/marke.txt"); err == nil {
		t.Fatal("Skript hat auf dem Host geschrieben")
	}
}

// TestVersionIsFirstArgument: apk passes the package version as $1.
func TestVersionIsFirstArgument(t *testing.T) {
	root := t.TempDir()
	if _, _, err := run(t, "echo \"v=$1 name=$PKG_NAME\" > /v.txt\n", root); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "v.txt"))
	if strings.TrimSpace(string(got)) != "v=1.0.0-r0 name=scprog" {
		t.Fatalf("Argumente: %q", got)
	}
}

// TestEscapeIsRefused: from the root, "../../etc/passwd" would climb out, so it
// is refused; the same file by absolute path lands in the root.
func TestEscapeIsRefused(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	os.MkdirAll(filepath.Join(root, "etc"), 0o755)
	os.WriteFile(filepath.Join(root, "etc/passwd"), []byte("root:x:0:0\n"), 0o644)
	if _, _, err := run(t, "while read l; do echo \"$l\"; done < ../../etc/passwd\n", root); err == nil ||
		!strings.Contains(err.Error(), "leaves the target root") {
		t.Fatalf("Ausbruch nicht abgelehnt: %v", err)
	}
	out, _, err := run(t, "while read l; do echo \"$l\"; done < /etc/passwd\n", root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "root:x:0:0" {
		t.Fatalf("absoluter Pfad nicht im Zielbaum: %q", out)
	}
}

// TestInsideRefusesParent: the helper behind every open.
func TestInsideRefusesParent(t *testing.T) {
	root := t.TempDir()
	if got, err := Inside(root, "../../etc/passwd"); err != nil || got != filepath.Join(root, "etc/passwd") {
		t.Fatalf("Parent-Pfad entkam: %q %v", got, err)
	}
	got, err := Inside(root, "/etc/passwd")
	if err != nil || got != filepath.Join(root, "etc/passwd") {
		t.Fatalf("Inside: %q %v", got, err)
	}
}

// TestFailureIsReported: a failing script must not look like success.
func TestFailureIsReported(t *testing.T) {
	root := t.TempDir()
	if _, _, err := run(t, "echo egal\nexit 3\n", root); err == nil {
		t.Fatal("Exit 3 wurde verschluckt")
	}
}

// TestMissingExternalCommandSaysWhy: without root an external binary cannot run,
// and the message must name the way out.
func TestMissingExternalCommandSaysWhy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("als root geht der chroot-Weg, gemeint ist der Normalfall")
	}
	root := t.TempDir()
	_, errText, err := run(t, "ldconfig\n", root)
	if err == nil {
		t.Fatal("fehlendes Binary nicht gemeldet")
	}
	if !strings.Contains(errText, "--no-scripts") || !strings.Contains(errText, "needs root") {
		t.Fatalf("Hinweis fehlt: %q", errText)
	}
}
