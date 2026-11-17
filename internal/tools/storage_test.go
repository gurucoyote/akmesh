package tools

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// capture runs one command with its streams redirected into temp files and
// returns what it wrote to stdout together with the exit code.
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

// TestHumanBytes pins the format of -h, because a size a screen reader reads
// wrong is worse than no size at all.
func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{512, "512"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{10 * 1024, "10K"},
		{1024 * 1024, "1.0M"},
		{3 * 1024 * 1024 * 1024, "3.0G"},
		{1906737 * 1024 * 1024, "1.8T"},
	}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Fatalf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestParseUnitFlags checks that -h is reported separately and that plain
// numbers stay in 1K blocks, as in df.
func TestParseUnitFlags(t *testing.T) {
	human, unit, rest, _ := parseUnitFlags([]string{"-h", "/"})
	if !human || len(rest) != 1 || rest[0] != "/" {
		t.Fatalf("human=%v unit=%d rest=%v", human, unit, rest)
	}
	human, unit, _, _ = parseUnitFlags([]string{"-m"})
	if human || unit != 1024*1024 {
		t.Fatalf("-m: human=%v unit=%d", human, unit)
	}
	human, unit, _, _ = parseUnitFlags(nil)
	if human || unit != 1024 {
		t.Fatalf("default: human=%v unit=%d", human, unit)
	}
}

// TestDuCountsTree checks the walk on a known tree: -s prints one line, the
// plain call lists the tree, -a lists files, -c appends a total.
func TestDuCountsTree(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat("x", 4096)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := capture(t, runDu, []string{"-b", "-s", dir})
	if code != 0 {
		t.Fatalf("du -s exit %d: %s", code, out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("-s must print one line, got %d:\n%s", len(lines), out)
	}
	fields := strings.Split(lines[0], "\t")
	if len(fields) != 2 || fields[1] != dir {
		t.Fatalf("unexpected line %q", lines[0])
	}
	size, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		t.Fatalf("size %q is not a number", fields[0])
	}
	if size < 8192 {
		t.Fatalf("size %d is smaller than the two 4K files", size)
	}
	oneFile, _ := capture(t, runDu, []string{"-b", "-s", filepath.Join(dir, "a.txt")})
	oneSize, _ := strconv.ParseInt(strings.TrimSpace(strings.Split(oneFile, "\t")[0]), 10, 64)
	if size < oneSize {
		t.Fatalf("tree size %d is smaller than a single file %d", size, oneSize)
	}

	// Without -s: the root and its subdirectory, but not the files.
	out, _ = capture(t, runDu, []string{"--bytes", dir})
	if strings.Count(out, "\n") != 2 {
		t.Fatalf("without -s the tree must list itself and its subdirectory:\n%s", out)
	}

	out, _ = capture(t, runDu, []string{"-b", "-a", dir})
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.txt") {
		t.Fatalf("-a must list the files:\n%s", out)
	}

	out, _ = capture(t, runDu, []string{"-b", "-s", "-c", dir})
	if !strings.HasSuffix(strings.TrimRight(out, "\n"), "total") {
		t.Fatalf("-c must end with a total line:\n%s", out)
	}
}

// TestDfAndFreeRejectUnknown checks that a bad switch is refused instead of
// silently ignored, and that the output carries what scripts grep for.
func TestDfAndFreeRejectUnknown(t *testing.T) {
	if _, code := capture(t, runDf, []string{"-Z"}); code != 2 {
		t.Fatalf("df -Z = %d, want 2", code)
	}
	if _, code := capture(t, runDu, []string{"-Z"}); code != 2 {
		t.Fatalf("du -Z = %d, want 2", code)
	}
	if _, code := capture(t, runFree, []string{"-Z"}); code != 2 {
		t.Fatalf("free -Z = %d, want 2", code)
	}

	out, code := capture(t, runFree, []string{"-h"})
	if code != 0 {
		t.Fatalf("free -h exit %d", code)
	}
	if !strings.Contains(out, "Mem:") || !strings.Contains(out, "available") {
		t.Fatalf("free -h output:\n%s", out)
	}

	out, code = capture(t, runDf, []string{"-h"})
	if code != 0 {
		t.Fatalf("df -h exit %d", code)
	}
	if !strings.Contains(out, "Mounted on") || !strings.Contains(out, "Use%") {
		t.Fatalf("df -h output:\n%s", out)
	}
	// Every line must keep the same shape, so column based parsing works.
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if len(strings.Fields(line)) < 6 {
			t.Fatalf("line with too few fields: %q", line)
		}
		if !strings.Contains(line, "%") {
			t.Fatalf("line without a percent column: %q", line)
		}
	}
}
