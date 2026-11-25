package apkcore

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// tarGz builds an apk data segment: a gzip'd tar with the given members, all
// with the same mode. It is enough for the extraction path under test, which
// only needs real tar entries (acmesh-vrt1).
func tarGz(t *testing.T, mode int64, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: mode, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestExtractTarWritesWholeFiles: the file lands with its content and its mode,
// and nothing of the temporary name stays behind.
func TestExtractTarWritesWholeFiles(t *testing.T) {
	root := t.TempDir()
	pkg := &Package{Name: "probe", Version: "1.0.0-r0"}
	raw := tarGz(t, 0o755, map[string]string{"bin/probe": "inhalt", "etc/probe.conf": "a=1\n"})
	opts := &ExtractOptions{Root: root, NoScripts: true}
	if err := extractTar(raw, opts, pkg); err != nil {
		t.Fatalf("extractTar: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "bin/probe"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "inhalt" {
		t.Fatalf("Inhalt ist %q, erwartet \"inhalt\"", body)
	}
	st, err := os.Stat(filepath.Join(root, "bin/probe"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o755 {
		t.Fatalf("Rechte sind %v, erwartet 0755", st.Mode().Perm())
	}
	leftovers, err := filepath.Glob(filepath.Join(root, "bin/.apk-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("Temp-Datei liegen geblieben: %v", leftovers)
	}
}

// failingReader hands out a few bytes and then breaks, the way a truncated
// package or a full disk would.
type failingReader struct{ left int }

func (f *failingReader) Read(p []byte) (int, error) {
	if f.left <= 0 {
		return 0, errors.New("Quelle bricht ab")
	}
	p[0] = 'x'
	f.left--
	return 1, nil
}

// TestWriteMemberKeepsTheOldFileWhenTheCopyFails is the reason for the whole
// change: before, the target was deleted before its replacement was complete,
// so a broken package or a full disk left nothing behind. Now the old file is
// still there, unchanged, and the temporary name is gone.
func TestWriteMemberKeepsTheOldFileWhenTheCopyFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "datei")
	if err := os.WriteFile(target, []byte("alter stand"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeMember(dir, target, 0o755, &failingReader{left: 3}); err == nil {
		t.Fatal("writeMember meldet keinen Fehler, obwohl die Quelle abbricht")
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("die alte Datei ist weg: %v", err)
	}
	if string(body) != "alter stand" {
		t.Fatalf("die alte Datei wurde angefasst: %q", body)
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".apk-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("Temp-Datei liegen geblieben: %v", leftovers)
	}
}

// TestWriteMemberReplacesRunningExecutable: a file that is being executed
// cannot be written (the kernel answers "text file busy"), but it can be
// replaced by a rename. That is what makes an update of the running binary
// possible at all, so the test pins both halves of the statement.
func TestWriteMemberReplacesRunningExecutable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("das Verhalten von rename ueber ein laufendes Binary ist ein Linux-Fall")
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep nicht gefunden")
	}
	dir := t.TempDir()
	// The copy must keep the name the binary expects: busybox dispatches on
	// argv[0] and refuses to run under an unknown name, so a copy called
	// "schlaefer" would exit at once and prove nothing.
	target := filepath.Join(dir, "sleep")
	orig, err := os.ReadFile(sleep)
	if err != nil {
		t.Skipf("sleep nicht lesbar: %v", err)
	}
	if err := os.WriteFile(target, orig, 0o755); err != nil {
		t.Fatal(err)
	}
	proc := exec.Command(target, "30")
	if err := proc.Start(); err != nil {
		t.Skipf("das kopierte Binary startet nicht: %v", err)
	}
	defer func() {
		_ = proc.Process.Kill()
		_, _ = proc.Process.Wait()
	}()
	// Wait until the kernel really executes the file, not just until the fork.
	alive := false
	for i := 0; i < 100; i++ {
		if _, err := os.Stat("/proc/" + strconv.Itoa(proc.Process.Pid)); err == nil {
			alive = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !alive {
		t.Skip("der Kopie gelang kein Lauf; ohne laufendes Binary sagt der Test nichts")
	}

	// The half that fails: in-place writing over a running binary.
	if f, err := os.OpenFile(target, os.O_WRONLY|os.O_TRUNC, 0o755); err == nil {
		_ = f.Close()
		t.Fatal("in-place-Schreiben ueber ein laufendes Binary hat geklappt; der Test braucht ein laufendes Binary")
	}

	// The half that works, and that the installer now uses.
	if err := writeMember(dir, target, 0o755, strings.NewReader("neue fassung")); err != nil {
		t.Fatalf("writeMember ueber ein laufendes Binary: %v", err)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "neue fassung" {
		t.Fatalf("Inhalt ist %q, erwartet \"neue fassung\"", body)
	}
	if err := proc.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("der laufende Prozess ist nach dem Austausch weg: %v", err)
	}
	if _, err := os.Stat("/proc/" + strconv.Itoa(proc.Process.Pid)); err != nil {
		t.Fatalf("der laufende Prozess ist nach dem Austausch verschwunden: %v", err)
	}
}
