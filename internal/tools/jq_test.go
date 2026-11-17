package tools

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureIn runs a command with the given stdin text and returns stdout plus
// the exit code.
func captureIn(t *testing.T, fn Func, args []string, input string) (string, int) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "stdin")
	if err := os.WriteFile(in, []byte(input), 0o600); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	stdin, err := os.Open(in)
	if err != nil {
		t.Fatalf("open stdin: %v", err)
	}
	defer stdin.Close()
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
	code := fn(args, stdin, out, errFile)
	if _, err := out.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	data, err := io.ReadAll(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(data), code
}

// TestJqBasics covers the filters that show up in every pipeline.
func TestJqBasics(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		input string
		want  string
	}{
		{"field", []string{".a"}, `{"a":1}`, "1\n"},
		{"nested", []string{".a.b"}, `{"a":{"b":"tief"}}`, "\"tief\"\n"},
		{"array", []string{".a[]"}, `{"a":[1,2]}`, "1\n2\n"},
		{"object", []string{".a"}, `{"a":{"x":1}}`, "{\n  \"x\": 1\n}\n"},
		{"compact", []string{"-c", ".a"}, `{"a":{"x":1,"y":2}}`, `{"x":1,"y":2}` + "\n"},
		{"raw", []string{"-r", ".a"}, `{"a":"ohne Anfuehrungszeichen"}`, "ohne Anfuehrungszeichen\n"},
		{"raw-plus", []string{"-rc", ".a"}, `{"a":"x"}`, "x\n"},
		{"slurp", []string{"-s", "length"}, `1 2 3`, "3\n"},
		{"multi-document", []string{".a"}, `{"a":1}{"a":2}`, "1\n2\n"},
		{"null-input", []string{"-n", `{"x":1}`}, "", "{\n  \"x\": 1\n}\n"},
		{"select", []string{`.[] | select(. > 1)`}, `[1,2,3]`, "2\n3\n"},
		{"map", []string{"map(. * 2)"}, `[1,2]`, "[\n  2,\n  4\n]\n"},
		{"len", []string{`length`}, `[1,2,3]`, "3\n"},
	}
	for _, c := range cases {
		got, code := captureIn(t, runJq, c.args, c.input)
		if code != 0 {
			t.Fatalf("%s: exit %d, output %q", c.name, code, got)
		}
		if got != c.want {
			t.Fatalf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestJqVariables checks --arg, --argjson and --slurpfile, which is how a script
// avoids building a filter by string concatenation.
func TestJqVariables(t *testing.T) {
	got, _ := captureIn(t, runJq, []string{"-r", "--arg", "name", "Martin", ".x + $name"}, `{"x":"Hallo "}`)
	if got != "Hallo Martin\n" {
		t.Fatalf("--arg gave %q", got)
	}
	got, _ = captureIn(t, runJq, []string{"--argjson", "n", "5", ".a + $n"}, `{"a":2}`)
	if got != "7\n" {
		t.Fatalf("--argjson gave %q", got)
	}

	dir := t.TempDir()
	extra := filepath.Join(dir, "extra.json")
	if err := os.WriteFile(extra, []byte(`{"b":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ = captureIn(t, runJq, []string{"-c", "--slurpfile", "e", extra, ".a + ($e[0].b)"}, `{"a":1}`)
	if got != "3\n" {
		t.Fatalf("--slurpfile gave %q", got)
	}
}

// TestJqExitStatus pins -e, because a script branching on jq relies on it.
func TestJqExitStatus(t *testing.T) {
	if _, code := captureIn(t, runJq, []string{"-e", ".a"}, `{"a":true}`); code != 0 {
		t.Fatalf("truthy value gave %d, want 0", code)
	}
	if _, code := captureIn(t, runJq, []string{"-e", ".a"}, `{"a":false}`); code != 1 {
		t.Fatalf("false gave %d, want 1", code)
	}
	if _, code := captureIn(t, runJq, []string{"-e", ".a"}, `{"a":null}`); code != 1 {
		t.Fatalf("null gave %d, want 1", code)
	}
	if _, code := captureIn(t, runJq, []string{"-e", "empty"}, `{"a":1}`); code != 4 {
		t.Fatalf("no output gave %d, want 4", code)
	}
}

// TestJqRawInput checks -R and -R -s, the usual way to treat text as lines.
func TestJqRawInput(t *testing.T) {
	got, _ := captureIn(t, runJq, []string{"-R", "-r", ". + \"!\""}, "eins\nzwei\n")
	if got != "eins!\nzwei!\n" {
		t.Fatalf("-R gave %q", got)
	}
	got, _ = captureIn(t, runJq, []string{"-R", "-s", "-r", "split(\"\\n\") | length"}, "a\nb\nc\n")
	if got != "4\n" {
		t.Fatalf("-R -s gave %q (three lines plus the trailing empty one)", got)
	}
}

// TestJqErrors checks that a broken filter and broken input fail loudly with
// different codes.
func TestJqErrors(t *testing.T) {
	if _, code := captureIn(t, runJq, []string{".a |"}, `{"a":1}`); code != jqExitCompile {
		t.Fatalf("broken filter gave %d, want %d", code, jqExitCompile)
	}
	if _, code := captureIn(t, runJq, []string{".a"}, `{"a":}`); code != 2 {
		t.Fatalf("broken input gave %d, want 2", code)
	}
	if _, code := captureIn(t, runJq, []string{"-Z"}, `{}`); code != 2 {
		t.Fatalf("unknown flag gave %d, want 2", code)
	}
}

// TestJqFiles checks reading from a named file instead of stdin.
func TestJqFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.json")
	if err := os.WriteFile(path, []byte(`{"a":[1,2]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, code := captureIn(t, runJq, []string{"-c", ".a", path}, "")
	if code != 0 || got != "[1,2]\n" {
		t.Fatalf("file input gave %q, exit %d", got, code)
	}
	got, _ = captureIn(t, runJq, []string{"-c", ".a"}, `{"a":[]}`)
	if !strings.Contains(got, "[]") {
		t.Fatalf("empty array gave %q", got)
	}
}
