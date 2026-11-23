package grep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runGrep calls Run the way the dispatcher does.
func runGrep(t *testing.T, args []string, input string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(input), &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestExtendedRegexpAndWordMatch: -E needs no backslashes for +, -w keeps the
// match on word boundaries.
func TestExtendedRegexpAndWordMatch(t *testing.T) {
	out, _, code := runGrep(t, []string{"-E", "a+b"}, "aaab\nxx\n")
	if code != 0 || out != "aaab\n" {
		t.Fatalf("grep -E = %d, %q", code, out)
	}
	out, _, _ = runGrep(t, []string{"-w", "cat"}, "cat\nconcat\n")
	if out != "cat\n" {
		t.Fatalf("grep -w = %q, want only the whole word", out)
	}
}

// TestContextLines: -A and -B print neighbours, groups separated by "--", and a
// line that is both after-context and before-context appears once.
func TestContextLines(t *testing.T) {
	input := "one\ntwo\nMATCH\nfour\nfive\nMATCH\nseven\n"
	out, _, _ := runGrep(t, []string{"-A", "1", "MATCH"}, input)
	want := "MATCH\nfour\n--\nMATCH\nseven\n"
	if out != want {
		t.Fatalf("grep -A 1 = %q, want %q", out, want)
	}
	// With -C 1 the two windows overlap, so GNU prints them as one group and
	// no "--" appears. Checked against GNU grep 3.11 in the container.
	out, _, _ = runGrep(t, []string{"-C", "1", "MATCH"}, input)
	want = "two\nMATCH\nfour\nfive\nMATCH\nseven\n"
	if out != want {
		t.Fatalf("grep -C 1 = %q, want %q", out, want)
	}
}

// TestOnlyMatchAndNumbers: -o prints each match, -n adds the line number.
func TestOnlyMatchAndNumbers(t *testing.T) {
	out, _, _ := runGrep(t, []string{"-o", "-n", "ab"}, "ab ab\nxx\n")
	if out != "1:ab\n1:ab\n" {
		t.Fatalf("grep -o -n = %q", out)
	}
}

// TestExitStatus: 0 when something matched, 1 when nothing did, 2 for a broken
// pattern. Upstream returned 0 for "no match", which a shell script misreads.
func TestExitStatus(t *testing.T) {
	if _, _, code := runGrep(t, []string{"nope"}, "a\nb\n"); code != 1 {
		t.Fatalf("no match exit %d, want 1", code)
	}
	if _, errOut, code := runGrep(t, []string{"("}, "a\n"); code != 2 || !strings.Contains(errOut, "grep:") {
		t.Fatalf("bad pattern exit %d, stderr %q; want 2 with a message", code, errOut)
	}
}

// TestRecursiveNamesTheFile: -r prints the path prefix and keeps the "./" that
// GNU prints for the start path ".".
func TestRecursiveNamesTheFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	out, _, code := runGrep(t, []string{"-r", "hit", "."}, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "./a.txt:hit") || !strings.Contains(out, "./sub/b.txt:hit") {
		t.Fatalf("grep -r = %q, want both files with the ./ prefix", out)
	}
}

// TestRecursiveDotRootKeepsTheDotOfHiddenDirs: filepath.Walk already cleans the
// names ("<>.git/HEAD", not "./.git/HEAD"), so a string prefix trim of "." ate
// the dot of the directory name and grep then opened a path that did not exist
// ("grep: ./git/HEAD: No such file or directory"). GNU grep prints "./.git/HEAD"
// and finds the line.
func TestRecursiveDotRootKeepsTheDotOfHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".hidden", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden", "deep", "h.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	out, errOut, code := runGrep(t, []string{"-r", "hit", "."}, "")
	if errOut != "" {
		t.Fatalf("stderr = %q, want none: the hidden directory must be readable", errOut)
	}
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if !strings.Contains(out, "./.hidden/deep/h.txt:hit") {
		t.Fatalf("grep -r = %q, want ./.hidden/deep/h.txt:hit", out)
	}
}

