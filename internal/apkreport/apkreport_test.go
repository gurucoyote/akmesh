package apkreport

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// root writes a target tree with an install database and a world file.
func root(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "lib/apk/db"), 0o755)
	os.MkdirAll(filepath.Join(dir, "etc/apk"), 0o755)
	db := `C:Q1aaa=
P:rotest
V:2.0.0-r0
A:x86_64
S:966
I:28
T:rollout test
o:rotest

C:Q1bbb=
P:anderes
V:1.0.0-r0
A:x86_64
S:100
I:10
T:anderes
o:anderes

`
	os.WriteFile(filepath.Join(dir, "lib/apk/db/installed"), []byte(db), 0o644)
	os.WriteFile(filepath.Join(dir, "etc/apk/world"), []byte("rotest@stable\nanderes=1.0.0-r0\n"), 0o644)
	return dir
}

func lines(t *testing.T, file string) []Line {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var out []Line
	for raw := range bytes.SplitSeq(bytes.TrimSpace(data), []byte("\n")) {
		var l Line
		if err := json.Unmarshal(raw, &l); err != nil {
			t.Fatalf("Zeile ist kein JSON: %q (%v)", raw, err)
		}
		out = append(out, l)
	}
	return out
}

func TestReportWritesOneLinePerPackage(t *testing.T) {
	dir := root(t)
	out := filepath.Join(t.TempDir(), "bericht.jsonl")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-p", dir, "-o", out, "--host", "kunde-1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got := lines(t, out)
	if len(got) != 2 {
		t.Fatalf("%d Zeilen, erwartet 2", len(got))
	}
	// Sortiert nach Paket: anderes, rotest.
	if got[0].Paket != "anderes" || got[1].Paket != "rotest" {
		t.Fatalf("Reihenfolge: %+v", got)
	}
	if got[1].Version != "2.0.0-r0" || got[1].Host != "kunde-1" {
		t.Fatalf("Felder: %+v", got[1])
	}
	if got[1].Kanal != "stable" {
		t.Fatalf("Kanal aus world: %q", got[1].Kanal)
	}
	if got[0].Kanal != "" {
		t.Fatalf("Pin ist kein Kanal: %q", got[0].Kanal)
	}
	if got[1].Ergebnis != "n/a" || got[1].Health != "skip" {
		t.Fatalf("ohne apk-Lauf: ergebnis=%q health=%q", got[1].Ergebnis, got[1].Health)
	}
}

// TestVorherComesFromTheFile is the property the rollback watcher depends on:
// the same report file says what the previous known-good version was.
func TestVorherComesFromTheFile(t *testing.T) {
	dir := root(t)
	out := filepath.Join(t.TempDir(), "bericht.jsonl")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-p", dir, "-o", out, "--health", "ok", "--apk-exit", "0"}, &stdout, &stderr); code != 0 {
		t.Fatalf("erster Lauf: %s", stderr.String())
	}
	// Neue Version eingespielt.
	db := filepath.Join(dir, "lib/apk/db/installed")
	body, _ := os.ReadFile(db)
	os.WriteFile(db, bytes.Replace(body, []byte("V:2.0.0-r0"), []byte("V:3.0.0-r0"), 1), 0o644)

	if code := Run([]string{"-p", dir, "-o", out, "--health", "fail", "--apk-exit", "0"}, &stdout, &stderr); code != 0 {
		t.Fatalf("zweiter Lauf: %s", stderr.String())
	}
	got := lines(t, out)
	if len(got) != 4 {
		t.Fatalf("%d Zeilen, erwartet 4", len(got))
	}
	newest := got[3]
	if newest.Paket != "rotest" || newest.Version != "3.0.0-r0" {
		t.Fatalf("neueste Zeile: %+v", newest)
	}
	if newest.Vorher != "2.0.0-r0" {
		t.Fatalf("vorher: %q, erwartet 2.0.0-r0", newest.Vorher)
	}
	if newest.Ergebnis != "ok" || newest.Health != "fail" {
		t.Fatalf("ergebnis=%q health=%q", newest.Ergebnis, newest.Health)
	}
	if newest.ApkExit == nil || *newest.ApkExit != 0 {
		t.Fatalf("apk_exit fehlt: %+v", newest)
	}
}

func TestHealthValueIsChecked(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-p", t.TempDir(), "--health", "vielleicht"}, &stdout, &stderr); code == 0 {
		t.Fatal("falscher Health-Wert akzeptiert")
	}
}
