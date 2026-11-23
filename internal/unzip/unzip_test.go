package unzip

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeZip writes a small archive with a nested file, an executable and an entry
// that tries to leave the directory (zip slip).
func makeZip(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	add := func(name, body string, mode os.FileMode) {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	add("a.txt", "inhalt-a\n", 0o644)
	add("dir/b.txt", "inhalt-b\n", 0o644)
	add("run.sh", "#!/bin/sh\n", 0o755)
	add("../evil.txt", "boese\n", 0o644)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractWritesFilesAndDirectories(t *testing.T) {
	archive := makeZip(t)
	target := t.TempDir()
	var out, errb bytes.Buffer
	if code := Run("unzip", []string{"-q", "-d", target, archive}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	data, err := os.ReadFile(filepath.Join(target, "dir/b.txt"))
	if err != nil {
		t.Fatalf("nested file not written: %v", err)
	}
	if string(data) != "inhalt-b\n" {
		t.Fatalf("nested file has %q", data)
	}
	// The executable bit survives, otherwise scripts break after unpacking.
	fi, err := os.Stat(filepath.Join(target, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Fatalf("run.sh is not executable: %v", fi.Mode())
	}
}

// TestZipSlipIsBlocked: "../evil.txt" must land inside the target, not next to
// it.
func TestZipSlipIsBlocked(t *testing.T) {
	archive := makeZip(t)
	target := filepath.Join(t.TempDir(), "ziel")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	_ = Run("unzip", []string{"-q", "-d", target, archive}, nil, &out, &errb)
	if _, err := os.Stat(filepath.Join(target, "..", "evil.txt")); err == nil {
		t.Fatal("an entry escaped the target directory")
	}
	if _, err := os.Stat(filepath.Join(target, "evil.txt")); err != nil {
		t.Fatalf("the entry was not placed inside the target: %v", err)
	}
}

// TestExistingFileNeedsOverwrite: without -o an existing file is reported and
// left alone, with -o it is replaced.
func TestExistingFileNeedsOverwrite(t *testing.T) {
	archive := makeZip(t)
	target := t.TempDir()
	old := filepath.Join(target, "a.txt")
	if err := os.WriteFile(old, []byte("alt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := Run("unzip", []string{"-d", target, archive}, nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1 without -o", code)
	}
	if data, _ := os.ReadFile(old); string(data) != "alt\n" {
		t.Fatalf("file was replaced without -o: %q", data)
	}
	out.Reset()
	errb.Reset()
	if code := Run("unzip", []string{"-o", "-q", "-d", target, archive}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d with -o: %s", code, errb.String())
	}
	if data, _ := os.ReadFile(old); string(data) != "inhalt-a\n" {
		t.Fatalf("-o did not replace: %q", data)
	}
}

// TestList: -l prints the table without touching the file system.
func TestList(t *testing.T) {
	archive := makeZip(t)
	var out, errb bytes.Buffer
	if code := Run("unzip", []string{"-l", archive}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	for _, want := range []string{"a.txt", "dir/b.txt", "run.sh", "Length"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("-l misses %q: %s", want, out.String())
		}
	}
}

// TestSingleMember: naming an entry extracts only that one, which is how agents
// pull one file out of a big archive.
func TestSingleMember(t *testing.T) {
	archive := makeZip(t)
	target := t.TempDir()
	var out, errb bytes.Buffer
	if code := Run("unzip", []string{"-q", "-d", target, archive, "dir/b.txt"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if _, err := os.Stat(filepath.Join(target, "a.txt")); err == nil {
		t.Fatal("a.txt was extracted although only dir/b.txt was asked for")
	}
	if _, err := os.Stat(filepath.Join(target, "dir/b.txt")); err != nil {
		t.Fatalf("the asked entry is missing: %v", err)
	}
}
