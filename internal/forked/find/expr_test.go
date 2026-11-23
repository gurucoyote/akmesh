package find

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exprTree legt einen kleinen Baum an: a.txt (3 B), empty.txt (0 B), b.log (3 B),
// sub/c.txt (2 B) und sub/deep (leer). Die Zeitstempel bleiben, wie sie beim
// Anlegen entstehen.
func exprTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"a.txt":     "aa\n",
		"empty":     "",
		"b.log":     "bb\n",
		"sub/c.txt": "x\n",
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub/deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runExpr(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb)
	return out.String(), code
}

// TestOrAndParentheses: -o bindet schwächer als -a, und Klammern gruppieren das
// so, wie GNU es tut.
func TestOrAndParentheses(t *testing.T) {
	dir := exprTree(t)
	out, code := runExpr(t, dir, dir, "-name", "*.txt", "-o", "-name", "*.log")
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.log") {
		t.Fatalf("out = %q", out)
	}
	if strings.Contains(out, "empty") {
		t.Fatalf("-o printed a non-matching file: %q", out)
	}
	out, _ = runExpr(t, dir, dir, "(", "-name", "*.txt", "-o", "-name", "*.log", ")", "-size", "-1c")
	if strings.Contains(out, "a.txt") {
		t.Fatalf("size filter ignored: %q", out)
	}
}

// TestNotAndEmpty: ! kehrt um, -empty trifft Datein und Verzeichnisse.
func TestNotAndEmpty(t *testing.T) {
	dir := exprTree(t)
	out, _ := runExpr(t, dir, dir, "!", "-type", "d")
	if strings.Contains(out, filepath.Join(dir, "sub")+"\n") {
		t.Fatalf("! did not drop the directory: %q", out)
	}
	out, _ = runExpr(t, dir, dir, "-empty")
	if !strings.Contains(out, "empty") || !strings.Contains(out, "deep") {
		t.Fatalf("-empty missed an entry: %q", out)
	}
}

// TestPrintfAndDelete: die Aktion mit Format, und -delete loescht von innen
// nach aussen, weil es -depth einschaltet.
func TestPrintfAndDelete(t *testing.T) {
	dir := exprTree(t)
	out, _ := runExpr(t, dir, dir, "-name", "a.txt", "-printf", "%p %s\\n")
	if strings.TrimSpace(out) != filepath.Join(dir, "a.txt")+" 3" {
		t.Fatalf("printf = %q", out)
	}
	if _, code := runExpr(t, dir, dir, "-name", "*.txt", "-delete"); code != 0 {
		t.Fatalf("delete exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("a.txt survived -delete")
	}
	// Die Verzeichnisse muessen noch da sein, weil das Muster nur Dateien traf.
	if _, err := os.Stat(filepath.Join(dir, "sub/deep")); err != nil {
		t.Fatalf("sub/deep went away: %v", err)
	}
}

// TestBrokenExpressionIsUsageError: ein leeres Praedikat endet mit 2.
func TestBrokenExpressionIsUsageError(t *testing.T) {
	dir := exprTree(t)
	if _, code := runExpr(t, dir, dir, "-name"); code != 2 {
		t.Fatalf("missing operand exit = %d, want 2", code)
	}
	if _, code := runExpr(t, dir, dir, "-nope", "x"); code != 2 {
		t.Fatalf("unknown predicate exit = %d, want 2", code)
	}
	if _, code := runExpr(t, dir, dir, "(", "-name", "x"); code != 2 {
		t.Fatalf("unclosed parenthesis exit = %d, want 2", code)
	}
}
