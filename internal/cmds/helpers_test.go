package cmds

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"akmesh/internal/cmdrun"
)

// captureIn runs a command with the given stdin text and returns stdout plus
// the exit code. It used to live in one command's test file; the tests here
// still need it after that file moved to its own package (acmesh-mq1).
func captureIn(t *testing.T, fn cmdrun.Func, args []string, input string) (string, int) {
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
