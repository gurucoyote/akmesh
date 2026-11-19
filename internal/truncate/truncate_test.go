package truncate

import (
	"os"
	"path/filepath"
	"testing"
)

// capture runs the command with stdin, stdout and stderr as real files. The
// test used to live in internal/tools, which left this package at 0 % coverage
// and gremlins blind to its 31 mutants (acmesh-i2t).
func capture(t *testing.T, args []string, input string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "stdin")
	if err := os.WriteFile(in, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	inF, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer inF.Close()
	outPath := filepath.Join(dir, "stdout")
	outF, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer outF.Close()
	errPath := filepath.Join(dir, "stderr")
	errF, err := os.Create(errPath)
	if err != nil {
		t.Fatal(err)
	}
	defer errF.Close()
	code := Run(args, inF, outF, errF)
	out, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	errOut, err := os.ReadFile(errPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(out), string(errOut), code
}

// TestReferenceAndCreate: -r takes the size from another file, and a missing
// target file is created unless -c was given.
func TestReferenceAndCreate(t *testing.T) {
	dir := t.TempDir()
	ref := filepath.Join(dir, "ref.txt")
	if err := os.WriteFile(ref, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "t.txt")
	if _, _, code := capture(t, []string{"-r", ref, target}, ""); code != 0 {
		t.Fatalf("truncate -r exit %d", code)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("truncate -r did not create the file: %v", err)
	}
	if info.Size() != 10 {
		t.Fatalf("size = %d, want 10 from the reference file", info.Size())
	}
}

// TestNoCreateKeepsTheFileAway: -c must not create a missing file. GNU is
// silent about it and exits 0 (measured against coreutils 9.7), so a script
// that protects itself with -c must not see an error either (acmesh-i2t).
func TestNoCreateKeepsTheFileAway(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "fehlt.txt")
	out, errOut, code := capture(t, []string{"-c", "-s", "0", target}, "")
	if code != 0 {
		t.Fatalf("truncate -c on a missing file exited %d, want 0 like GNU", code)
	}
	if out != "" || errOut != "" {
		t.Fatalf("truncate -c said something: stdout %q, stderr %q", out, errOut)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("-c created the file anyway: %v", err)
	}
	// Without -c the same call creates the file, so the flag is what made the
	// difference and not a missing permission.
	if _, _, code := capture(t, []string{"-s", "0", target}, ""); code != 0 {
		t.Fatalf("truncate without -c exit %d", code)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("truncate without -c did not create the file: %v", err)
	}
}

// TestShrinkAndGrow: -s sets the size in both directions, and the file is
// filled with zero bytes when it grows.
func TestShrinkAndGrow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := capture(t, []string{"-s", "4", path}, ""); code != 0 {
		t.Fatalf("truncate -s 4 exit %d", code)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "0123" {
		t.Fatalf("after -s 4 the file is %q, want 0123", got)
	}
	if _, _, code := capture(t, []string{"-s", "8", path}, ""); code != 0 {
		t.Fatalf("truncate -s 8 exit %d", code)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 8 || string(got[:4]) != "0123" {
		t.Fatalf("after -s 8 the file is %q, want 0123 plus zeros", got)
	}
	for _, b := range got[4:] {
		if b != 0 {
			t.Fatalf("grown part holds %q, want zero bytes", got[4:])
		}
	}
}
