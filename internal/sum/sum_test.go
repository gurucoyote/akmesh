package sum

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runSum(t *testing.T, kind, stdin string, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(kind, args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestCheckOKAndFailed: -c verifies a list and says OK or FAILED per file, the
// two words every script greps for.
func TestCheckOKAndFailed(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	if err := os.WriteFile(good, []byte("hallo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "sums.txt")
	// The sha256 of "hallo\n" and one wrong digest for the same file.
	content := "622cb3371c1a08096eaac564fb59acccda1fcdbe13a9dd10b486e6463c8c2525  " + good + "\n" +
		"0000000000000000000000000000000000000000000000000000000000000000  " + good + "\n"
	if err := os.WriteFile(list, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runSum(t, "sha256", "", "-c", list)
	if code != 1 {
		t.Fatalf("one bad line must exit 1, got %d", code)
	}
	if !strings.Contains(out, good+": OK") {
		t.Fatalf("output %q misses the OK line", out)
	}
	if !strings.Contains(out, good+": FAILED") {
		t.Fatalf("output %q misses the FAILED line", out)
	}
	if !strings.Contains(errOut, "did NOT match") {
		t.Fatalf("stderr %q misses the warning", errOut)
	}
}

// TestCheckAllGood: a list where every line matches exits 0.
func TestCheckAllGood(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "l.txt")
	if err := os.WriteFile(list, []byte("a9993e364706816aba3e25717850c26c9cd0d89d  "+file+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := runSum(t, "sha1", "", "-c", list)
	if code != 0 || !strings.Contains(out, file+": OK") {
		t.Fatalf("-c = %d, %q", code, out)
	}
}

// TestCheckStarSeparator: GNU also writes "hex *name" for binary mode; the star
// must not become part of the file name.
func TestCheckStarSeparator(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(file, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "l.txt")
	if err := os.WriteFile(list, []byte("a9993e364706816aba3e25717850c26c9cd0d89d *"+file+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runSum(t, "sha1", "", "-c", list)
	if code != 0 {
		t.Fatalf("-c with * separator exit %d, stderr %q", code, errOut)
	}
}

// TestCheckMissingFile: a file named in the list but not on disk is a failure,
// not a silent pass.
func TestCheckMissingFile(t *testing.T) {
	dir := t.TempDir()
	list := filepath.Join(dir, "l.txt")
	missing := filepath.Join(dir, "nix.txt")
	if err := os.WriteFile(list, []byte("0000000000000000000000000000000000000000000000000000000000000000  "+missing+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := runSum(t, "sha256", "", "-c", list)
	if code != 1 || !strings.Contains(out, "FAILED open or read") {
		t.Fatalf("-c on a missing file = %d, %q", code, out)
	}
}

// TestBareRun: without -c the output keeps the "hex  name" shape.
func TestBareRun(t *testing.T) {
	out, _, code := runSum(t, "sha256", "hallo\n")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.HasPrefix(out, "622cb3371c1a08096eaac564fb59acccda1fcdbe13a9dd10b486e6463c8c2525  -") {
		t.Fatalf("output %q", out)
	}
}

// TestEmptyListNamesItsSource: GNU does not print a bare sentence when a list
// has no usable line, it names the list, and says 'standard input' for stdin.
// Pinned against GNU coreutils 9.7; without this the check counter could be
// broken (checked++ turned into checked--) and nothing noticed (acmesh-urt).
func TestEmptyListNamesItsSource(t *testing.T) {
	dir := t.TempDir()
	list := filepath.Join(dir, "leer.txt")
	if err := os.WriteFile(list, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runSum(t, "sha256", "", "-c", list)
	if code != 1 {
		t.Fatalf("exit %d, want 1: GNU treats this as a failure", code)
	}
	want := list + ": no properly formatted checksum lines found"
	if !strings.Contains(errOut, want) {
		t.Fatalf("stderr %q misses %q", errOut, want)
	}
	if strings.Contains(errOut, "sha256sum: no properly") {
		t.Fatalf("stderr %q reports the message without naming the list", errOut)
	}

	_, errOut, code = runSum(t, "sha256", "", "-c")
	if code != 1 {
		t.Fatalf("empty stdin exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "'standard input': no properly formatted checksum lines found") {
		t.Fatalf("stderr %q misses the quoted standard input name", errOut)
	}
}

// TestWarningsUseGNUPlural: the warning counts carry GNU's singular and plural
// forms, which a script may match on (acmesh-urt).
func TestWarningsUseGNUPlural(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, "one.txt")
	missing := filepath.Join(dir, "nix.txt")
	zeros := strings.Repeat("0", 64)
	if err := os.WriteFile(one, []byte(zeros+"  "+missing+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, _ := runSum(t, "sha256", "", "-c", one)
	if !strings.Contains(errOut, "WARNING: 1 listed file could not be read") {
		t.Fatalf("one unreadable file: stderr %q", errOut)
	}

	two := filepath.Join(dir, "two.txt")
	body := zeros + "  " + missing + "\n" + zeros + "  " + filepath.Join(dir, "auchnix.txt") + "\n"
	if err := os.WriteFile(two, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, _ = runSum(t, "sha256", "", "-c", two)
	if !strings.Contains(errOut, "WARNING: 2 listed files could not be read") {
		t.Fatalf("two unreadable files: stderr %q", errOut)
	}

	file := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(file, []byte("hallo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.txt")
	wrong := zeros + "  " + file + "\n" + zeros + "  " + file + "\n"
	if err := os.WriteFile(bad, []byte(wrong), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, _ = runSum(t, "sha256", "", "-c", bad)
	if !strings.Contains(errOut, "WARNING: 2 computed checksums did NOT match") {
		t.Fatalf("two mismatches: stderr %q", errOut)
	}
}

// TestNoStderrOnSuccess: a clean check says nothing on stderr. The old test
// looked at stdout only, so a stray warning would have gone unnoticed
// (acmesh-urt: an assertion on presence needs one on absence).
func TestNoStderrOnSuccess(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "l.txt")
	if err := os.WriteFile(list, []byte("a9993e364706816aba3e25717850c26c9cd0d89d  "+file+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runSum(t, "sha1", "", "-c", list)
	if code != 0 || !strings.Contains(out, file+": OK") {
		t.Fatalf("-c = %d, %q", code, out)
	}
	if errOut != "" {
		t.Fatalf("a clean check wrote to stderr: %q", errOut)
	}
}

// TestMixedListDoesNotClaimNoLines: one usable line plus one garbage line. The
// garbage gets its own sentence, and the "no properly formatted" one must stay
// away - that is the assertion that catches a broken counter (checked++ turned
// into checked--), because the counter then falls back to zero and the wrong
// sentence appears (acmesh-urt).
func TestMixedListDoesNotClaimNoLines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "mixed.txt")
	body := "a9993e364706816aba3e25717850c26c9cd0d89d  " + file + "\n" + "das ist keine Pruefzeile\n"
	if err := os.WriteFile(list, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runSum(t, "sha1", "", "-c", list)
	if code != 0 || !strings.Contains(out, file+": OK") {
		t.Fatalf("one good line must still pass: exit %d, stdout %q", code, out)
	}
	if !strings.Contains(errOut, "improperly formatted checksum line") {
		t.Fatalf("the garbage line was not reported: stderr %q", errOut)
	}
	if strings.Contains(errOut, "no properly formatted checksum lines found") {
		t.Fatalf("a list with a usable line claimed to have none: stderr %q", errOut)
	}
}

// TestUnknownOptionIsRefused pins GNU's report (acmesh-4qw): the message, the
// "Try --help" line, exit 1. Measured on coreutils 9.1:
//
//	sha256sum: unrecognized option '--bogus'
//	Try 'sha256sum --help' for more information.
func TestUnknownOptionIsRefused(t *testing.T) {
	_, errOut, code := runSum(t, "sha256", "x\n", "--bogus")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	want := "sha256sum: unrecognized option '--bogus'\nTry 'sha256sum --help' for more information.\n"
	if errOut != want {
		t.Fatalf("stderr %q, want %q", errOut, want)
	}
}
