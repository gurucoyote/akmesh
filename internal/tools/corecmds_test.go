package tools

import (
	"os"
	"strings"
	"testing"
)

// TestEchoMatchesGNU pins the difference that made us write our own echo:
// GNU echo prints a backslash sequence literally unless -e is given. u-root's
// echo interprets escapes by default, so taking that one would have changed
// script output silently.
func TestEchoMatchesGNU(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		input string
		want  string
	}{
		{"plain", []string{"hallo", "welt"}, "", "hallo welt\n"},
		{"no-newline", []string{"-n", "x"}, "", "x"},
		{"default-keeps-backslash", []string{`a\nb`}, "", `a\nb` + "\n"},
		{"escape-newline", []string{"-e", `a\nb`}, "", "a\nb\n"},
		{"escape-tab", []string{"-e", `a\tb`}, "", "a\tb\n"},
		{"escape-octal", []string{"-e", `x\0101y`}, "", "xAy\n"},
		{"escape-hex", []string{"-e", `x\x41y`}, "", "xAy\n"},
		{"stop-with-c", []string{"-e", `a\cb`}, "", "a"},
		{"E-after-e", []string{"-e", "-E", `a\nb`}, "", `a\nb` + "\n"},
		{"dashdash-operand", []string{"-e", "-"}, "", "-\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, code := captureIn(t, runEcho, c.args, c.input)
			if code != 0 {
				t.Fatalf("exit %d", code)
			}
			if out != c.want {
				t.Fatalf("got %q, want %q", out, c.want)
			}
		})
	}
}

// TestPwdPrintsTheWorkingDirectory checks the one thing an agent needs: the
// path it is actually in, with -P resolving symlinks.
func TestPwdPrintsTheWorkingDirectory(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	out, code := captureIn(t, runPwd, nil, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.TrimSpace(out) != wd {
		t.Fatalf("pwd printed %q, want %q", strings.TrimSpace(out), wd)
	}
	out, code = captureIn(t, runPwd, []string{"-P"}, "")
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("pwd -P: exit %d, output %q", code, out)
	}
}

func TestPwdRejectsUnknownOption(t *testing.T) {
	_, code := captureIn(t, runPwd, []string{"-Z"}, "")
	if code != 2 {
		t.Fatalf("pwd -Z exit %d, want 2", code)
	}
}

// TestTrueAndFalseExitCodes keeps the two commands honest: a script branches on
// exactly these codes.
func TestTrueAndFalseExitCodes(t *testing.T) {
	if _, code := captureIn(t, runTrue, nil, ""); code != 0 {
		t.Fatalf("true exit %d, want 0", code)
	}
	if _, code := captureIn(t, runFalse, nil, ""); code != 1 {
		t.Fatalf("false exit %d, want 1", code)
	}
}
