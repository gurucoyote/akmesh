package rgcmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// zipBytes builds a zip in memory. Our command has no zip writer, so the
// fixture must come from the standard library, never from a checked-in binary.
func zipBytes(t *testing.T, files map[string][]byte, order []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range order {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func tarBytes(t *testing.T, files map[string][]byte, order []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range order {
		data := files[name]
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// archiveFixture writes a tar, a tar.gz, a zip with a nested directory and a
// single gzipped text file, all naming the same word.
func archiveFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string][]byte{
		"a.txt": []byte("hello alpha\n"),
		"b.txt": []byte("hello beta\n"),
	}
	tarData := tarBytes(t, files, []string{"a.txt", "b.txt"})
	writeBytes(t, filepath.Join(dir, "t.tar"), tarData)
	writeBytes(t, filepath.Join(dir, "t.tar.gz"), gzipBytes(t, tarData))
	zipData := zipBytes(t, map[string][]byte{
		"in/a.txt": []byte("hello inner\n"),
		"in/b.md":  []byte("hello markdown\n"),
	}, []string{"in/a.txt", "in/b.md"})
	writeBytes(t, filepath.Join(dir, "z.zip"), zipData)
	writeBytes(t, filepath.Join(dir, "c.txt.gz"), gzipBytes(t, []byte("hello compressed\n")))
	return dir
}

func writeBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestArchiveLabelInWalk is the label case: a hit in a member names the
// archive and the member, with the archive's path as the walk found it.
func TestArchiveLabelInWalk(t *testing.T) {
	dir := archiveFixture(t)
	out, stderr, code := runOurs(t, dir, []string{"-z", "hello", "."})
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	for _, want := range []string{
		"t.tar{a.txt}:hello alpha",
		"t.tar.gz{b.txt}:hello beta",
		"z.zip{in/a.txt}:hello inner",
		"c.txt.gz:hello compressed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

// TestArchiveNamedArgumentStillLabels: the archive on the command line prints
// its members with the archive name, so a hit can be resolved. ugrep drops the
// name here and prints "{a.txt}:hello"; we keep the full label on purpose.
func TestArchiveNamedArgumentStillLabels(t *testing.T) {
	dir := archiveFixture(t)
	out, stderr, code := runOurs(t, dir, []string{"-z", "hello", "t.tar"})
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(out, "t.tar{a.txt}:hello alpha") || !strings.Contains(out, "t.tar{b.txt}:hello beta") {
		t.Fatalf("member labels missing:\n%s", out)
	}
}

// TestCompressedFilePrintsBareLines: a merely compressed file is the file
// itself, not a container of members. One named file prints bare lines, as
// ugrep does.
func TestCompressedFilePrintsBareLines(t *testing.T) {
	dir := archiveFixture(t)
	out, stderr, code := runOurs(t, dir, []string{"-z", "hello", "c.txt.gz"})
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	if out != "hello compressed\n" {
		t.Fatalf("output %q, want the bare line", out)
	}
}

// TestArchiveListAndCount pins -l and -c to one line per member, always with
// the full label - measured against ugrep 7.8.5.
func TestArchiveListAndCount(t *testing.T) {
	dir := archiveFixture(t)
	out, stderr, code := runOurs(t, dir, []string{"-z", "-l", "hello", "z.zip"})
	if code != 0 {
		t.Fatalf("-l exit %d, want 0 (stderr %q)", code, stderr)
	}
	if want := "z.zip{in/a.txt}\nz.zip{in/b.md}\n"; out != want {
		t.Fatalf("-l output %q, want %q", out, want)
	}
	out, stderr, code = runOurs(t, dir, []string{"-z", "-c", "hello", "z.zip"})
	if code != 0 {
		t.Fatalf("-c exit %d, want 0 (stderr %q)", code, stderr)
	}
	if want := "z.zip{in/a.txt}:1\nz.zip{in/b.md}:1\n"; out != want {
		t.Fatalf("-c output %q, want %q", out, want)
	}
}

// TestZmaxStages: at --zmax=1 a zip inside a zip is searched as packed bytes;
// --zmax=2 opens it and the inner member's label carries both levels.
func TestZmaxStages(t *testing.T) {
	dir := t.TempDir()
	inner := zipBytes(t, map[string][]byte{"deep.txt": []byte("hello deep\n")}, []string{"deep.txt"})
	outer := zipBytes(t, map[string][]byte{
		"inner.zip": inner,
		"top.txt":   []byte("hello top\n"),
	}, []string{"inner.zip", "top.txt"})
	writeBytes(t, filepath.Join(dir, "nested.zip"), outer)

	out, _, code := runOurs(t, dir, []string{"-z", "--zmax=1", "hello", "nested.zip"})
	if code != 0 {
		t.Fatalf("zmax=1 exit %d, want 0", code)
	}
	if strings.Contains(out, "deep") {
		t.Fatalf("--zmax=1 opened the inner zip:\n%s", out)
	}
	if !strings.Contains(out, "nested.zip{top.txt}:hello top") {
		t.Fatalf("--zmax=1 lost the stored member:\n%s", out)
	}

	out, _, code = runOurs(t, dir, []string{"-z", "--zmax=2", "hello", "nested.zip"})
	if code != 0 {
		t.Fatalf("zmax=2 exit %d, want 0", code)
	}
	if !strings.Contains(out, "nested.zip{inner.zip:deep.txt}:hello deep") {
		t.Fatalf("--zmax=2 did not label the two levels:\n%s", out)
	}
}

// TestZmaxRejectsOutOfRange: ugrep accepts 1..99 and refuses both ends; a bad
// value is a parser error with exit 2, never a silent default.
func TestZmaxRejectsOutOfRange(t *testing.T) {
	dir := archiveFixture(t)
	for _, v := range []string{"0", "100", "x"} {
		_, stderr, code := runOurs(t, dir, []string{"-z", "--zmax", v, "hello", "."})
		if code != 2 {
			t.Fatalf("--zmax=%s: exit %d, want 2 (stderr %q)", v, code, stderr)
		}
		if !strings.Contains(stderr, "--zmax") {
			t.Fatalf("--zmax=%s: stderr does not name the flag: %q", v, stderr)
		}
	}
}

// TestEncryptedMemberIsReported: an encrypted zip entry must be named, never
// counted as a file without matches.
func TestEncryptedMemberIsReported(t *testing.T) {
	dir := t.TempDir()
	data := zipBytes(t, map[string][]byte{"secret.txt": []byte("hello secret\n")}, []string{"secret.txt"})
	writeBytes(t, filepath.Join(dir, "enc.zip"), markEncrypted(t, data))

	out, stderr, code := runOurs(t, dir, []string{"-z", "hello", "enc.zip"})
	if code != 1 {
		t.Fatalf("exit %d, want 1 (no hit) - stderr %q", code, stderr)
	}
	if !strings.Contains(stderr, "encrypted member") {
		t.Fatalf("no encrypted-member message: %q", stderr)
	}
	if out != "" {
		t.Fatalf("encrypted member produced output: %q", out)
	}
}

// TestBrokenArchiveIsAnError: a zip header without a readable directory is a
// read error, so the run ends with rg's exit 2 and names the file.
func TestBrokenArchiveIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeBytes(t, filepath.Join(dir, "kaputt.zip"), []byte("PK\x03\x04kaputt ohne verzeichnis\n"))
	_, stderr, code := runOurs(t, dir, []string{"-z", "hello", "kaputt.zip"})
	if code != 2 {
		t.Fatalf("exit %d, want 2 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stderr, "kaputt.zip") {
		t.Fatalf("stderr does not name the archive: %q", stderr)
	}
}

// TestPreOnMemberName is the acceptance case for the seam: the document inside
// the archive is converted because of its member name, and a text member is
// left alone. Without --pre the docx text is not found.
func TestPreOnMemberName(t *testing.T) {
	dir := t.TempDir()
	docx := docxBytes(t, "# Vertrag\n\nDer Vertrag endet am 31.12.\n")
	zipData := zipBytes(t, map[string][]byte{
		"akte/vertrag.docx": docx,
		"akte/note.txt":     []byte("Der Vertrag als Text.\n"),
	}, []string{"akte/vertrag.docx", "akte/note.txt"})
	writeBytes(t, filepath.Join(dir, "akte.zip"), zipData)

	out, stderr, code := runOurs(t, dir, []string{"-z", "--pre", "any2md", "endet am 31.12", "akte.zip"})
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(out, "akte.zip{akte/vertrag.docx}:") {
		t.Fatalf("hit does not name the member docx:\n%s", out)
	}

	out, _, code = runOurs(t, dir, []string{"-z", "endet am 31.12", "akte.zip"})
	if code != 1 || out != "" {
		t.Fatalf("without --pre: exit %d output %q, want 1 and nothing", code, out)
	}
}

// TestMemberSizeLimit: a member above the 64 MiB cap is named and skipped, so
// a huge archive cannot exhaust memory.
func TestMemberSizeLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("needs a 65 MiB fixture")
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	big := bytes.Repeat([]byte("hello big "), (maxMemberBytes/10)+10)
	if err := tw.WriteHeader(&tar.Header{Name: "big.txt", Mode: 0o644, Size: int64(len(big)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(big); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	dir := t.TempDir()
	writeBytes(t, filepath.Join(dir, "big.tar"), buf.Bytes())

	_, stderr, code := runOurs(t, dir, []string{"-z", "hello", "big.tar"})
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stderr, "member larger than") {
		t.Fatalf("no size message: %q", stderr)
	}
}

// TestMemberCountLimit checks the counter directly: a 100 000-member fixture
// would only make the test slow.
func TestMemberCountLimit(t *testing.T) {
	e := &expander{seenMembers: maxMembers, warn: func(string) {}}
	if e.takeMember("too many") {
		t.Fatal("the member limit did not stop the walk")
	}
}

// TestAgainstUgrep compares labels and exit codes on a walked tree with a tar,
// a tar.gz and a zip. It skips with a message when ugrep is absent, so the
// parity is never claimed without the reference.
func TestAgainstUgrep(t *testing.T) {
	ugrep, err := exec.LookPath("ugrep")
	if err != nil {
		t.Skip("no ugrep on PATH: the comparison against ugrep did not run")
	}
	dir := archiveFixture(t)
	out, _, code := runOurs(t, dir, []string{"-z", "hello", "."})
	theirOut, theirCode := runUgrep(t, ugrep, dir, []string{"-z", "hello", "."})
	if code != theirCode {
		t.Fatalf("exit %d, ugrep %d", code, theirCode)
	}
	ours := labelSet(out)
	theirs := labelSet(theirOut)
	if strings.Join(ours, "\n") != strings.Join(theirs, "\n") {
		t.Fatalf("labels differ\nours:\n%s\nugrep:\n%s", strings.Join(ours, "\n"), strings.Join(theirs, "\n"))
	}
}

// runUgrep runs the reference with the same arguments in dir.
func runUgrep(t *testing.T, ugrep, dir string, args []string) (string, int) {
	t.Helper()
	cmd := exec.Command(ugrep, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if ok := asExitError(err, &ee); !ok {
			t.Fatalf("ugrep: %v", err)
		}
		code = ee.ExitCode()
	}
	return out.String(), code
}

// asExitError keeps runUgrep readable without importing errors in every line.
func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

// labelSet reduces lines to the path or label before the match, sorted, so the
// walk's order does not matter.
func labelSet(out string) []string {
	var labels []string
	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		if i := strings.LastIndex(line, ":"); i >= 0 {
			line = line[:i]
		}
		labels = append(labels, strings.TrimPrefix(line, "./"))
	}
	sort.Strings(labels)
	return labels
}

// markEncrypted sets the encryption bit of every central directory entry, so
// the reader sees an encrypted member without us writing one.
func markEncrypted(t *testing.T, data []byte) []byte {
	t.Helper()
	out := append([]byte{}, data...)
	found := 0
	for i := 0; i+10 < len(out); i++ {
		if out[i] == 'P' && out[i+1] == 'K' && out[i+2] == 1 && out[i+3] == 2 {
			flags := binary.LittleEndian.Uint16(out[i+8 : i+10])
			binary.LittleEndian.PutUint16(out[i+8:i+10], flags|0x1)
			found++
		}
	}
	if found == 0 {
		t.Fatal("no central directory entry in the fixture")
	}
	return out
}
