package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"akmesh/internal/usage"
)

// runFileCommand is runThroughDispatcher with stdin content and a working
// directory, which is what cp, tar, diff and patch need: they read and write
// real files, and the battery used to be their only proof (acmesh-qiz).
func runFileCommand(t *testing.T, dir, stdin, name string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	inPath := filepath.Join(dir, "stdin")
	if err := os.WriteFile(inPath, []byte(stdin), 0o600); err != nil {
		t.Fatal(err)
	}
	stdinF, err := os.Open(inPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stdinF.Close()
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer errf.Close()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("chdir back: %v", err)
		}
	}()

	// diff and patch left this table with acmesh-rnyu; the compatibility tests
	// still call them through the dispatcher, so the lookup falls back to the
	// shims in shims_test.go and keeps the help gate in the path.
	fn := Commands[name]
	if fn == nil {
		switch name {
		case "diff":
			fn = runDiff
		case "patch":
			fn = runPatch
		case "sed":
			fn = runSed
		case "tar":
			fn = runTar
		}
	}
	if fn == nil {
		t.Fatalf("no command %q in the table and no shim for it", name)
	}
	code = usage.Run(name, args, stdinF, out, errf, fn)
	return readBack(t, out), readBack(t, errf), code
}

// writeTree creates the fixture both cp and tar build on.
func writeTree(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "src", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "sub", "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return string(b)
}

// TestCpArchiveCopiesTheTree: "cp -a src dst" keeps the subtree and the content,
// which is what a harness does before touching a checkout (cp-archive,
// cp-archive-nested). The mode is checked on Linux only and stays in the
// battery for the target, so it is not asserted here.
func TestCpArchiveCopiesTheTree(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir)
	_, errOut, code := runFileCommand(t, dir, "", "cp", "-a", "src", "dst")
	if code != 0 {
		t.Fatalf("cp -a = %d, %q", code, errOut)
	}
	if got := readFileString(t, filepath.Join(dir, "dst", "a.txt")); got != "a\n" {
		t.Fatalf("dst/a.txt = %q", got)
	}
	if got := readFileString(t, filepath.Join(dir, "dst", "sub", "b.txt")); got != "b\n" {
		t.Fatalf("dst/sub/b.txt = %q, want the nested file", got)
	}
}

// TestCpTargetDirectory: "cp -t DIR a b" puts both files into DIR
// (cp-target-dir, cp-target-files).
func TestCpTargetDirectory(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runFileCommand(t, dir, "", "cp", "-t", "target", "src/a.txt", "src/sub/b.txt")
	if code != 0 {
		t.Fatalf("cp -t = %d, %q", code, errOut)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "target"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("target holds %v, want a.txt and b.txt", names)
	}
}

// TestCpNoClobber: -n leaves an existing file alone, which is the whole reason
// the fork exists (cp-no-clobber).
func TestCpNoClobber(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := runFileCommand(t, dir, "", "cp", "-n", "src/sub/b.txt", "keep.txt"); code != 0 {
		t.Fatalf("cp -n = %d, %q", code, errOut)
	}
	if got := readFileString(t, filepath.Join(dir, "keep.txt")); got != "old\n" {
		t.Fatalf("cp -n overwrote the file: %q", got)
	}
	// Without -n it does replace.
	if _, errOut, code := runFileCommand(t, dir, "", "cp", "src/sub/b.txt", "keep.txt"); code != 0 {
		t.Fatalf("cp = %d, %q", code, errOut)
	}
	if got := readFileString(t, filepath.Join(dir, "keep.txt")); got != "b\n" {
		t.Fatalf("cp did not replace the file: %q", got)
	}
}

// listArchive returns what "tar -tf" prints for the given archive.
func listArchive(t *testing.T, dir, archive string) string {
	t.Helper()
	out, errOut, code := runFileCommand(t, dir, "", "tar", "-tf", archive)
	if code != 0 {
		t.Fatalf("tar -tf %s = %d, %q", archive, code, errOut)
	}
	return out
}

// TestTarCreateAndList: the two spellings every agent writes, with and without
// -z (tar-create-gzip, tar-list-gzip, tar-list-without-z).
func TestTarCreateAndList(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir)
	if _, errOut, code := runFileCommand(t, dir, "", "tar", "-czf", "t.tgz", "src"); code != 0 {
		t.Fatalf("tar -czf = %d, %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "t.tgz")); err != nil {
		t.Fatalf("no archive: %v", err)
	}
	for _, flag := range []string{"-tzf", "-tf"} {
		out, errOut, code := runFileCommand(t, dir, "", "tar", flag, "t.tgz")
		if code != 0 {
			t.Fatalf("tar %s = %d, %q", flag, code, errOut)
		}
		if !containsLineSuffix(out, "a.txt") {
			t.Fatalf("tar %s listed %q, want the file a.txt", flag, out)
		}
	}
}

