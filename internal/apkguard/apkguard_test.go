package apkguard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gurucoyote/akmesh/internal/apkreport"
)

// fakeApk writes a shell script that behaves like apk for these tests: it logs
// every call, bumps the installed version on "upgrade" and restores it on
// "add pkg=version".
func fakeApk(t *testing.T) (apk string, log string) {
	t.Helper()
	dir := t.TempDir()
	apk = filepath.Join(dir, "apk")
	log = filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
echo "$*" >> ` + log + `
db="$2/lib/apk/db/installed"
case "$*" in
  *upgrade*)  sed -i 's/^V:1.0.0-r0/V:2.0.0-r0/' "$db" ;;
  *"add rotest=1.0.0-r0"*) sed -i 's/^V:2.0.0-r0/V:1.0.0-r0/' "$db" ;;
esac
exit 0
`
	if err := os.WriteFile(apk, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return apk, log
}

func tree(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "lib/apk/db"), 0o755)
	os.MkdirAll(filepath.Join(dir, "etc/apk"), 0o755)
	db := "P:rotest\nV:" + version + "\nA:x86_64\nS:1\nI:1\n\n"
	os.WriteFile(filepath.Join(dir, "lib/apk/db/installed"), []byte(db), 0o644)
	os.WriteFile(filepath.Join(dir, "etc/apk/world"), []byte("rotest@stable\n"), 0o644)
	return dir
}

func reportWithGood(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bericht.jsonl")
	os.WriteFile(path, []byte(`{"zeit":"2026-11-24T10:00:00Z","host":"h","paket":"rotest","version":"1.0.0-r0","ergebnis":"ok","health":"ok"}`+"\n"), 0o644)
	return path
}

// TestRollbackOnceThenStandStill is the promise from the ticket: exactly one
// rollback, then a report and no further automatic action.
func TestRollbackOnceThenStandStill(t *testing.T) {
	apk, log := fakeApk(t)
	root := tree(t, "1.0.0-r0")
	rep := reportWithGood(t)
	var out, errb bytes.Buffer

	code := Run([]string{"-p", root, "-o", rep, "--apk", apk, "--health", "exit 1"}, &out, &errb)
	if code == 0 {
		t.Fatalf("gescheiterter Health-Check muss Exit 1 geben: %s", errb.String())
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "add rotest=1.0.0-r0") {
		t.Fatalf("kein Rueckfall aufgerufen: %s", calls)
	}
	// Die Version steht wieder auf der alten.
	got := apkreport.LastGood(rep)
	if got["rotest"] != "1.0.0-r0" {
		t.Fatalf("Rueckfall nicht wirksam: %v", got)
	}
	if st := readState(rep + ".state"); !st.RollbackUsed {
		t.Fatalf("Zustand merkt den Rueckfall nicht: %+v", st)
	}

	// Zweiter Lauf, wieder kranker Health-Check: kein zweiter Rueckfall.
	os.WriteFile(log, nil, 0o644)
	os.WriteFile(filepath.Join(root, "lib/apk/db/installed"), []byte("P:rotest\nV:2.0.0-r0\nA:x86_64\nS:1\nI:1\n\n"), 0o644)
	var out2, errb2 bytes.Buffer
	code = Run([]string{"-p", root, "-o", rep, "--apk", apk, "--health", "exit 1"}, &out2, &errb2)
	if code == 0 {
		t.Fatal("zweiter Lauf muss weiterhin Exit 1 geben")
	}
	calls2, _ := os.ReadFile(log)
	if strings.Contains(string(calls2), "add rotest=") {
		t.Fatalf("zweiter Rueckfall trotz Zustand: %s", calls2)
	}
	if !strings.Contains(errb2.String(), "standing still") {
		t.Fatalf("Meldung fehlt: %s", errb2.String())
	}
}

func TestHealthyRunReportsOk(t *testing.T) {
	apk, _ := fakeApk(t)
	root := tree(t, "1.0.0-r0")
	rep := reportWithGood(t)
	var out, errb bytes.Buffer
	if code := Run([]string{"-p", root, "-o", rep, "--apk", apk, "--health", "true"}, &out, &errb); code != 0 {
		t.Fatalf("gesunder Lauf muss Exit 0 geben: %s", errb.String())
	}
	body, _ := os.ReadFile(rep)
	last := string(body[bytes.LastIndexByte(body[:len(body)-1], '\n')+1:])
	if !strings.Contains(last, `"health":"ok"`) {
		t.Fatalf("Bericht ohne health ok: %s", last)
	}
	if st := readState(rep + ".state"); st.RollbackUsed {
		t.Fatal("gesunder Lauf darf keinen Rueckfall vermerken")
	}
}

// TestNothingChangedNoRollback: an upgrade that changes nothing must not trigger
// a rollback, otherwise a broken health check would roll back unrelated packages.
func TestNothingChangedNoRollback(t *testing.T) {
	apk, log := fakeApk(t)
	root := tree(t, "1.0.0-r0")
	rep := reportWithGood(t)
	var out, errb bytes.Buffer
	code := Run([]string{"-p", root, "-o", rep, "--apk", apk, "--health", "exit 1", "--no-upgrade"}, &out, &errb)
	if code == 0 {
		t.Fatal("kranker Check muss Exit 1 geben")
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "add rotest=") {
		t.Fatalf("Rueckfall ohne Aenderung: %s", calls)
	}
}

func TestReportIsRequired(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run(nil, &out, &errb); code == 0 {
		t.Fatal("ohne -o muss apkguard ablehnen")
	}
}
