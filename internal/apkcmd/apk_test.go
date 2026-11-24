package apkcmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"akmesh/internal/apkcore"
)

// The fixtures build a real repository on disk: two gzip streams per package
// and a signed-shape index. Nothing here is mocked, so a wrong format shows up
// as a failing install instead of a passing test.

func gzTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type pkgSpec struct {
	name, version string
	depends       []string
	files         map[string]string
}

// writeRepo writes <dir>/<arch>/ with the packages and an index that carries
// the real Q1 checksums, which is what makes the add path realistic.
func writeRepo(t *testing.T, dir string, specs []pkgSpec) {
	t.Helper()
	archDir := filepath.Join(dir, "x86_64")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var index strings.Builder
	var totalData int64
	for _, s := range specs {
		data := gzTar(t, s.files)
		dataSum := sha256.Sum256(data)
		control := gzTar(t, map[string]string{
			".PKGINFO": fmt.Sprintf("pkgname = %s\npkgver = %s\npkgdesc = test %s\nsize = %d\narch = x86_64\ndatahash = %x\n",
				s.name, s.version, s.name, len(data), dataSum),
		})
		apk := append(append([]byte{}, control...), data...)
		sum := sha1.Sum(control)
		hash := "Q1" + base64.StdEncoding.EncodeToString(sum[:])
		fileName := fmt.Sprintf("%s-%s.apk", s.name, s.version)
		if err := os.WriteFile(filepath.Join(archDir, fileName), apk, 0o644); err != nil {
			t.Fatal(err)
		}
		var installed int64
		for _, body := range s.files {
			installed += int64(len(body))
		}
		totalData += installed
		fmt.Fprintf(&index, "C:%s\nP:%s\nV:%s\nA:x86_64\nS:%d\nI:%d\nT:test %s\n",
			hash, s.name, s.version, len(apk), installed, s.name)
		if len(s.depends) > 0 {
			fmt.Fprintf(&index, "D:%s\n", strings.Join(s.depends, " "))
		}
		index.WriteString("\n")
	}
	idx := gzTar(t, map[string]string{"APKINDEX": index.String()})
	if err := os.WriteFile(filepath.Join(archDir, "APKINDEX.tar.gz"), idx, 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRoot prepares an empty target tree.
func newRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"etc/apk", "lib/apk/db"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runApk(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	full := append([]string{"--root", root}, args...)
	code := Run("apk", full, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

// TestAddInstallsFilesAndRecords: the plain case, with an unsigned index that
// the caller has to allow explicitly.
func TestAddInstallsFilesAndRecords(t *testing.T) {
	repo := t.TempDir()
	writeRepo(t, repo, []pkgSpec{{name: "hello", version: "0.0.1-r0", files: map[string]string{"usr/bin/hello": "hallo\n"}}})
	root := newRoot(t)

	code, _, errText := runApk(t, root, "-X", repo, "add", "hello")
	if code == 0 || !strings.Contains(errText, "UNTRUSTED signature") {
		t.Fatalf("unsigned index was accepted: %d %q", code, errText)
	}
	code, out, errText := runApk(t, root, "-X", repo, "--allow-untrusted", "add", "hello")
	if code != 0 {
		t.Fatalf("add failed: %d %q", code, errText)
	}
	if !strings.Contains(out, "(1/1) Installing hello (0.0.1-r0)") {
		t.Fatalf("progress line wrong: %q", out)
	}
	if !strings.Contains(out, "OK: ") {
		t.Fatalf("summary missing: %q", out)
	}
	body, err := os.ReadFile(filepath.Join(root, "usr/bin/hello"))
	if err != nil || string(body) != "hallo\n" {
		t.Fatalf("file not installed: %v %q", err, body)
	}
	world, _ := os.ReadFile(filepath.Join(root, "etc/apk/world"))
	if strings.TrimSpace(string(world)) != "hello" {
		t.Fatalf("world wrong: %q", world)
	}
	db, _ := os.ReadFile(filepath.Join(root, "lib/apk/db/installed"))
	for _, want := range []string{"P:hello", "V:0.0.1-r0", "R:hello", "a:0:0:755"} {
		if !strings.Contains(string(db), want) {
			t.Fatalf("installed db misses %q:\n%s", want, db)
		}
	}
}

// TestAddPullsDependencies: b before a, and both end up in the database.
func TestAddPullsDependencies(t *testing.T) {
	repo := t.TempDir()
	writeRepo(t, repo, []pkgSpec{
		{name: "libbase", version: "1.0-r0", files: map[string]string{"usr/lib/libbase.so": "x"}},
		{name: "tool", version: "2.0-r0", depends: []string{"libbase"}, files: map[string]string{"usr/bin/tool": "y"}},
	})
	root := newRoot(t)
	code, out, errText := runApk(t, root, "-X", repo, "--allow-untrusted", "add", "tool")
	if code != 0 {
		t.Fatalf("add with dependency failed: %d %q", code, errText)
	}
	if !strings.Contains(out, "(1/2) Installing libbase (1.0-r0)") || !strings.Contains(out, "(2/2) Installing tool (2.0-r0)") {
		t.Fatalf("dependency order wrong:\n%s", out)
	}
	code, out, _ = runApk(t, root, "info")
	if code != 0 || !strings.Contains(out, "libbase") || !strings.Contains(out, "tool") {
		t.Fatalf("info: %d %q", code, out)
	}
	world, _ := os.ReadFile(filepath.Join(root, "etc/apk/world"))
	if strings.TrimSpace(string(world)) != "tool" {
		t.Fatalf("only the requested name belongs in world: %q", world)
	}
}

// TestDelRemovesFilesAndRefusesNeededOnes.
func TestDelRemovesFilesAndRefusesNeededOnes(t *testing.T) {
	repo := t.TempDir()
	writeRepo(t, repo, []pkgSpec{
		{name: "libbase", version: "1.0-r0", files: map[string]string{"usr/lib/libbase.so": "x"}},
		{name: "tool", version: "2.0-r0", depends: []string{"libbase"}, files: map[string]string{"usr/bin/tool": "y"}},
	})
	root := newRoot(t)
	runApk(t, root, "-X", repo, "--allow-untrusted", "add", "tool")

	code, _, errText := runApk(t, root, "del", "libbase")
	if code == 0 || !strings.Contains(errText, "libbase is required by tool") {
		t.Fatalf("dependency not protected: %d %q", code, errText)
	}
	code, out, errText := runApk(t, root, "del", "tool")
	if code != 0 {
		t.Fatalf("del failed: %d %q", code, errText)
	}
	if !strings.Contains(out, "Purging tool (2.0-r0)") {
		t.Fatalf("purging line wrong: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "usr/bin/tool")); !os.IsNotExist(err) {
		t.Fatal("file survived the purging")
	}
	world, _ := os.ReadFile(filepath.Join(root, "etc/apk/world"))
	if strings.TrimSpace(string(world)) != "" {
		t.Fatalf("world not cleared: %q", world)
	}
	code, out, _ = runApk(t, root, "info")
	if code != 0 || strings.TrimSpace(out) != "libbase" {
		t.Fatalf("info after del: %q", out)
	}
}

// TestChecksumMismatchIsRefused: a corrupted package must not install.
func TestChecksumMismatchIsRefused(t *testing.T) {
	repo := t.TempDir()
	writeRepo(t, repo, []pkgSpec{{name: "hello", version: "1.0-r0", files: map[string]string{"usr/bin/hello": "x"}}})
	path := filepath.Join(repo, "x86_64", "hello-1.0-r0.apk")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Flip a byte in the data segment: the control hash stays valid, the
	// datahash in .PKGINFO must catch it.
	data[len(data)-5] ^= 0xff
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	root := newRoot(t)
	code, _, errText := runApk(t, root, "-X", repo, "--allow-untrusted", "add", "hello")
	if code == 0 || !strings.Contains(errText, "data mismatch") {
		t.Fatalf("corrupted package was accepted: %d %q", code, errText)
	}
}

// TestVersionCompare covers the forms a repository of ours will carry.
func TestVersionCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0-r0", "1.0-r0", 0},
		{"1.0-r1", "1.0-r0", 1},
		{"1.10", "1.9", 1},
		{"2.0", "2.0.1", -1},
		{"1.0_alpha", "1.0", -1},
		{"0.9", "1.0", -1},
	}
	for _, c := range cases {
		got := apkcore.CompareVersions(c.a, c.b)
		if got != c.want {
			t.Errorf("compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