// TestTarExclude: a pattern keeps the file out of the archive
// (tar-exclude, tar-exclude-worked).
func TestTarExclude(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "src", "skip.log"), []byte("z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := runFileCommand(t, dir, "", "tar", "-czf", "e.tgz", "--exclude=*.log", "src"); code != 0 {
		t.Fatalf("tar --exclude = %d, %q", code, errOut)
	}
	if out := listArchive(t, dir, "e.tgz"); containsLineSuffix(out, "skip.log") {
		t.Fatalf("skip.log is in the archive: %q", out)
	}
	if out := listArchive(t, dir, "e.tgz"); !containsLineSuffix(out, "a.txt") {
		t.Fatalf("the exclude dropped too much: %q", out)
	}
}

// TestTarExtractStripComponents: -C and --strip-components together are how a
// checkout is unpacked without its top directory (tar-extract-strip,
// tar-strip-file).
func TestTarExtractStripComponents(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir)
	if _, errOut, code := runFileCommand(t, dir, "", "tar", "-czf", "t.tgz", "src"); code != 0 {
		t.Fatalf("tar -czf = %d, %q", code, errOut)
	}
	if err := os.Mkdir(filepath.Join(dir, "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runFileCommand(t, dir, "", "tar", "-xzf", "t.tgz", "-C", "out", "--strip-components=1")
	if code != 0 {
		t.Fatalf("tar -xzf --strip-components = %d, %q", code, errOut)
	}
	if got := readFileString(t, filepath.Join(dir, "out", "a.txt")); got != "a\n" {
		t.Fatalf("out/a.txt = %q, want the file without the src directory", got)
	}
}

// TestTarCreateFromDirectory: -C on create archives a file relative to another
// directory (tar-C-create, tar-C-listing).
func TestTarCreateFromDirectory(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir)
	if _, errOut, code := runFileCommand(t, dir, "", "tar", "-cf", "t2.tar", "-C", "src", "a.txt"); code != 0 {
		t.Fatalf("tar -cf -C = %d, %q", code, errOut)
	}
	out := listArchive(t, dir, "t2.tar")
	if !containsLineSuffix(out, "a.txt") || containsLineSuffix(out, "sub") {
		t.Fatalf("tar -cf -C listed %q, want a.txt only", out)
	}
}

// TestTarRefusesBzip2: -j is not built in and has to say so instead of writing
// a plain archive under a .tbz name (tar-j-refused).
func TestTarRefusesBzip2(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir)
	_, errOut, code := runFileCommand(t, dir, "", "tar", "-cjf", "t.tbz", "src")
	if code == 0 {
		t.Fatalf("tar -cjf succeeded, want a refusal")
	}
	if errOut == "" {
		t.Fatalf("tar -cjf refused without a message")
	}
}

// containsLineSuffix reports whether one line ends with suffix, because the tar
// listing of Windows uses backslashes.
func containsLineSuffix(out, suffix string) bool {
	for _, line := range splitLines(out) {
		if len(line) >= len(suffix) && line[len(line)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			if i > start {
				lines = append(lines, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// TestDiffAndPatchRoundTrip: diff writes the header GNU writes, patch applies
// it from stdin, and the result is the changed file (diff-header, patch-apply,
// patch-result).
func TestDiffAndPatchRoundTrip(t *testing.T) {
	dir := t.TempDir()
	oldBody := "eins\nzwei\ndrei\nvier\nfuenf\n"
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(oldBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("eins\nZWEI\ndrei\nvier\nfuenf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Without -u GNU writes the normal format: the change group and no header
	// (measured 24.11.2026 against GNU diff in the reference image).
	out, errOut, code := runFileCommand(t, dir, "", "diff", "a.txt", "b.txt")
	if code != 1 {
		t.Fatalf("diff = %d, %q; want 1 (files differ)", code, errOut)
	}
	if firstLineOf(out) != "2c2" {
		t.Fatalf("diff first line is %q, want 2c2", firstLineOf(out))
	}
	for _, want := range []string{"< zwei", "---", "> ZWEI"} {
		if !containsLineSuffix(out, want) {
			t.Fatalf("diff output %q has no %q line", out, want)
		}
	}

	// -u gives the unified header GNU writes: the name, a tab, the timestamp.
	uni, errOut, code := runFileCommand(t, dir, "", "diff", "-u", "a.txt", "b.txt")
	if code != 1 {
		t.Fatalf("diff -u = %d, %q", code, errOut)
	}
	if !strings.HasPrefix(firstLineOf(uni), "--- a.txt\t") {
		t.Fatalf("diff -u header is %q, want it to start with --- a.txt and a tab", firstLineOf(uni))
	}
	if !strings.HasPrefix(secondLineOf(uni), "+++ b.txt\t") {
		t.Fatalf("diff -u header has no +++ b.txt line: %q", secondLineOf(uni))
	}

	if _, errOut, code := runFileCommand(t, dir, uni, "patch"); code != 0 {
		t.Fatalf("patch = %d, %q", code, errOut)
	}
	if got := readFileString(t, filepath.Join(dir, "a.txt")); got != "eins\nZWEI\ndrei\nvier\nfuenf\n" {
		t.Fatalf("patched file = %q", got)
	}
}

func secondLineOf(s string) string {
	lines := strings.SplitN(s, "\n", 3)
	if len(lines) < 2 {
		return ""
	}
	return lines[1]
}

func firstLineOf(s string) string {
	lines := splitLines(s)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}
