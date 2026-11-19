package sortcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures and the expected outputs below were produced with GNU sort 9.x
// (debian stable-slim) in the same run that these tests were written; every
// case in this file agreed with GNU before it was recorded. The point of the
// table is that a later change cannot silently drift away from GNU.

var fixtures = map[string]string{
	"a": `b 2 x
  a 10 y
c 1 z
A 3 w
b 2 a

10 K
2 M
1 G
Jan 5
Feb 9
v1.10
v1.9
v1.2.3
Zebra
apple
`,
	"b": `3 c
1 a
2 b
10 z
`,
	"c": `Jan 5
February 9
jan 3
Mar
DEC 1
xx
`,
	"e": `-5
3.5
0.5
10
-0.5
1e3
0x10
.5
`,
	"f": `x.tar.gz
x.tar
x.a
x.c
x.o
1.0
1.0.0
1.0~rc1
a.b.c
`,
	"nul": "b\x00a\x00c\x00",
	// "h" exists for -h alone: fixture "e" has no SI suffix, so its -h case was
	// identical to -n and could not fail when the suffix order broke (acmesh-k7t).
	// The order below is GNU coreutils 9.7's output for this input.
	"h": `1K
500
2M
3G
1.5K
-4
10
1k
0
1T
1M500K
`,
}

// runSort executes the package the way the binary does: stdin, stdout and
// stderr are real files.
func runSort(t *testing.T, args []string, fixture string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in")
	if err := os.WriteFile(in, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	inF, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer inF.Close()
	outF, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer outF.Close()
	errF, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errF.Close()
	code := Run(args, inF, outF, errF)
	outF.Seek(0, 0)
	errF.Seek(0, 0)
	out, _ := os.ReadFile(outF.Name())
	errText, _ := os.ReadFile(errF.Name())
	return code, string(out), string(errText)
}

func TestSortMatchesGNU(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		fixture string
		want    string
	}{
		{name: "plain", args: []string{}, fixture: fixtures["a"], want: `
  a 10 y
1 G
10 K
2 M
A 3 w
Feb 9
Jan 5
Zebra
apple
b 2 a
b 2 x
c 1 z
v1.10
v1.2.3
v1.9
`},
		{name: "-n", args: []string{"-n"}, fixture: fixtures["e"], want: `-5
-0.5
0x10
.5
0.5
1e3
3.5
10
`},
		{name: "-r", args: []string{"-r"}, fixture: fixtures["a"], want: `v1.9
v1.2.3
v1.10
c 1 z
b 2 x
b 2 a
apple
Zebra
Jan 5
Feb 9
A 3 w
2 M
10 K
1 G
  a 10 y

`},
		{name: "-u", args: []string{"-u"}, fixture: fixtures["a"], want: `
  a 10 y
1 G
10 K
2 M
A 3 w
Feb 9
Jan 5
Zebra
apple
b 2 a
b 2 x
c 1 z
v1.10
v1.2.3
v1.9
`},
		{name: "-k2,2", args: []string{"-k2,2"}, fixture: fixtures["a"], want: `
Zebra
apple
v1.10
v1.2.3
v1.9
c 1 z
  a 10 y
b 2 a
b 2 x
A 3 w
Jan 5
Feb 9
1 G
10 K
2 M
`},
		{name: "-t   -k2,2n", args: []string{"-t", " ", "-k2,2n"}, fixture: fixtures["a"], want: `
  a 10 y
1 G
10 K
2 M
Zebra
apple
v1.10
v1.2.3
v1.9
c 1 z
b 2 a
b 2 x
A 3 w
Jan 5
Feb 9
`},
		{name: "-M", args: []string{"-M"}, fixture: fixtures["c"], want: `xx
Jan 5
jan 3
February 9
Mar
DEC 1
`},
		{name: "-V", args: []string{"-V"}, fixture: fixtures["f"], want: `1.0~rc1
1.0
1.0.0
a.b.c
x.a
x.c
x.o
x.tar
x.tar.gz
`},
		{name: "-h", args: []string{"-h"}, fixture: fixtures["e"], want: `-5
-0.5
0x10
.5
0.5
1e3
3.5
10
`},
		{name: "-h with SI suffixes", args: []string{"-h"}, fixture: fixtures["h"], want: `-4
0
10
500
1K
1k
1.5K
1M500K
2M
3G
1T
`},
		{name: "-g", args: []string{"-g"}, fixture: fixtures["e"], want: `-5
-0.5
.5
0.5
3.5
10
0x10
1e3
`},
		{name: "-k1.1,1.2", args: []string{"-k1.1,1.2"}, fixture: fixtures["a"], want: `
  a 10 y
1 G
10 K
2 M
A 3 w
Feb 9
Jan 5
Zebra
apple
b 2 a
b 2 x
c 1 z
v1.10
v1.2.3
v1.9
`},
		{name: "-s -k1,1", args: []string{"-s", "-k1,1"}, fixture: fixtures["a"], want: `
  a 10 y
1 G
10 K
2 M
A 3 w
Feb 9
Jan 5
Zebra
apple
b 2 x
b 2 a
c 1 z
v1.10
v1.2.3
v1.9
`},
		{name: "-z", args: []string{"-z"}, fixture: fixtures["nul"], want: "a\x00b\x00c\x00"},
		{name: "-k2,2 -u", args: []string{"-k2,2", "-u"}, fixture: fixtures["b"], want: `1 a
2 b
3 c
10 z
`},
		{name: "-f", args: []string{"-f"}, fixture: fixtures["a"], want: `
  a 10 y
1 G
10 K
2 M
A 3 w
apple
b 2 a
b 2 x
c 1 z
Feb 9
Jan 5
v1.10
v1.2.3
v1.9
Zebra
`},
		{name: "-d", args: []string{"-d"}, fixture: fixtures["a"], want: `
  a 10 y
1 G
10 K
2 M
A 3 w
Feb 9
Jan 5
Zebra
apple
b 2 a
b 2 x
c 1 z
v1.10
v1.2.3
v1.9
`},
	}
	for _, tc := range cases {
		t.Run(strings.ReplaceAll(tc.name, " ", "_"), func(t *testing.T) {
			code, out, errText := runSort(t, tc.args, tc.fixture)
			if code != 0 {
				t.Fatalf("exit %d, stderr %q", code, errText)
			}
			if out != tc.want {
				t.Fatalf("got\n%q\nwant\n%q", out, tc.want)
			}
		})
	}
}