// TestRecursiveRootsAgreeWithTheirOwnSpelling: the display keeps the spelling of
// the start path - "." stays "./", a relative root stays without "./", and an
// absolute root is not doubled (Windows: os.PathSeparator is "\\" while the root
// arrives with "/", the prefix trim then failed and appended the whole path).
func TestRecursiveRootsAgreeWithTheirOwnSpelling(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	out, errOut, _ := runGrep(t, []string{"-r", "hit", "sub"}, "")
	if errOut != "" || !strings.Contains(out, "sub/b.txt:hit") || strings.Contains(out, "./sub/") {
		t.Fatalf("root \"sub\": out %q stderr %q, want sub/b.txt:hit", out, errOut)
	}

	out, errOut, _ = runGrep(t, []string{"-r", "hit", dir}, "")
	if errOut != "" {
		t.Fatalf("absolute root: stderr %q, want none", errOut)
	}
	if !strings.Contains(out, dir+"/sub/b.txt:hit") {
		t.Fatalf("absolute root: out %q, want %s/sub/b.txt:hit once", out, dir)
	}
}

// TestCountPerFile: -c prints one count per file, with the name when there is
// more than one file.
func TestCountPerFile(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, "one.txt")
	two := filepath.Join(dir, "two.txt")
	if err := os.WriteFile(one, []byte("a\nb\na\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, _ := runGrep(t, []string{"-c", "a", one, two}, "")
	if !strings.Contains(out, one+":2") || !strings.Contains(out, two+":0") {
		t.Fatalf("grep -c = %q, want one count per file", out)
	}
}

// GNU grep 3.11 in the container produced every expected value below; the
// fixtures are "alpha 1 2 3 beta" (gap.txt) and the tree with a symlinked
// directory (real/deep -> ../linkdir).

// TestContextGroupsAreSeparatedByDashes: two matches with a gap between their
// windows are two groups, and GNU puts a bare "--" between them.
func TestContextGroupsAreSeparatedByDashes(t *testing.T) {
	input := "alpha\n1\n2\n3\nbeta\n"
	out, _, code := runGrep(t, []string{"-A", "1", "-e", "alpha", "-e", "beta"}, input)
	if code != 0 || out != "alpha\n1\n--\nbeta\n" {
		t.Fatalf("grep -A 1 = %d, %q, want alpha,1,--,beta", code, out)
	}
	out, _, _ = runGrep(t, []string{"-C", "1", "-e", "alpha", "-e", "beta"}, input)
	if want := "alpha\n1\n--\n3\nbeta\n"; out != want {
		t.Fatalf("grep -C 1 = %q, want %q", out, want)
	}
	out, _, _ = runGrep(t, []string{"-B", "1", "beta"}, input)
	if want := "3\nbeta\n"; out != want {
		t.Fatalf("grep -B 1 = %q, want %q", out, want)
	}
}

// TestAdjacentWindowsAreOneGroup: two windows that share an edge are one group,
// so no "--" appears. GNU grep 3.11 prints a, x, b for "a x b" with -A1.
func TestAdjacentWindowsAreOneGroup(t *testing.T) {
	out, _, _ := runGrep(t, []string{"-A", "1", "-e", "a", "-e", "b"}, "a\nx\nb\n")
	if out != "a\nx\nb\n" {
		t.Fatalf("got %q, want the three lines without a separator", out)
	}
}

// TestContextAndNumbersUseTheDashSeparator: with -n a match reads "5:beta", a
// context line "4-3". Getting this wrong breaks every awk on grep -A output.
func TestContextAndNumbersUseTheDashSeparator(t *testing.T) {
	input := "alpha\n1\n2\n3\nbeta\n"
	out, _, _ := runGrep(t, []string{"-n", "-C", "1", "-e", "alpha", "-e", "beta"}, input)
	want := "1:alpha\n2-1\n--\n4-3\n5:beta\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// TestQuietPrintsNothingAndKeepsTheStatus: -q is the cheap yes/no an agent uses
// in an if; the output must stay empty, the status must speak.
func TestQuietPrintsNothingAndKeepsTheStatus(t *testing.T) {
	out, errOut, code := runGrep(t, []string{"-q", "alpha"}, "alpha\n")
	if code != 0 || out != "" || errOut != "" {
		t.Fatalf("grep -q hit = %d, stdout %q, stderr %q", code, out, errOut)
	}
	if _, _, code := runGrep(t, []string{"-q", "zenix"}, "alpha\n"); code != 1 {
		t.Fatalf("grep -q miss = %d, want 1", code)
	}
}

// TestInvertSelectsTheRest: -v keeps the lines without a match, -cv counts them.
func TestInvertSelectsTheRest(t *testing.T) {
	input := "one\nalpha\nmid\nbeta\ngamma\n"
	out, _, code := runGrep(t, []string{"-v", "alpha"}, input)
	if code != 0 || out != "one\nmid\nbeta\ngamma\n" {
		t.Fatalf("grep -v = %d, %q", code, out)
	}
	out, _, _ = runGrep(t, []string{"-cv", "alpha"}, input)
	if out != "4\n" {
		t.Fatalf("grep -cv = %q, want 4", out)
	}
}

// TestMissingFileIsTwoWithGNUsWording: the second cheapest signal an agent reads
// is the status, the first is the message. "stat x: no such file" is not grep.
func TestMissingFileIsTwoWithGNUsWording(t *testing.T) {
	_, errOut, code := runGrep(t, []string{"alpha", "/nicht/vorhanden"}, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if want := "grep: /nicht/vorhanden: No such file or directory\n"; errOut != want {
		t.Fatalf("stderr %q, want %q", errOut, want)
	}
}

// TestUnknownOptionIsTwoWithGNUsWording: the message names the whole token and
// points at --help, exit 2 as GNU does.
func TestUnknownOptionIsTwoWithGNUsWording(t *testing.T) {
	_, errOut, code := runGrep(t, []string{"--bogus", "alpha"}, "alpha\n")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.HasPrefix(errOut, "grep: unrecognized option '--bogus'\n") {
		t.Fatalf("stderr %q does not name the option", errOut)
	}
	if !strings.Contains(errOut, "Try 'grep --help'") {
		t.Fatalf("stderr %q does not point at --help", errOut)
	}
}

// TestDirectoryWithoutRecursiveIsTwo: GNU refuses to read a directory and says so.
func TestDirectoryWithoutRecursiveIsTwo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runGrep(t, []string{"alpha", dir}, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.HasSuffix(errOut, "Is a directory\n") {
		t.Fatalf("stderr %q does not say Is a directory", errOut)
	}
}

// TestDashRAndDashRDifferOnSymlinks: GNU -r never reads through a link, -R does.
// This is the difference an agent relies on when it does not want to walk into
// a mounted tree twice.
func TestDashRAndDashRDifferOnSymlinks(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "linkdir")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "inner.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	top := filepath.Join(base, "top")
	if err := os.MkdirAll(filepath.Join(top, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(top, "plain.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "linkdir"), filepath.Join(top, "real", "deep")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}

	// -r reads the plain file and stops at the link, so the target directory
	// contributes only when it is named on the command line itself.
	out, errOut, code := runGrep(t, []string{"-r", "x", top, real}, "")
	if code != 0 {
		t.Fatalf("grep -r = %d, stderr %q", code, errOut)
	}
	if got := strings.Count(out, "inner.txt:x"); got != 1 {
		t.Fatalf("grep -r found inner.txt %d times, want 1 (no walk through the link): %q", got, out)
	}

	// -R follows the link, so the same target is reached twice.
	out, _, code = runGrep(t, []string{"-R", "x", top, real}, "")
	if code != 0 {
		t.Fatalf("grep -R = %d", code)
	}
	if got := strings.Count(out, "inner.txt:x"); got != 2 {
		t.Fatalf("grep -R found inner.txt %d times, want 2 (link followed): %q", got, out)
	}
	if !strings.Contains(out, "deep/inner.txt:x") {
		t.Fatalf("grep -R = %q does not name the path through the link", out)
	}
}

// TestLongOptionNamesMatchTheirShortSpelling: an agent writes --only-matching as
// often as -o, so both spellings have to end in the same output.
func TestLongOptionNamesMatchTheirShortSpelling(t *testing.T) {
	input := "alpha\nBeta\nalpha beta\n"
	pairs := [][2]string{
		{"-i", "--ignore-case"},
		{"-v", "--invert-match"},
		{"-n", "--line-number"},
		{"-o", "--only-matching"},
		{"-w", "--word-regexp"},
		{"-c", "--count"},
		{"-q", "--quiet"},
		{"-h", "--no-filename"},
		{"-E", "--extended-regexp"},
		{"-F", "--fixed-strings"},
	}
	for _, pair := range pairs {
		shortOut, shortErr, shortCode := runGrep(t, []string{pair[0], "a.ph"}, input)
		longOut, longErr, longCode := runGrep(t, []string{pair[1], "a.ph"}, input)
		if shortCode != longCode || shortOut != longOut || shortErr != longErr {
			t.Errorf("%s = %d/%q against %s = %d/%q", pair[0], shortCode, shortOut, pair[1], longCode, longOut)
		}
	}
}

// TestIgnoreCaseIsAbsolute: -i keeps the line as written, only the comparison
// ignores case. Checked against GNU grep 3.11 ("grep -i bravo" on "Bravo").
func TestIgnoreCaseIsAbsolute(t *testing.T) {
	out, _, code := runGrep(t, []string{"-i", "bravo"}, "Bravo\nzenix\n")
	if code != 0 || out != "Bravo\n" {
		t.Fatalf("grep -i = %d, %q", code, out)
	}
}

// TestFilesWithMatchesNamesOnly: -l is what an agent greps for when it wants the
// file, not the line.
func TestFilesWithMatchesNamesOnly(t *testing.T) {
	dir := t.TempDir()
	hit := filepath.Join(dir, "hit.txt")
	miss := filepath.Join(dir, "miss.txt")
	if err := os.WriteFile(hit, []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(miss, []byte("beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := runGrep(t, []string{"-l", "alpha", hit, miss}, "")
	if code != 0 || out != hit+"\n" {
		t.Fatalf("grep -l = %d, %q, want only %q", code, out, hit)
	}
	// -h drops the name on a normal match, so the line is the whole output.
	out, _, _ = runGrep(t, []string{"-h", "alpha", hit, miss}, "")
	if out != "alpha\n" {
		t.Fatalf("grep -h = %q, want the bare line", out)
	}
}

// TestFixedStringsAreNotPatterns: -F takes the text literally, so the dot of a
// version number does not match any character.
func TestFixedStringsAreNotPatterns(t *testing.T) {
	out, _, code := runGrep(t, []string{"-F", "1.2"}, "1.2\n1x2\n")
	if code != 0 || out != "1.2\n" {
		t.Fatalf("grep -F = %d, %q, want only the literal line", code, out)
	}
}

// TestValueOptionsInTheirEqualsForm: --context=1 and --regexp=x are the spellings
// that come out of config files and scripts.
func TestValueOptionsInTheirEqualsForm(t *testing.T) {
	input := "alpha\n1\n2\n"
	out, _, code := runGrep(t, []string{"--after-context=1", "--regexp=alpha"}, input)
	if code != 0 || out != "alpha\n1\n" {
		t.Fatalf("--after-context=1 --regexp=alpha = %d, %q", code, out)
	}
	out, _, _ = runGrep(t, []string{"--context=1", "alpha"}, input)
	if want := "alpha\n1\n"; out != want {
		t.Fatalf("--context=1 = %q, want %q", out, want)
	}
	out, _, _ = runGrep(t, []string{"-e", "alpha", "-e", "2"}, "alpha\n2\n")
	if out != "alpha\n2\n" {
		t.Fatalf("-e twice = %q", out)
	}
}

// TestBadContextLengthIsTwo: "-A x" must not silently mean zero lines.
func TestBadContextLengthIsTwo(t *testing.T) {
	for _, args := range [][]string{
		{"-A", "x", "alpha"},
		{"-C", "-1", "alpha"},
		{"--context=abc", "alpha"},
	} {
		_, errOut, code := runGrep(t, args, "alpha\n")
		if code != 2 {
			t.Errorf("%v: exit %d, want 2 (stderr %q)", args, code, errOut)
			continue
		}
		if !strings.Contains(errOut, "invalid context length") {
			t.Errorf("%v: stderr %q does not name the reason", args, errOut)
		}
	}
}

// TestMissingOptionValueIsTwo: "-e" at the end has nothing to take.
func TestMissingOptionValueIsTwo(t *testing.T) {
	_, errOut, code := runGrep(t, []string{"-e"}, "alpha\n")
	if code != 2 || !strings.Contains(errOut, "needs a value") {
		t.Fatalf("grep -e = %d, %q", code, errOut)
	}
}

// TestBrokenPatternIsTwo: a bad expression is a usage error, not "no match".
func TestBrokenPatternIsTwo(t *testing.T) {
	_, errOut, code := runGrep(t, []string{"["}, "alpha\n")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.HasPrefix(errOut, "grep: ") {
		t.Fatalf("stderr %q has no message", errOut)
	}
}

// TestFilesWithMatchesOnAStreamAndOnAMiss: -l names stdin "(standard input)"
// and says 1 when nothing matched, as GNU does.
func TestFilesWithMatchesOnAStreamAndOnAMiss(t *testing.T) {
	out, _, code := runGrep(t, []string{"-l", "alpha"}, "alpha\n")
	if code != 0 || out != "(standard input)\n" {
		t.Fatalf("grep -l on stdin = %d, %q", code, out)
	}
	out, _, code = runGrep(t, []string{"-l", "zenix"}, "alpha\n")
	if code != 1 || out != "" {
		t.Fatalf("grep -l miss = %d, %q, want 1 and no output", code, out)
	}
}

// TestPerlRegexp: -P has to reach PCRE, not RE2. These three patterns are the
// ones RE2 refuses and GNU grep -P accepts (docs/pcre-recherche.md).
func TestPerlRegexp(t *testing.T) {
	cases := []struct {
		name, pattern, input, want string
	}{
		{"lookahead", `foo(?=bar)`, "foobar\nfoobaz\n", "foobar\n"},
		{"k-reset", `a\Kb`, "xab\n", "xab\n"},
		{"possessive", `a++b`, "aaab\n", "aaab\n"},
	}
	for _, c := range cases {
		out, _, code := runGrep(t, []string{"-P", c.pattern}, c.input)
		if code != 0 || out != c.want {
			t.Errorf("-P %q (%s) = %d, %q, want %q", c.pattern, c.name, code, out, c.want)
		}
	}
	// Without -P the same lookahead must be an error, exactly as RE2 behaves.
	_, errOut, code := runGrep(t, []string{`foo(?=bar)`}, "foobar\n")
	if code != 2 || !strings.Contains(errOut, "invalid or unsupported") && !strings.Contains(errOut, "missing") && !strings.Contains(errOut, "error parsing") {
		t.Errorf("lookahead without -P = %d, %q; want exit 2 with a pattern error", code, errOut)
	}
}

// TestPerlRegexpOnlyMatch: -o with -P prints the PCRE match, including the part
// \K moved the start of.
func TestPerlRegexpOnlyMatch(t *testing.T) {
	out, _, code := runGrep(t, []string{"-P", "-o", `a\Kb`}, "xab yab\n")
	if code != 0 || out != "b\nb\n" {
		t.Fatalf("grep -P -o = %d, %q", code, out)
	}
}

// TestPerlRegexpBadPattern: a broken PCRE pattern is exit 2, like GNU.
func TestPerlRegexpBadPattern(t *testing.T) {
	_, errOut, code := runGrep(t, []string{"-P", "a(b"}, "ab\n")
	if code != 2 || errOut == "" {
		t.Fatalf("grep -P a(b = %d, %q; want exit 2 and a message", code, errOut)
	}
}
