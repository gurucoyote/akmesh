package tools

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// captureIn runs a command with the given stdin text and returns stdout plus
// the exit code. It used to live in one command's test file; the tests here
// still need it after that file moved to its own package (acmesh-mq1).
func captureIn(t *testing.T, fn Func, args []string, input string) (string, int) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "stdin")
	if err := os.WriteFile(in, []byte(input), 0o600); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	stdin, err := os.Open(in)
	if err != nil {
		t.Fatalf("open stdin: %v", err)
	}
	defer stdin.Close()
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
	code := fn(args, stdin, out, errFile)
	if _, err := out.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	data, err := io.ReadAll(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(data), code
}

// readBack reads a file a command wrote, from its beginning. It lived in
// help_test.go until the wiring tests moved to internal/cmds (acmesh-58me);
// files_test.go still needs it here.
func readBack(t *testing.T, f *os.File) string {
	t.Helper()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	b, err := os.ReadFile(filepath.Clean(f.Name()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}
