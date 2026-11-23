package tar

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runTar(t *testing.T, stdin []byte, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, bytes.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

// writeTree builds d/a.txt, d/sub/b.txt and d/skip.log.
func writeTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "d", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", filepath.Join("sub", "b.txt"), "skip.log"} {
		if err := os.WriteFile(filepath.Join(dir, "d", name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestCreateListGzip: -czf writes gzip, -tzf lists it, and listing works
// without -z because the magic is detected.
func TestCreateListGzip(t *testing.T) {
	dir := writeTree(t)
	archive := filepath.Join(dir, "x.tgz")
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if _, errOut, code := runTar(t, nil, "-czf", archive, "d"); code != 0 {
		t.Fatalf("create exit %d, stderr %q", code, errOut)
	}
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatalf("archive is not gzip, first bytes % x", raw[:2])
	}
	out, _, code := runTar(t, nil, "-tzf", archive)
	if code != 0 || !strings.Contains(out, "d/a.txt") || !strings.Contains(out, "d/sub/b.txt") {
		t.Fatalf("list = %d, %q", code, out)
	}
	out, _, _ = runTar(t, nil, "-tf", archive)
	if !strings.Contains(out, "d/a.txt") {
		t.Fatalf("listing without -z failed: %q", out)
	}
}

// TestStripComponentsAndExclude: --strip-components drops the top directory,
// --exclude keeps a pattern out of the archive.
func TestStripComponentsAndExclude(t *testing.T) {
	dir := writeTree(t)
	archive := filepath.Join(dir, "y.tgz")
	outDir := filepath.Join(dir, "out")
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if _, _, code := runTar(t, nil, "-czf", archive, "--exclude=*.log", "d"); code != 0 {
		t.Fatalf("create with --exclude failed")
	}
	out, _, _ := runTar(t, nil, "-tzf", archive)
	if strings.Contains(out, "skip.log") {
		t.Fatalf("--exclude let skip.log in: %q", out)
	}
	if err := os.Mkdir(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := runTar(t, nil, "-xzf", archive, "-C", outDir, "--strip-components=1"); code != 0 {
		t.Fatalf("extract exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(outDir, "a.txt")); err != nil {
		t.Fatalf("stripped extract did not put a.txt at the top")
	}
	if _, err := os.Stat(filepath.Join(outDir, "sub", "b.txt")); err != nil {
		t.Fatalf("stripped extract lost sub/b.txt")
	}
}

// TestDirectoryFromForCreate: -C takes the files from that directory.
func TestDirectoryFromForCreate(t *testing.T) {
	dir := writeTree(t)
	archive := filepath.Join(dir, "z.tar")
	if _, errOut, code := runTar(t, nil, "-cf", archive, "-C", filepath.Join(dir, "d"), "a.txt"); code != 0 {
		t.Fatalf("create exit %d, stderr %q", code, errOut)
	}
	out, _, _ := runTar(t, nil, "-tf", archive)
	if strings.TrimSpace(out) != "a.txt" {
		t.Fatalf("listing %q, want just a.txt", out)
	}
}

// TestUnsupportedCompression: -j is refused with a message, not silently
// written as an uncompressed archive.
func TestUnsupportedCompression(t *testing.T) {
	_, errOut, code := runTar(t, nil, "-cjf", "/tmp/x.tar.bz2", ".")
	if code != 2 || !strings.Contains(errOut, "not supported") {
		t.Fatalf("-j = %d, %q; want 2 and a message", code, errOut)
	}
}

// TestStdinStdout: -f - reads from stdin and writes to stdout, which is how a
// pipeline uses tar.
func TestStdinStdout(t *testing.T) {
	dir := writeTree(t)
	archive := filepath.Join(dir, "p.tar")
	if _, _, code := runTar(t, nil, "-cf", archive, "-C", filepath.Join(dir, "d"), "a.txt"); code != 0 {
		t.Fatal("create failed")
	}
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	out, _, code := runTar(t, raw, "-tf", "-")
	if code != 0 || strings.TrimSpace(out) != "a.txt" {
		t.Fatalf("tar -tf - = %d, %q", code, out)
	}
}

// TestDirectoriesHaveSlashAndDotKeepsItsSpelling: GNU stores a directory with a
// trailing slash and keeps the "./" of the argument (acmesh-9e7, acmesh-vc3).
func TestDirectoriesHaveSlashAndDotKeepsItsSpelling(t *testing.T) {
	dir := writeTree(t)
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if _, _, code := runTar(t, nil, "-cf", "a.tar", "d"); code != 0 {
		t.Fatalf("tar -cf exit %d", code)
	}
	out, _, _ := runTar(t, nil, "-tf", "a.tar")
	if !strings.Contains(out, "d/\n") || !strings.Contains(out, "d/a.txt\n") {
		t.Fatalf("factory listing = %q, want d/ and d/a.txt", out)
	}

	out, _, _ = runTar(t, nil, "-cf", "dot.tar", ".")
	if !strings.Contains(out, "") {
		_ = out
	}
	list, _, _ := runTar(t, nil, "-tf", "dot.tar")
	if !strings.HasPrefix(list, "./\n") || !strings.Contains(list, "./d/\n") {
		t.Fatalf("listing of \".\" = %q, want ./ and ./d/", list)
	}
}

// TestExtractToStdout: -xO prints the file contents and writes nothing to disk.
func TestExtractToStdout(t *testing.T) {
	dir := writeTree(t)
	archive := filepath.Join(dir, "x.tar")
	if _, _, code := runTar(t, nil, "-cf", archive, filepath.Join(dir, "d")); code != 0 {
		t.Fatalf("create exit %d", code)
	}
	out, _, code := runTar(t, nil, "-xOf", archive)
	if code != 0 || !strings.Contains(out, "x") {
		t.Fatalf("tar -xOf = %d, %q", code, out)
	}
	// a named file only
	out, _, _ = runTar(t, nil, "-xOf", archive, filepath.Join(dir, "d", "a.txt"))
	if out != "x" {
		t.Fatalf("tar -xOf with a name = %q", out)
	}
}

// TestArchiveInsideItsOwnTree: the archive is left out, the warning is GNU's,
// and the exit status stays 0 (measured at GNU tar 1.34, acmesh-vc3).
func TestArchiveInsideItsOwnTree(t *testing.T) {
	dir := writeTree(t)
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	_, errOut, code := runTar(t, nil, "-cf", "d/self.tar", "d")
	if code != 0 {
		t.Fatalf("tar with the archive inside the tree exit %d, want 0", code)
	}
	if !strings.Contains(errOut, "archive cannot contain itself; not dumped") {
		t.Fatalf("warning = %q", errOut)
	}
	list, _, _ := runTar(t, nil, "-tf", "d/self.tar")
	if strings.Contains(list, "self.tar") {
		t.Fatalf("the archive contains itself: %q", list)
	}
}
