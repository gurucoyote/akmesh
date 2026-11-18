package touch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func runTouch(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var out, errOut stringWriter
	code := Run(args, nil, &out, &errOut)
	return out.text(), code
}

type stringWriter struct{ b []byte }

func (s *stringWriter) Write(p []byte) (int, error) { s.b = append(s.b, p...); return len(p), nil }
func (s *stringWriter) text() string                { return string(s.b) }

// TestStampForm: -t reads [[CC]YY]MMDDhhmm[.ss] in local time, the form GNU and
// every backup script uses.
func TestStampForm(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, code := runTouch(t, "-t", "202001020304", file); code != 0 {
		t.Fatalf("touch -t exit %d", code)
	}
	want := time.Date(2020, 1, 2, 3, 4, 0, 0, time.Local)
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(want) {
		t.Fatalf("mtime %v, want %v", info.ModTime(), want)
	}
	if _, code := runTouch(t, "-t", "202001020304.05", file); code != 0 {
		t.Fatalf("touch -t with seconds exit %d", code)
	}
	info, _ = os.Stat(file)
	if info.ModTime().Second() != 5 {
		t.Fatalf("seconds %d, want 5", info.ModTime().Second())
	}
}

// TestDateForms: -d takes the written forms, not only RFC3339.
func TestDateForms(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2020, 1, 2, 3, 4, 5, 0, time.Local)
	for _, value := range []string{"2020-01-02 03:04:05", "2020-01-02T03:04:05"} {
		if _, code := runTouch(t, "-d", value, file); code != 0 {
			t.Fatalf("touch -d %q exit %d", value, code)
		}
		info, _ := os.Stat(file)
		if !info.ModTime().Equal(want) {
			t.Fatalf("-d %q gave %v, want %v", value, info.ModTime(), want)
		}
	}
}

// TestReference: -r takes the times from another file.
func TestReference(t *testing.T) {
	dir := t.TempDir()
	ref := filepath.Join(dir, "ref")
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(ref, []byte("r"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("t"), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2019, 5, 6, 7, 8, 9, 0, time.Local)
	if err := os.Chtimes(ref, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, code := runTouch(t, "-r", ref, target); code != 0 {
		t.Fatalf("touch -r exit %d", code)
	}
	info, _ := os.Stat(target)
	if !info.ModTime().Equal(stamp) {
		t.Fatalf("-r gave %v, want %v", info.ModTime(), stamp)
	}
}

// TestNoCreate: -c does not create a missing file.
func TestNoCreate(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nix")
	if _, code := runTouch(t, "-c", missing); code != 0 {
		t.Fatalf("touch -c exit %d", code)
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("touch -c created the file")
	}
}
