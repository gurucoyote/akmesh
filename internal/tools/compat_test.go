package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestTacSeparatorAndRegexp covers tac -s and -r: the separator stays attached
// to its record, and a regexp separator ends a record where it matched.
func TestTacSeparatorAndRegexp(t *testing.T) {
	out, code := captureIn(t, runTac, []string{"-s", "-"}, "a-b-c")
	if code != 0 || out != "cb-a-" {
		t.Fatalf("tac -s - = %d, %q; want cb-a-", code, out)
	}
	out, code = captureIn(t, runTac, []string{"-r", "-s", "[0-9]+"}, "a1b22c")
	if code != 0 || out != "cb22a1" {
		t.Fatalf("tac -r -s = %d, %q; want cb22a1", code, out)
	}
}

// TestSplitNumericChunks: split -n 3 -d writes three files with numeric
// suffixes, and no line is cut in half.
func TestSplitNumericChunks(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(in, []byte("1\n2\n3\n4\n5\n6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if _, code := captureIn(t, runSplit, []string{"-n", "3", "-d", in}, ""); code != 0 {
		t.Fatalf("split -n 3 -d exit %d", code)
	}
	for i, want := range []string{"1\n2\n", "3\n4\n", "5\n6\n"} {
		name := filepath.Join(dir, "x0"+string(rune('0'+i)))
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("part %d missing: %v", i, err)
		}
		if string(data) != want {
			t.Fatalf("part %d = %q, want %q", i, data, want)
		}
	}
}

// TestCutComplement: --complement prints the fields that were not selected.
func TestCutComplement(t *testing.T) {
	out, code := captureIn(t, runCut, []string{"-d", ":", "-f", "1", "--complement"}, "a:b:c\n")
	if code != 0 || out != "b:c\n" {
		t.Fatalf("cut --complement = %d, %q; want b:c", code, out)
	}
}

// TestSedExtendedRegexp: -E makes + and | work without backslashes.
func TestSedExtendedRegexp(t *testing.T) {
	out, code := captureIn(t, runSed, []string{"-E", "s/a+/X/"}, "baaad\n")
	if code != 0 || out != "bXd\n" {
		t.Fatalf("sed -E = %d, %q; want bXd", code, out)
	}
}

// TestNlCombinedFlag: nl -ba is -b a, so blank lines get a number too.
func TestNlCombinedFlag(t *testing.T) {
	out, code := captureIn(t, runNl, []string{"-ba"}, "a\n\nb\n")
	if code != 0 {
		t.Fatalf("nl -ba exit %d", code)
	}
	if !strings.Contains(out, "3") || strings.Count(out, "\n") != 3 {
		t.Fatalf("nl -ba = %q; want three numbered lines", out)
	}
}

// TestDuMaxDepth: -d 0 reports the top directory only, not the tree below it.
func TestDuMaxDepth(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "f.txt"), []byte(strings.Repeat("x", 4096)), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := captureIn(t, runDu, []string{"-d", "0", dir}, "")
	if code != 0 {
		t.Fatalf("du -d 0 exit %d", code)
	}
	if strings.Contains(out, "sub") {
		t.Fatalf("du -d 0 listed a subdirectory: %q", out)
	}
}

// TestDfInodesAndType: -i switches the table to inodes, -T adds a Type column.
func TestDfInodesAndType(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("file system tables are Linux only here")
	}
	out, code := captureIn(t, runDf, []string{"-i", "/"}, "")
	if code != 0 || !strings.Contains(out, "Inodes") {
		t.Fatalf("df -i = %d, %q; want an Inodes column", code, out)
	}
	out, code = captureIn(t, runDf, []string{"-T", "/"}, "")
	if code != 0 || !strings.Contains(out, "Type") {
		t.Fatalf("df -T = %d, %q; want a Type column", code, out)
	}
}

// TestFreeTotalRow: -t adds the Total line an agent sums with.
func TestFreeTotalRow(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("free reads /proc/meminfo")
	}
	out, code := captureIn(t, runFree, []string{"-t"}, "")
	if code != 0 || !strings.Contains(out, "Total:") {
		t.Fatalf("free -t = %d, %q; want a Total line", code, out)
	}
}

// TestKillList: kill -l prints the signal names and exits 0, so an agent can
// look a name up instead of guessing.
func TestKillList(t *testing.T) {
	out, code := captureIn(t, runKill, []string{"-l"}, "")
	if code != 0 || !strings.Contains(out, "KILL") {
		t.Fatalf("kill -l = %d, %q; want the signal names", code, out)
	}
}

// TestTimeoutSignalFlag: -s KILL is accepted and the child still runs.
func TestTimeoutSignalFlag(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the timeout test runs a helper through the Linux path")
	}
	out, code := captureIn(t, runTimeout, []string{"-s", "KILL", "5", "echo", "ok"}, "")
	if code != 0 || !strings.Contains(out, "ok") {
		t.Fatalf("timeout -s KILL = %d, %q; want the command output", code, out)
	}
}

