package stat

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// capture runs the command with stdin, stdout and stderr as real files, the way
// the binary does. These tests used to live in internal/tools and reach stat
// through the table; there they left the package at 0 % coverage, which made
// gremlins blind to 113 mutants (acmesh-i2t).
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
	outF, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	defer outF.Close()
	errF, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	defer errF.Close()
	code := Run(args, inF, outF, errF)
	out, err := os.ReadFile(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	errOut, err := os.ReadFile(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	return string(out), string(errOut), code
}

// TestTerseUsesHexMode pins stat -t: 16 space separated fields in the GNU
// order, and %f and %D in hex. An agent splits that line, so position matters.
func TestTerseUsesHexMode(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the inode, uid and block columns are Linux values")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := capture(t, []string{"-t", path}, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	fields := strings.Fields(strings.TrimSuffix(out, "\n"))
	if len(fields) != 16 {
		t.Fatalf("stat -t gave %d fields, want 16: %q", len(fields), out)
	}
	if fields[0] != path {
		t.Fatalf("field 1 = %q, want the name", fields[0])
	}
	if want := "81a4"; fields[3] != want { // 0100644 in hex
		t.Fatalf("raw mode = %q, want %q (hex, not decimal)", fields[3], want)
	}
}

// TestPrintfKeepsEscapes: --printf prints no newline of its own and turns
// backslash escapes into bytes, -c does neither.
func TestPrintfKeepsEscapes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := capture(t, []string{"--printf", "%n\t%s", path}, "")
	if code != 0 || out != path+"\t6" {
		t.Fatalf("--printf = %d, %q; want no trailing newline and a real tab", code, out)
	}
	out, _, code = capture(t, []string{"-c", "%s", path}, "")
	if code != 0 || out != "6\n" {
		t.Fatalf("-c = %d, %q; want a trailing newline", code, out)
	}
}

// TestFollowsSymlinkOnlyWithL: without -L the link itself is described, as GNU
// does. An agent that forgets -L must not silently get the target.
func TestFollowsSymlinkOnlyWithL(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("a.txt", link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	out, _, _ := capture(t, []string{"-c", "%F", link}, "")
	if out != "symbolic link\n" {
		t.Fatalf("stat on a link = %q, want symbolic link", out)
	}
	out, _, _ = capture(t, []string{"-L", "-c", "%F", link}, "")
	if out != "regular file\n" {
		t.Fatalf("stat -L on a link = %q, want regular file", out)
	}
}

// TestFileSystemIsHonest: stat -f answers with numbers on Linux and with an
// error where the platform has no statfs, instead of printing zeros.
func TestFileSystemIsHonest(t *testing.T) {
	out, _, code := capture(t, []string{"-f", "-c", "%T", "."}, "")
	if runtime.GOOS == "linux" {
		if code != 0 || strings.TrimSpace(out) == "" || strings.Contains(out, "?") {
			t.Fatalf("stat -f -c %%T = %d, %q; want a file system type", code, out)
		}
		return
	}
	if code == 0 {
		t.Fatalf("stat -f on %s exited 0 with %q, want a reported error", runtime.GOOS, out)
	}
}

// TestCountedFieldsAndOperandErrors: the two -c formats the battery used
// (stat-size, stat-name) and the two refusals (stat-missing, stat-no-operand).
// Checked against GNU stat: a missing file is 1, no operand at all is 2.
func TestCountedFieldsAndOperandErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statme.txt")
	if err := os.WriteFile(path, []byte("hallo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, code := capture(t, []string{"-c", "%s", path}, "")
	if code != 0 || strings.TrimSpace(out) != "6" {
		t.Fatalf("stat -c %%s = %d, %q, want 6", code, out)
	}
	out, _, code = capture(t, []string{"-c", "%n", path}, "")
	if code != 0 || strings.TrimSpace(out) != path {
		t.Fatalf("stat -c %%n = %d, %q, want the path", code, out)
	}
	if _, errOut, code := capture(t, []string{"-c", "%s", "/nicht/vorhanden"}, ""); code != 1 || errOut == "" {
		t.Fatalf("stat on a missing file = %d, %q, want 1 and a message", code, errOut)
	}
	if _, errOut, code := capture(t, nil, ""); code != 2 || errOut == "" {
		t.Fatalf("stat without an operand = %d, %q, want 2 and a message", code, errOut)
	}
}
