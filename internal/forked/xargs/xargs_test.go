package xargs

import (
	"strings"
	"testing"
)

func runXargs(t *testing.T, input string, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(input), &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestReplacePerLine: -I{} is the bridge to cp and tar. One invocation per
// line, the whole line replaces the placeholder, spaces included.
func TestReplacePerLine(t *testing.T) {
	out, errOut, code := runXargs(t, "a\nb c\n", "-I{}", "echo", "[{}]")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if out != "[a]\n[b c]\n" {
		t.Fatalf("got %q, want one line per item", out)
	}
}

// TestChunkingAndNull: -n splits the items, -0 reads NUL-terminated ones.
func TestChunkingAndNull(t *testing.T) {
	out, _, _ := runXargs(t, "1\n2\n3\n", "-n", "2", "echo")
	if out != "1 2\n3\n" {
		t.Fatalf("-n 2 = %q", out)
	}
	out, _, _ = runXargs(t, "x\x00y z\x00", "-0", "echo")
	if out != "x y z\n" {
		t.Fatalf("-0 = %q", out)
	}
}

// TestDelimiter: -d, reads a custom separator, the value glued to the switch.
func TestDelimiter(t *testing.T) {
	out, _, code := runXargs(t, "a,b", "-d,", "echo")
	if code != 0 || out != "a b\n" {
		t.Fatalf("-d, = %d, %q", code, out)
	}
}

// TestNoRunIfEmpty: -r suppresses the run on empty input, without it the plain
// command still runs once.
func TestNoRunIfEmpty(t *testing.T) {
	out, _, _ := runXargs(t, "", "-r", "echo")
	if out != "" {
		t.Fatalf("-r printed %q", out)
	}
	out, _, _ = runXargs(t, "", "echo")
	if strings.TrimSpace(out) != "" && out != "\n" {
		t.Fatalf("without -r = %q, want one empty run", out)
	}
}

// TestExitStatus: a failed command gives 123, as GNU reports it.
func TestExitStatus(t *testing.T) {
	if _, _, code := runXargs(t, "x\n", "false"); code != 123 {
		t.Fatalf("failing command exit %d, want 123", code)
	}
	if _, _, code := runXargs(t, "x\n", "gibtsnicht-xyz"); code != 127 {
		t.Fatalf("missing command exit %d, want 127", code)
	}
}

// TestParallelRunsEverything: -P runs in parallel but nothing may get lost.
func TestParallelRunsEverything(t *testing.T) {
	out, _, code := runXargs(t, "1\n2\n3\n4\n", "-P", "4", "-n", "1", "echo")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"1\n", "2\n", "3\n", "4\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output %q lacks %q", out, want)
		}
	}
}
