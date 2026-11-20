package find

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// runFind calls Run the way the dispatcher does and returns the lines.
func runFind(t *testing.T, args []string, dir string) ([]string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	code := Run(args, strings.NewReader(""), &out, &errOut)
	lines := strings.Fields(out.String())
	sort.Strings(lines)
	return lines, errOut.String(), code
}

// tree builds a small hierarchy: a.txt, sub/b.txt, sub/deep/c.log.
func tree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"a.txt": "x", "sub/b.txt": "y", "sub/deep/c.log": "z"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestStartPathIsOptional and the "./" prefix are the two things agents type
// without thinking: `find -name x` and `find . -name x`.
func TestStartPathIsOptional(t *testing.T) {
	dir := tree(t)
	lines, errOut, code := runFind(t, []string{"-name", "a.txt"}, dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if len(lines) != 1 || lines[0] != "./a.txt" {
		t.Fatalf("got %v, want ./a.txt", lines)
	}
	lines, _, _ = runFind(t, []string{".", "-name", "b.txt"}, dir)
	if len(lines) != 1 || lines[0] != "./sub/b.txt" {
		t.Fatalf("got %v, want ./sub/b.txt", lines)
	}
}

// TestMaxDepthReportsTheBoundaryDirectory: GNU prints a directory that sits on
// maxdepth and stops there. Skipping it silently was the first bug of this fork.
func TestMaxDepthReportsTheBoundaryDirectory(t *testing.T) {
	dir := tree(t)
	lines, _, code := runFind(t, []string{".", "-maxdepth", "1", "-type", "d"}, dir)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(lines) != 2 || lines[0] != "." || lines[1] != "./sub" {
		t.Fatalf("got %v, want . and ./sub", lines)
	}
	lines, _, _ = runFind(t, []string{".", "-maxdepth", "1", "-name", "*.txt"}, dir)
	if len(lines) != 1 || lines[0] != "./a.txt" {
		t.Fatalf("got %v, want only ./a.txt", lines)
	}
}

// TestMindepth: -mindepth 2 leaves out the top level.
func TestMindepth(t *testing.T) {
	dir := tree(t)
	lines, _, _ := runFind(t, []string{".", "-mindepth", "2", "-type", "f"}, dir)
	if len(lines) != 2 || lines[0] != "./sub/b.txt" || lines[1] != "./sub/deep/c.log" {
		t.Fatalf("got %v, want the two deeper files", lines)
	}
}

// TestNotAndType: -not negates the following predicate only.
func TestNotAndType(t *testing.T) {
	dir := tree(t)
	lines, _, _ := runFind(t, []string{".", "-type", "f", "-not", "-name", "*.log"}, dir)
	if len(lines) != 2 {
		t.Fatalf("got %v, want a.txt and sub/b.txt", lines)
	}
	for _, l := range lines {
		if strings.HasSuffix(l, ".log") {
			t.Fatalf("%q should have been excluded", l)
		}
	}
}

// TestExecRunsOncePerMatch: -exec with {} replaced, and no implicit print.
func TestExecRunsOncePerMatch(t *testing.T) {
	dir := tree(t)
	lines, errOut, code := runFind(t, []string{".", "-name", "*.txt", "-exec", "echo", "HIT", "{}", ";"}, dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if len(lines) != 4 { // two hits, each "HIT" plus the path
		t.Fatalf("got %v, want two HIT lines with their paths", lines)
	}
}

// TestErrorsAreNotSuccess: a missing start path and an unknown predicate fail.
func TestErrorsAreNotSuccess(t *testing.T) {
	dir := tree(t)
	if _, _, code := runFind(t, []string{"/nicht/vorhanden"}, dir); code != 1 {
		t.Fatalf("missing path exit %d, want 1", code)
	}
	if _, errOut, code := runFind(t, []string{"-Z"}, dir); code != 2 || !strings.Contains(errOut, "usage") {
		t.Fatalf("unknown predicate exit %d, stderr %q; want 2 with usage", code, errOut)
	}
}

// runFindRaw returns stdout byte for byte, which the NUL of -print0 needs.
func runFindRaw(t *testing.T, args []string, dir string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	code := Run(args, strings.NewReader(""), &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestNameTakesAGlob: agents type `-name '*.txt'`, not only an exact name.
func TestNameTakesAGlob(t *testing.T) {
	dir := tree(t)
	lines, errOut, code := runFind(t, []string{".", "-name", "*.txt"}, dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if len(lines) != 2 || lines[0] != "./a.txt" || lines[1] != "./sub/b.txt" {
		t.Fatalf("got %v, want the two .txt files", lines)
	}
}

// TestPathMatchesTheWholePath: -path compares the printed path, so a directory
// can be selected without listing its children one by one.
func TestPathMatchesTheWholePath(t *testing.T) {
	dir := tree(t)
	lines, _, code := runFind(t, []string{".", "-path", "./sub/*"}, dir)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, "./sub/") {
			t.Fatalf("%q does not sit under ./sub/", l)
		}
	}
	if len(lines) == 0 {
		t.Fatal("no path matched ./sub/*")
	}
}

// TestRegexMatchesTheWholePath: -regex is not anchored by the caller, so the
// expression has to describe the whole printed path.
func TestRegexMatchesTheWholePath(t *testing.T) {
	dir := tree(t)
	lines, errOut, code := runFind(t, []string{".", "-regex", `.*\.log$`}, dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if len(lines) != 1 || lines[0] != "./sub/deep/c.log" {
		t.Fatalf("got %v, want only ./sub/deep/c.log", lines)
	}
}

// TestModeMatchesPermissions: -mode compares the permission bits only.
func TestModeMatchesPermissions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "open.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tight.txt"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, _, _ := runFind(t, []string{".", "-mode", "644"}, dir)
	if len(lines) != 1 || lines[0] != "./open.txt" {
		t.Fatalf("got %v, want only ./open.txt", lines)
	}
}

// TestPrint0EndsEveryNameWithNUL: the form find -print0 | xargs -0 needs. A
// newline would split a name with a blank in it.
func TestPrint0EndsEveryNameWithNUL(t *testing.T) {
	dir := tree(t)
	out, errOut, code := runFindRaw(t, []string{".", "-maxdepth", "1", "-name", "*.txt", "-print0"}, dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if strings.Contains(out, "\n") {
		t.Fatalf("output %q contains a newline, want NUL only", out)
	}
	parts := strings.Split(out, "\x00")
	if len(parts) != 2 || parts[0] != "./a.txt" {
		t.Fatalf("got %q, want exactly ./a.txt and the trailing NUL", out)
	}
}

// TestSymlinkIsListedButNotFollowed: without -L the walk reports the link and
// stops, so a link to a parent cannot loop forever.
func TestSymlinkIsListedButNotFollowed(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "inside.txt"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}

	lines, _, code := runFind(t, []string{".", "-type", "l"}, dir)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(lines) != 1 || lines[0] != "./link" {
		t.Fatalf("got %v, want only ./link", lines)
	}

	// The walk must not descend through the link: inside.txt stays invisible.
	all, _, _ := runFind(t, []string{"."}, dir)
	for _, l := range all {
		if strings.Contains(l, "inside.txt") {
			t.Fatalf("%q was reached through the symlink", l)
		}
	}
}

// TestWalkErrorIsNotSuccess: a start path whose parent is a file makes the walk
// fail; the exit must say so instead of reporting an empty, clean tree.
func TestWalkErrorIsNotSuccess(t *testing.T) {
	dir := tree(t)
	out, errOut, code := runFindRaw(t, []string{"a.txt/x"}, dir)
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stdout %q, stderr %q)", code, out, errOut)
	}
	if errOut == "" {
		t.Fatal("no message on stderr")
	}
}

// TestExplicitPrintIsTheDefault: `-print` must not print twice and must not
// suppress the walk's own output.
func TestExplicitPrintIsTheDefault(t *testing.T) {
	dir := tree(t)
	plain, _, _ := runFind(t, []string{".", "-maxdepth", "1", "-name", "*.txt"}, dir)
	explicit, _, code := runFind(t, []string{".", "-maxdepth", "1", "-name", "*.txt", "-print"}, dir)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Join(plain, " ") != strings.Join(explicit, " ") {
		t.Fatalf("-print changed the output: %v against %v", plain, explicit)
	}
}

// TestPredicateErrorsAreUsageErrors pins every parse failure to exit 2 with the
// usage text: an agent that mistypes a predicate must see the spelling, not an
// empty result.
func TestPredicateErrorsAreUsageErrors(t *testing.T) {
	dir := tree(t)
	cases := [][]string{
		{"-name"},                        // value missing
		{"-maxdepth"},                    // number missing
		{"-maxdepth", "x"},               // not a number
		{"-mindepth", "-1"},              // negative
		{"-type", "q"},                   // no such file type
		{"-mode", "999"},                 // not octal for a mode
		{"-regex", "["},                  // broken expression
		{"-exec", "echo"},                // no terminating ';'
		{"-not"},                         // nothing to negate
		{".", "-name", "a", "sub/b.txt"}, // path after a predicate
	}
	for _, args := range cases {
		_, errOut, code := runFind(t, args, dir)
		if code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
			continue
		}
		if !strings.Contains(errOut, "usage: find") {
			t.Errorf("%v: stderr %q does not carry the usage line", args, errOut)
		}
	}
}

// TestTypeDirectorySelectsDirectories: -type d is the other half of -type f.
// The names are pinned, not the count: with only a count, a mutant that maps
// "d" to the file bits still passes, because it then finds three files.
func TestTypeDirectorySelectsDirectories(t *testing.T) {
	dir := tree(t)
	lines, _, _ := runFind(t, []string{".", "-type", "d"}, dir)
	want := []string{".", "./sub", "./sub/deep"}
	if strings.Join(lines, " ") != strings.Join(want, " ") {
		t.Fatalf("got %v, want %v", lines, want)
	}
}
