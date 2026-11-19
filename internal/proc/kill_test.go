package proc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runProc(t *testing.T, cmd string, args []string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errFile, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errFile.Close()
	code := Run(cmd, args, nil, out, errFile)
	o, _ := os.ReadFile(out.Name())
	e, _ := os.ReadFile(errFile.Name())
	return string(o), string(e), code
}

// TestKillNeedsAPid: kill without an operand is an error, not a no-op.
func TestKillNeedsAPid(t *testing.T) {
	_, errText, code := runProc(t, "kill", nil)
	if code == 0 {
		t.Fatalf("exit: want nonzero, got 0")
	}
	if errText == "" {
		t.Errorf("no message on stderr")
	}
}

// TestKillRejectsUnknownSignal: a name the table does not know must be refused
// instead of being sent as a guess.
func TestKillRejectsUnknownSignal(t *testing.T) {
	_, errText, code := runProc(t, "kill", []string{"-NOSUCHSIGNAL", "1"})
	if code == 0 {
		t.Fatalf("exit: want nonzero, got 0")
	}
	if !strings.Contains(strings.ToLower(errText), "signal") {
		t.Errorf("message does not name the signal: %q", errText)
	}
}

// TestKillAcceptsTheCommonSpellings is the part agents type: -9, -TERM and -s
// TERM all mean the same table entry.
func TestKillAcceptsTheCommonSpellings(t *testing.T) {
	for _, name := range []string{"9", "TERM", "SIGTERM", "15"} {
		if _, err := NormaliseSignal(name); err != nil {
			t.Errorf("NormaliseSignal(%q): %v", name, err)
		}
	}
	if _, err := NormaliseSignal("NOPE"); err == nil {
		t.Errorf("NormaliseSignal(NOPE) accepted an invented signal")
	}
}

// TestKillallWithoutMatchSaysSo: no process by that name is a report, not a
// crash.
func TestKillallWithoutMatchSaysSo(t *testing.T) {
	_, errText, code := runProc(t, "killall", []string{"akmesh-no-such-process-xyz"})
	if code == 0 {
		t.Fatalf("exit: want nonzero when nothing matched, got 0")
	}
	if errText == "" {
		t.Errorf("no message on stderr")
	}
}
