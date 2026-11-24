package apkbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"akmesh/internal/apkcore"
)

// tree writes a package payload and returns its directory.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func manifest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestBuildRoundTrip is the real check: the package we write must pass our own
// verifier and extract to the same bytes we put in.
func TestBuildRoundTrip(t *testing.T) {
	src := tree(t, map[string]string{
		"usr/bin/akmesh-hello": "#!/bin/sh\necho hallo\n",
		"usr/share/doc/readme": "doc\n",
	})
	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "test.rsa")
	if _, err := GenerateKey(keyPath, 2048); err != nil {
		t.Fatal(err)
	}
	man := &Manifest{
		Name: "akmesh-hello", Version: "0.0.1", Release: "0",
		Description: "hello", Arch: "x86_64", SourceDateEpoch: 1700000000,
	}
	out := t.TempDir()
	res, err := Build(man, src, out, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(res.Path, "akmesh-hello-0.0.1-r0.apk") {
		t.Fatalf("file name: %s", res.Path)
	}
	data, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	key, err := apkcore.LoadKey("test.rsa.pub", pub)
	if err != nil {
		t.Fatal(err)
	}
	if err := apkcore.VerifyPackage(data, &apkcore.Package{
		Name: "akmesh-hello", Version: "0.0.1-r0", Hash: res.ControlSHA,
	}, key); err != nil {
		t.Fatalf("our own package does not verify: %v", err)
	}
	root := t.TempDir()
	if _, err := apkcore.Extract(data, &apkcore.Package{Name: "akmesh-hello"}, apkcore.ExtractOptions{Root: root}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "usr/bin/akmesh-hello"))
	if err != nil || !strings.Contains(string(body), "echo hallo") {
		t.Fatalf("payload: %v %q", err, body)
	}
}

// TestIndexIsSigned checks the index the tool writes: our parser must find the
// signature and the C: value must match the package's control hash.
func TestIndexIsSigned(t *testing.T) {
	src := tree(t, map[string]string{"usr/bin/x": "x\n"})
	keyPath := filepath.Join(t.TempDir(), "k.rsa")
	if _, err := GenerateKey(keyPath, 2048); err != nil {
		t.Fatal(err)
	}
	man := &Manifest{Name: "x", Version: "1.0", Release: "2", Arch: "x86_64", Description: "x pkg", Depends: []string{"sh"}}
	res, err := Build(man, src, t.TempDir(), keyPath)
	if err != nil {
		t.Fatal(err)
	}
	repoDir := t.TempDir()
	indexPath := filepath.Join(repoDir, "APKINDEX.tar.gz")
	if _, err := WriteIndex([]IndexEntry{EntryFromManifest(man, res, time.Now())}, indexPath, keyPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := os.ReadFile(keyPath + ".pub")
	key, err := apkcore.LoadKey("k.rsa.pub", pub)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := apkcore.ParseIndex(data, indexPath, []*apkcore.Key{key})
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if idx.Unsigned || idx.Untrusted || len(idx.Keys) == 0 {
		t.Fatalf("index not trusted: %+v", idx.Keys)
	}
	if len(idx.Packages) != 1 {
		t.Fatalf("packages: %d", len(idx.Packages))
	}
	p := idx.Packages[0]
	if p.Name != "x" || p.Version != "1.0-r2" || p.Hash != res.ControlSHA {
		t.Fatalf("entry: %+v (want hash %s)", p, res.ControlSHA)
	}
	if len(p.Depends) != 1 || p.Depends[0] != "sh" {
		t.Fatalf("depends: %v", p.Depends)
	}
}

// TestIndexExtends checks that "index" adds a second package instead of
// replacing the first, which is what a repository needs.
func TestIndexExtends(t *testing.T) {
	indexPath := filepath.Join(t.TempDir(), "APKINDEX.tar.gz")
	for _, name := range []string{"one", "two"} {
		man := &Manifest{Name: name, Version: "1.0", Release: "0", Arch: "x86_64"}
		res, err := Build(man, tree(t, map[string]string{"usr/bin/" + name: name}), t.TempDir(), "")
		if err != nil {
			t.Fatal(err)
		}
		// first call writes, second must merge
		entries, _ := ReadIndexFile(indexPath)
		entries = append(entries, EntryFromManifest(man, res, time.Now()))
		if _, err := WriteIndex(entries, indexPath, ""); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		idx, err := apkcore.ParseIndex(data, indexPath, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if name == "two" {
			want = 2
		}
		if len(idx.Packages) != want {
			t.Fatalf("after %s: %d packages, want %d", name, len(idx.Packages), want)
		}
	}
}

// TestManifestRejectsUnknownKey keeps typos from being silently ignored: a
// misspelled "pkgdesc" would otherwise produce a package with no description.
func TestManifestRejectsUnknownKey(t *testing.T) {
	path := manifest(t, "pkgname = a\npkgver = 1\npkgdesk = typo\n")
	if _, err := ReadManifest(path); err == nil {
		t.Fatal("typo accepted")
	}
	path = manifest(t, "pkgname = a\npkgver = 1\ndepends = sh musl\n")
	m, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Depends) != 2 {
		t.Fatalf("depends: %v", m.Depends)
	}
}
