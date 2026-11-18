package dirname

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

// TestKeepsTheDirectoryPart pins the three shapes dirname has to answer.
func TestKeepsTheDirectoryPart(t *testing.T) {
	cases := map[string]string{
		"/a/b/c.txt": "/a/b\n",
		"/a":         "/\n",
		"rel.txt":    ".\n",
	}
	for in, want := range cases {
		if got, code := run(t, []string{in}); code != 0 || got != want {
			t.Errorf("dirname %q: got %q, want %q (exit %d)", in, got, want, code)
		}
	}
}