// TestDirnameZero: -z separates results with NUL instead of a newline.
func TestDirnameZero(t *testing.T) {
	out, code := captureIn(t, runDirname, []string{"-z", "a/b"}, "")
	if code != 0 || out != "a\x00" {
		t.Fatalf("dirname -z = %d, %q; want a NUL terminator", code, out)
	}
}

// TestDiffIgnoreSwitches: -i, -w and -b each make a pair that differs only in
// that respect count as equal, so diff exits 0 and prints nothing.
func TestDiffIgnoreSwitches(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	upper := write("upper.txt", "A\n")
	lower := write("lower.txt", "a\n")
	if out, code := captureIn(t, runDiff, []string{upper, lower}, ""); code != 1 || out == "" {
		t.Fatalf("plain diff = %d, %q; want a difference", code, out)
	}
	if out, code := captureIn(t, runDiff, []string{"-i", upper, lower}, ""); code != 0 || out != "" {
		t.Fatalf("diff -i = %d, %q; want equal", code, out)
	}
	spaced := write("spaced.txt", "a  b\n")
	split := write("split.txt", "a\tb\n")
	if _, code := captureIn(t, runDiff, []string{"-w", spaced, split}, ""); code != 0 {
		t.Fatalf("diff -w exit %d, want 0", code)
	}
	if _, code := captureIn(t, runDiff, []string{"-b", spaced, split}, ""); code != 0 {
		t.Fatalf("diff -b exit %d, want 0", code)
	}
}

// TestDiffBriefAndRecursive: -q names the pair instead of printing hunks, and
// -r walks two trees. A file present on one side only is "Only in" without -N
// and a full diff with it.
func TestDiffBriefAndRecursive(t *testing.T) {
	dir := t.TempDir()
	dirA, dirB := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	for _, d := range []string{dirA, dirB} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dirA, "x.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := captureIn(t, runDiff, []string{"-r", dirA, dirB}, "")
	if code != 1 || !strings.Contains(out, "Only in "+dirA+": x.txt") {
		t.Fatalf("diff -r = %d, %q; want an Only in line", code, out)
	}
	out, code = captureIn(t, runDiff, []string{"-r", "-q", dirA, dirB}, "")
	if code != 1 || strings.Contains(out, "@@") {
		t.Fatalf("diff -r -q = %d, %q; want no hunks", code, out)
	}
	out, code = captureIn(t, runDiff, []string{"-r", "-N", dirA, dirB}, "")
	if code != 1 || !strings.Contains(out, "-x") {
		t.Fatalf("diff -r -N = %d, %q; want the removed line", code, out)
	}
}

// TestPatchReverseAndForward: -R undoes a patch, -N skips a patch that is
// already applied instead of failing on it, and the file keeps its content.
func TestPatchReverseAndForward(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if err := os.WriteFile("w.txt", []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "--- a/w.txt\n+++ b/w.txt\n@@ -1,2 +1,2 @@\n a\n-b\n+B\n"
	if _, code := captureIn(t, runPatch, []string{"-p1"}, patch); code != 0 {
		t.Fatalf("apply exit %d", code)
	}
	if data, _ := os.ReadFile("w.txt"); string(data) != "a\nB\n" {
		t.Fatalf("after apply = %q, want a\\nB\\n", data)
	}
	out, code := captureIn(t, runPatch, []string{"-N", "-p1"}, patch)
	if code != 1 || !strings.Contains(out, "Reversed (or previously applied) patch detected!") {
		t.Fatalf("patch -N = %d, %q; want GNU's skip message", code, out)
	}
	if _, code := captureIn(t, runPatch, []string{"-R", "-p1"}, patch); code != 0 {
		t.Fatalf("patch -R exit %d", code)
	}
	if data, _ := os.ReadFile("w.txt"); string(data) != "a\nb\n" {
		t.Fatalf("after -R = %q, want the original content", data)
	}
}

// TestKillSignalZero: -s 0 sends nothing and only checks that the process
// exists, which is how an agent probes a pid. Exit 0 for our own pid, nonzero
// for a pid that cannot exist.
func TestKillSignalZero(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the pid probe is checked on Linux")
	}
	own := strconv.Itoa(os.Getpid())
	if _, code := captureIn(t, runKill, []string{"-s", "0", own}, ""); code != 0 {
		t.Fatalf("kill -s 0 %s exit %d, want 0", own, code)
	}
	if _, code := captureIn(t, runKill, []string{"-s", "0", "999999"}, ""); code == 0 {
		t.Fatal("kill -s 0 on a free pid exited 0, want a failure")
	}
}
