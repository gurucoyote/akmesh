package chmod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runChmod(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(""), &out, &errOut)
	return out.String(), code
}

// TestPlusXIsTheReason: "+x" without a class is GNU's "a+x". The checkout patch
// fixed it, the fork has to keep that or the silent failure returns.
func TestPlusXIsTheReason(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, code := runChmod(t, "+x", path); code != 0 {
		t.Fatalf("chmod +x exit %d", code)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("mode %v has no execute bit after +x", info.Mode().Perm())
	}
}

// TestRecursiveShortFlag: -R is the spelling agents use; upstream only knew
// --recursive.
func TestRecursiveShortFlag(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(sub, "b.txt")
	if err := os.WriteFile(inner, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, code := runChmod(t, "-R", "700", dir); code != 0 {
		t.Fatalf("chmod -R exit %d", code)
	}
	info, err := os.Stat(inner)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("nested file mode %04o, want 0700", info.Mode().Perm())
	}
}

// TestVerboseWording: -v prints the mode change in GNU's wording, -c stays
// quiet when nothing changed.
func TestVerboseWording(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := runChmod(t, "-v", "755", path)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "mode of '" + path + "' changed from 0644 (rw-r--r--) to 0755 (rwxr-xr-x)"
	if strings.TrimSpace(out) != want {
		t.Fatalf("verbose = %q, want %q", out, want)
	}
	out, _ = runChmod(t, "-c", "755", path)
	if out != "" {
		t.Fatalf("-c printed %q although nothing changed", out)
	}
}

// TestErrors: a bad mode and a missing operand exit 1 with GNU's hint.
func TestErrors(t *testing.T) {
	if _, code := runChmod(t, "999", "x"); code != 1 {
		t.Fatalf("bad mode exit %d, want 1", code)
	}
	if _, code := runChmod(t, "755"); code != 1 {
		t.Fatalf("missing operand exit %d, want 1", code)
	}
}
