package apkcore

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractDefersTheRunningBinary: the member that stands for our own binary
// is written beside itself as .apk-new, and the old file is untouched until the
// caller has checked the new one (acmesh-ieh3).
func TestExtractDefersTheRunningBinary(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "bin/akmesh")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("alte fassung"), 0o755); err != nil {
		t.Fatal(err)
	}
	pkg := &Package{Name: "akmesh", Version: "2.0.0-r0"}
	raw := tarGz(t, 0o755, map[string]string{"bin/akmesh": "neue fassung"})
	var deferred string
	opts := &ExtractOptions{Root: root, NoScripts: true, DeferTarget: target, Deferred: &deferred}
	if err := extractTar(raw, opts, pkg); err != nil {
		t.Fatalf("extractTar: %v", err)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "alte fassung" {
		t.Fatalf("die laufende Fassung wurde schon ersetzt: %q", body)
	}
	want := target + ".apk-new"
	if deferred != want {
		t.Fatalf("Deferred ist %q, erwartet %q", deferred, want)
	}
	newBody, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("die neue Fassung liegt nicht daneben: %v", err)
	}
	if string(newBody) != "neue fassung" {
		t.Fatalf("Inhalt der neuen Fassung ist %q", newBody)
	}
}