func TestCheckReportsDisorder(t *testing.T) {
	code, _, errText := runSort(t, []string{"-c"}, "b\na\n")
	if code != 1 {
		t.Fatalf("-c on unsorted input: exit %d, want 1", code)
	}
	if !strings.Contains(errText, "disorder") {
		t.Fatalf("-c message lacks the GNU word \"disorder\": %q", errText)
	}
	if code, _, _ := runSort(t, []string{"-c"}, "a\nb\n"); code != 0 {
		t.Fatalf("-c on sorted input: exit %d, want 0", code)
	}
	if code, _, errText := runSort(t, []string{"-C"}, "b\na\n"); code != 1 || errText != "" {
		t.Fatalf("-C: exit %d stderr %q, want 1 and silence", code, errText)
	}
}

// TestRandomSortGroupsIdenticalKeys pins the GNU meaning of -R: sort by a hash
// of the key, so identical keys stay together. A uniform permutation is shuf's
// job, and the two must not be confused.
func TestRandomSortGroupsIdenticalKeys(t *testing.T) {
	const in = "b 1\na 2\nb 3\nc 4\nb 5\n"
	var seen []string
	for attempt := 0; attempt < 5; attempt++ {
		code, out, errText := runSort(t, []string{"-R"}, in)
		if code != 0 {
			t.Fatalf("exit %d, stderr %q", code, errText)
		}
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(lines) != 5 {
			t.Fatalf("got %d lines from 5 inputs: %q", len(lines), out)
		}
		last, open := "", map[string]bool{}
		for _, l := range lines {
			k := l[:1]
			if k != last {
				if open[k] {
					t.Fatalf("-R split a group of equal keys: %q", out)
				}
				open[last] = true
				last = k
			}
		}
		seen = append(seen, out)
	}
	same := true
	for _, s := range seen {
		if s != seen[0] {
			same = false
		}
	}
	if same {
		t.Fatalf("-R produced the same order five times: is the seed random?")
	}
}

func TestLocaleOtherThanCIsRefused(t *testing.T) {
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	code, _, errText := runSort(t, nil, "b\na\n")
	if code != 2 {
		t.Fatalf("exit %d, want 2 for an unsupported locale", code)
	}
	if !strings.Contains(errText, "C locale") {
		t.Fatalf("message does not say what is supported: %q", errText)
	}
}

func TestUsageErrorsExit2(t *testing.T) {
	for _, args := range [][]string{{"--bogus"}, {"-k0"}, {"-t", "ab"}, {"--debug"}} {
		if code, _, errText := runSort(t, args, "a\n"); code != 2 {
			t.Errorf("%v: exit %d, want 2 (stderr %q)", args, code, errText)
		}
	}
}
