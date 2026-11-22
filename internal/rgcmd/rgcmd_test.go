package rgcmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// write puts a file under root, creating parents.
func write(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture is one tree for every case: plain text, nesting, a dot-file, a
// gitignore and a Go file.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "a.txt", "alpha\nbeta alpha\ngamma\n")
	write(t, dir, "sub/b.txt", "alpha\nbeta\n")
	write(t, dir, ".hidden.txt", "alpha\n")
	write(t, dir, "ignored.log", "alpha\n")
	write(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	write(t, dir, ".gitignore", "*.log\n")
	// rg applies .gitignore only inside a repository; without .git both it and
	// this command search the ignored file.
	write(t, dir, ".git/HEAD", "ref: refs/heads/main\n")
	return dir
}

// runOurs runs the command in dir and returns output lines, stderr and code.
func runOurs(t *testing.T, dir string, args []string) (out string, stderr string, code int) {
	t.Helper()
	t.Chdir(dir)
	var o, e bytes.Buffer
	code = Run(args, strings.NewReader(""), &o, &e)
	return o.String(), e.String(), code
}

// rgBinary returns the ripgrep on PATH, or skips with a message that says the
// comparison did not happen. A silently green test would claim a parity nobody
// checked.
func rgBinary(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("rg")
	if err != nil {
		t.Skip("no ripgrep on PATH: the comparison against rg did not run")
	}
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		t.Skipf("rg %s did not answer --version: %v", path, err)
	}
	t.Logf("comparing against %s", strings.SplitN(string(out), "\n", 2)[0])
	return path
}

func runRG(t *testing.T, rg, dir string, args []string) (out string, code int) {
	t.Helper()
	cmd := exec.Command(rg, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("")
	var o bytes.Buffer
	cmd.Stdout = &o
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return o.String(), ee.ExitCode()
		}
		t.Fatalf("run rg %q: %v", args, err)
	}
	return o.String(), 0
}

func sortedLines(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// TestMatchesRipgrep is the acceptance probe from acmesh-55u: the same calls
// through both binaries, output compared as a sorted set (the walk is parallel,
// so order is not part of the contract) plus the exit code.
func TestMatchesRipgrep(t *testing.T) {
	rg := rgBinary(t)
	dir := fixture(t)

	cases := [][]string{
		{"alpha", "."},
		{"-n", "alpha", "."},
		{"-c", "alpha", "."},
		{"-l", "alpha", "."},
		{"-F", "a.txt", "."},
		{"-w", "alpha", "."},
		{"-i", "ALPHA", "a.txt"},
		{"-S", "Alpha", "a.txt"},
		{"-v", "-n", "alpha", "a.txt"},
		{"-A1", "beta", "a.txt"},
		{"-B1", "gamma", "a.txt"},
		{"-C1", "beta", "a.txt"},
		{"-m1", "alpha", "a.txt"},
		{"-o", `a\w+`, "."},
		{"-x", "alpha", "a.txt"},
		{"-g", "*.txt", "alpha", "."},
		{"-g", "!*.txt", "alpha", "."},
		{"-t", "go", "func", "."},
		{"-d", "1", "alpha", "."},
		{"--hidden", "alpha", "."},
		{"--no-ignore", "alpha", "."},
		{"--files", "."},
		{"--files", "-g", "*.go", "."},
		{"-e", "alpha", "-e", "gamma", "a.txt"},
		{"alpha", "a.txt", "sub/b.txt"},
		{"zzz", "."},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			got, _, gotCode := runOurs(t, dir, args)
			want, wantCode := runRG(t, rg, dir, args)
			if gotCode != wantCode {
				t.Fatalf("rg %q exit %d, ours %d (ours stderr above)\nours: %s", args, wantCode, gotCode, got)
			}
			if sortedLines(got) != sortedLines(want) {
				t.Fatalf("rg %q\ngot:\n%s\nwant:\n%s", args, got, want)
			}
		})
	}
}

// TestExitCodes pins 0, 1 and 2 without needing rg.
func TestExitCodes(t *testing.T) {
	dir := fixture(t)
	if _, _, code := runOurs(t, dir, []string{"alpha", "a.txt"}); code != 0 {
		t.Fatalf("match: exit %d, want 0", code)
	}
	if out, _, code := runOurs(t, dir, []string{"zzz", "a.txt"}); code != 1 || out != "" {
		t.Fatalf("no match: exit %d output %q, want 1 and empty", code, out)
	}
	if _, stderr, code := runOurs(t, dir, []string{"[", "a.txt"}); code != 2 || stderr == "" {
		t.Fatalf("broken pattern: exit %d stderr %q, want 2 with a message", code, stderr)
	}
	if _, stderr, code := runOurs(t, dir, []string{"alpha", "does-not-exist"}); code != 2 || !strings.Contains(stderr, "does-not-exist") {
		t.Fatalf("missing path: exit %d stderr %q, want 2 naming the path", code, stderr)
	}
}

// TestParseCombinesShortFlags pins the spelling an agent types by habit:
// combined letters, a value flag inside the cluster, and --flag=value.
func TestParseCombinesShortFlags(t *testing.T) {
	cfg, err := parse([]string{"-in", "-A2", "PAT", "dir"})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ignoreCase || !cfg.lineNumbers || cfg.after != 2 {
		t.Fatalf("combined flags not read: %+v", cfg)
	}
	if len(cfg.patterns) != 1 || cfg.patterns[0] != "PAT" || len(cfg.paths) != 1 || cfg.paths[0] != "dir" {
		t.Fatalf("pattern/paths wrong: %+v", cfg)
	}

	cfg, err = parse([]string{"--glob=*.go", "--max-count=3", "P"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.globs) != 1 || cfg.globs[0] != "*.go" || cfg.maxCount != 3 {
		t.Fatalf("long flags with = not read: %+v", cfg)
	}

	if _, err := parse([]string{"-Z", "P"}); err == nil {
		t.Fatal("unknown flag -Z accepted")
	}
	if _, err := parse([]string{}); err == nil {
		t.Fatal("no pattern accepted")
	}
}

// TestFilesModeTakesNoPattern: --files with a PATH must not read that PATH as
// the pattern (it did once; the walk then started at the default root).
func TestFilesModeTakesNoPattern(t *testing.T) {
	dir := fixture(t)
	out, _, code := runOurs(t, dir, []string{"--files", "."})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"./a.txt", "./sub/b.txt", "./main.go"} {
		if !strings.Contains(out, want+"\n") {
			t.Errorf("--files output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "ignored.log") {
		t.Errorf("--files listed a .gitignore'd file:\n%s", out)
	}
}
