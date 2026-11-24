package apkcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"akmesh/internal/apkcore"
)

// TestRealAbuildPackage is the regression the unit fixtures cannot give: the
// files in testdata/real come from abuild 3.15 in alpine:3.22, so they have
// three gzip members and the hashes apk really writes. A gzip read-ahead bug
// once made every hash point at the wrong bytes and only this file showed it.
func TestRealAbuildPackage(t *testing.T) {
	dir := "testdata/real"
	keys, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var trusted []*apkcore.Key
	for _, e := range keys {
		if !strings.HasSuffix(e.Name(), ".pub") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		k, err := apkcore.LoadKey(e.Name(), data)
		if err != nil {
			t.Fatalf("loading %s: %v", e.Name(), err)
		}
		trusted = append(trusted, k)
	}
	if len(trusted) == 0 {
		t.Fatal("no key in testdata")
	}
	indexData, err := os.ReadFile(filepath.Join(dir, "APKINDEX.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	idx, err := apkcore.ParseIndex(indexData, dir, trusted)
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if idx.Unsigned || len(idx.Keys) == 0 {
		t.Fatalf("signature not verified: %+v", idx.Keys)
	}
	if len(idx.Packages) != 1 || idx.Packages[0].Name != "akmesh-hello" {
		t.Fatalf("index content: %+v", idx.Packages)
	}
	apkData, err := os.ReadFile(filepath.Join(dir, "akmesh-hello-0.0.1-r0.apk"))
	if err != nil {
		t.Fatal(err)
	}
	if err := apkcore.VerifyPackage(apkData, idx.Packages[0], trusted...); err != nil {
		t.Fatalf("package check: %v", err)
	}
	root := t.TempDir()
	files, _, err := apkcore.Extract(apkData, idx.Packages[0], apkcore.ExtractOptions{Root: root})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "usr/bin/akmesh-hello"))
	if err != nil || !strings.Contains(string(body), "hallo aus dem eigenen paket") {
		t.Fatalf("installed file: %v %q", err, body)
	}
	if len(files) == 0 {
		t.Fatal("no file records")
	}
}
