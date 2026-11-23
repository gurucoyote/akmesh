package bsdsum

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// The expected numbers come from BusyBox 1.37.0 ("busybox sum" and
// "busybox sum -s"), so they are an outside reference, not our own output.
func TestChecksumAgainstBusyBox(t *testing.T) {
	cases := []struct {
		in      string
		bsd, sv uint32
		blocks  int64
		what    string
	}{
		{"", 0, 0, 0, "empty"},
		{"hello\n", 36979, 542, 1, "hello"},
		{"abc", 16556, 294, 1, "abc without a newline"},
	}
	for _, c := range cases {
		got, blocks, err := checksum(strings.NewReader(c.in), false)
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		if got != c.bsd || blocks != c.blocks {
			t.Errorf("%s: bsd %d/%d, want %d/%d", c.what, got, blocks, c.bsd, c.blocks)
		}
		got, _, err = checksum(strings.NewReader(c.in), true)
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		if got != c.sv {
			t.Errorf("%s: sysv %d, want %d", c.what, got, c.sv)
		}
	}
}

// TestOutputShape pins the column layout scripts parse: five digit checksum,
// right aligned blocks, then the name.
func TestOutputShape(t *testing.T) {
	path := t.TempDir() + "/hello.txt"
	if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := Run("sum", []string{path}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d, %s", code, errb.String())
	}
	want := "36979     1 " + path + "\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
	// System V prints the folding sum and no padding.
	out.Reset()
	if code := Run("sum", []string{"-s", path}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d, %s", code, errb.String())
	}
	if out.String() != "542 1 "+path+"\n" {
		t.Fatalf("sysv got %q", out.String())
	}
}

// TestMissingFile: a file that does not exist is reported, not silently summed
// as empty input.
func TestMissingFile(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run("sum", []string{t.TempDir() + "/nope"}, nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "no such file") {
		t.Fatalf("no message: %q", errb.String())
	}
}
