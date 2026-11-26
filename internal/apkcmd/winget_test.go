package apkcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wingetSearchTable is what winget prints in a German Windows; the header is
// localized, the dashed rule is the one thing to key on.
const wingetSearchTable = "Name         ID                Version \u00dcbereinstimmung Quelle\r\n" +
	"-----------------------------------------------------\r\n" +
	"jq           jqlang.jq         1.8.2                winget\r\n" +
	"celq         IvanIsCoding.celq 0.8.0   Tag: jq      winget\r\n"

func TestParseWingetLine(t *testing.T) {
	cases := []struct {
		line   string
		want   string
		wantOK bool
	}{
		{"winget", "winget", true},
		{"winget:msstore", "msstore", true},
		{" https://example.org/akmesh/windows/amd64", "", false},
		{"winget:", "", false},
	}
	for _, c := range cases {
		got, ok := parseWingetLine(c.line)
		if ok != c.wantOK || got.Source != c.want {
			t.Errorf("parseWingetLine(%q) = %q,%v; want %q,%v", c.line, got.Source, ok, c.want, c.wantOK)
		}
	}
}

func TestSplitOnGapsKeepsSingleSpacesInNames(t *testing.T) {
	got := splitOnGaps("RipGrep GNU       BurntSushi.ripgrep.GNU  16.0.0")
	want := []string{"RipGrep GNU", "BurntSushi.ripgrep.GNU", "16.0.0"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("field %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseWingetSearchIgnoresLocalizedHeader(t *testing.T) {
	rows := parseWingetSearch(wingetSearchTable)
	if len(rows) != 2 {
		t.Fatalf("rows: got %d, want 2 (%+v)", len(rows), rows)
	}
	if rows[0].ID != "jqlang.jq" || rows[0].Version != "1.8.2" || rows[0].Name != "jq" {
		t.Fatalf("first row: %+v", rows[0])
	}
}

// writeWingetStub puts a script where the package manager would be and records
// the arguments of every call.
func writeWingetStub(t *testing.T) (calls string) {
	t.Helper()
	dir := t.TempDir()
	calls = filepath.Join(dir, "calls")
	stub := filepath.Join(dir, "winget")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> " + calls + "\n" +
		"case \"$1\" in\n" +
		"search) printf '%s' '" + wingetSearchTable + "';;\n" +
		// winget export -o FILE ...: the file is the third word.
		"export) echo '{\"Sources\":[{\"Packages\":[{\"PackageIdentifier\":\"jqlang.jq\",\"Version\":\"1.8.2\"}]}]}' > \"$3\";;\n" +
		"esac\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	old := wingetCommand
	wingetCommand = stub
	t.Cleanup(func() { wingetCommand = old })
	return calls
}

// rootWithWinget builds a root whose repository file holds the given lines.
func rootWithWinget(t *testing.T, lines ...string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "etc", "apk"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "etc", "apk", "repositories"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAddWithoutApkReposHandsTheNameToWinget(t *testing.T) {
	calls := writeWingetStub(t)
	root := rootWithWinget(t, "winget")
	var out, errOut bytes.Buffer
	code := Run("apk", []string{"-p", root, "add", "jq"}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr=%q", code, errOut.String())
	}
	logged := readCalls(t, calls)
	if !strings.Contains(logged, "search jq") {
		t.Fatalf("no search through winget: %q", logged)
	}
	if !strings.Contains(logged, "install --id jqlang.jq") {
		t.Fatalf("no install of the id: %q", logged)
	}
	for _, want := range []string{"--exact", "--silent", "--disable-interactivity", "--accept-package-agreements"} {
		if !strings.Contains(logged, want) {
			t.Fatalf("install call misses %s: %q", want, logged)
		}
	}
	text := out.String()
	if !strings.Contains(text, "jq is not an apk package") {
		t.Fatalf("no line naming the source: %q", text)
	}
	if !strings.Contains(text, "changes this Windows machine") {
		t.Fatalf("no warning where it lands: %q", text)
	}
}

func TestAddWithIdGoesStraightToWinget(t *testing.T) {
	calls := writeWingetStub(t)
	root := rootWithWinget(t, "winget")
	var out, errOut bytes.Buffer
	if code := Run("apk", []string{"-p", root, "add", "MikeFarah.yq"}, nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, errOut.String())
	}
	logged := readCalls(t, calls)
	if strings.Contains(logged, "search") {
		t.Fatalf("an id must not be searched: %q", logged)
	}
	if !strings.Contains(logged, "install --id MikeFarah.yq") {
		t.Fatalf("install call: %q", logged)
	}
}

func TestSearchAsksWingetWhenItIsTheOnlySource(t *testing.T) {
	writeWingetStub(t)
	var out, errOut bytes.Buffer
	// search reads -X or the file of the real root; here the source is named on
	// the command line so no real repository is consulted.
	code := Run("apk", []string{"-X", "winget", "search", "jq"}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, errOut.String())
	}
	text := out.String()
	if !strings.Contains(text, "searching winget for \"jq\"") {
		t.Fatalf("no line naming the source: %q", text)
	}
	if !strings.Contains(text, "jqlang.jq") {
		t.Fatalf("winget's table is missing: %q", text)
	}
}

func TestChannelOnWingetLineIsRefused(t *testing.T) {
	root := rootWithWinget(t, "@stable winget")
	var out, errOut bytes.Buffer
	code := Run("apk", []string{"-p", root, "update"}, nil, &out, &errOut)
	if code == 0 {
		t.Fatalf("a channel on winget must fail; got 0, stdout=%q", out.String())
	}
	if !strings.Contains(errOut.String(), "cannot be set on winget") {
		t.Fatalf("message: %q", errOut.String())
	}
}

func TestListAllIncludesWinget(t *testing.T) {
	writeWingetStub(t)
	root := rootWithWinget(t, "winget")
	var out, errOut bytes.Buffer
	if code := Run("apk", []string{"-p", root, "list", "--all"}, nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d; stderr=%q", code, errOut.String())
	}
	text := out.String()
	if !strings.Contains(text, "jqlang.jq-1.8.2 [winget]") {
		t.Fatalf("winget's packages are missing: %q", text)
	}
}

func readCalls(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the stub winget was never called: %v", err)
	}
	return string(data)
}
