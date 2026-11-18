package tools

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// capture runs one command with its streams redirected into temp files. It
// lives here because the tests of several command files use it and it used to
// sit in one of them (acmesh-mq1 moved that file out).
func capture(t *testing.T, fn Func, args []string) (string, int) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatalf("create stdout: %v", err)
	}
	defer out.Close()
	errFile, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatalf("create stderr: %v", err)
	}
	defer errFile.Close()
	code := fn(args, os.Stdin, out, errFile)
	if _, err := out.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	data, err := io.ReadAll(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(data), code
}
