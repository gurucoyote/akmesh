package basename

import (
	"os"
	"path/filepath"
	"testing"
)

func run(t *testing.T, args []string) (string, int) {
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
	data, _ := os.ReadFile(out.Name())
	return string(data), code
}

// TestStripsDirectoriesAndSuffix is the form a script uses to name a file.
func TestStripsDirectoriesAndSuffix(t *testing.T) {
	if got, code := run(t, []string{"/a/b/c.txt", ".txt"}); code != 0 || got != "c\n" {
		t.Fatalf("got %q, exit %d", got, code)
	}
	if got, code := run(t, []string{"/a/b/c.txt"}); code != 0 || got != "c.txt\n" {
		t.Fatalf("got %q, exit %d", got, code)
	}
	if got, code := run(t, []string{"/"}); code != 0 || got != "/\n" {
		t.Fatalf("root: got %q, exit %d", got, code)
	}
}
