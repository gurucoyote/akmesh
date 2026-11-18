package hexdump

import (
	"strings"
	"testing"
)

func runHex(t *testing.T, stdin string, args ...string) (string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), code
}

// TestCanonicalLayout: the layout must be byte-identical to GNU's -C, because
// agents cut columns out of that output.
func TestCanonicalLayout(t *testing.T) {
	out, code := runHex(t, "ABCDEFGHIJKLMNOPq", "-C")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "00000000  41 42 43 44 45 46 47 48  49 4a 4b 4c 4d 4e 4f 50  |ABCDEFGHIJKLMNOP|\n" +
		"00000010  71                                                |q|\n" +
		"00000011\n"
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

// TestNonPrintableDots: control bytes become dots, as in GNU.
func TestNonPrintableDots(t *testing.T) {
	out, _ := runHex(t, "a\nb", "-C")
	if !strings.Contains(out, "|a.b|") {
		t.Fatalf("got %q, want |a.b|", out)
	}
	if !strings.HasSuffix(out, "00000003\n") {
		t.Fatalf("final offset missing: %q", out)
	}
}

// TestLengthLimit: -n cuts the output at that many bytes.
func TestLengthLimit(t *testing.T) {
	out, _ := runHex(t, "0123456789abcdefgh", "-C", "-n", "4")
	if !strings.Contains(out, "30 31 32 33") || strings.Contains(out, "34") {
		t.Fatalf("-n 4 = %q", out)
	}
}
