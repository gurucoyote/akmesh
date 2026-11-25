package cmdlist

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPrintPutsDocumentsFirst is the point of the whole file: the block an agent
// is most likely to need stands at the top, and every name has its own line.
func TestPrintPutsDocumentsFirst(t *testing.T) {
	var b bytes.Buffer
	names := []string{"ls", "pdf2md", "curl", "any2md", "sh", "weird"}
	if err := Print(&b, names); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	doc := strings.Index(got, "Documents (")
	if doc < 0 {
		t.Fatalf("no documents block:\n%s", got)
	}
	for _, later := range []string{Files, Network, Shell, Other} {
		if i := strings.Index(got, later); i >= 0 && i < doc {
			t.Errorf("block %q stands before the documents block:\n%s", later, got)
		}
	}
	for _, n := range names {
		if !strings.Contains(got, "\n - "+n+"\n") && !strings.HasSuffix(got, " - "+n+"\n") {
			t.Errorf("name %q has no line of its own:\n%s", n, got)
		}
	}
	if !strings.Contains(got, "\n\n") {
		t.Errorf("no blank line between two blocks:\n%s", got)
	}
}

// TestUnknownNameLandsInOther: the list must never lose a command, because a
// name that is missing from the table is still a command the binary answers to.
func TestUnknownNameLandsInOther(t *testing.T) {
	titles, blocks := ByGroup([]string{"gibtsnicht"})
	if len(titles) != 1 || titles[0] != Other {
		t.Fatalf("titles %q, want just Other", titles)
	}
	if len(blocks[0]) != 1 || blocks[0][0] != "gibtsnicht" {
		t.Fatalf("block %q", blocks[0])
	}
}

// TestEveryCommandIsPlaced is the gate: every command this build can carry has a
// decided block. The lists are read from the same two places the build reads
// them from, so a new command in cmds.txt or OURS_ALL cannot appear silently at
// the end of the list.
func TestEveryCommandIsPlaced(t *testing.T) {
	names := append(readCmdsTxt(t), readOursAll(t)...)
	if len(names) < 50 {
		t.Fatalf("read only %d names, the lists were not found", len(names))
	}
	if missing := Unplaced(names); len(missing) > 0 {
		t.Fatalf("no block decided for %s: add them to groups in cmdlist.go",
			strings.Join(missing, ", "))
	}
}

func readCmdsTxt(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "cmds.txt"))
	if err != nil {
		t.Fatalf("read cmds.txt: %v", err)
	}
	var out []string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// A line may start with "linux:", the marker for u-root commands that
		// only build for Linux (see scripts/cmds.txt). The marker is not a name.
		line = strings.TrimPrefix(line, "linux:")
		out = append(out, strings.Fields(line)...)
	}
	return out
}

func readOursAll(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "build-bb.sh"))
	if err != nil {
		t.Fatalf("read build-bb.sh: %v", err)
	}
	m := regexp.MustCompile(`(?m)^OURS_ALL="(.*)"$`).FindSubmatch(data)
	if m == nil {
		t.Fatal("OURS_ALL not found in build-bb.sh")
	}
	return strings.Fields(string(m[1]))
}

// TestGroupIDsAndNamesIn pins the build side of the table: a profile drops a
// whole block by id, and an id that is not in the table must be refused
// (acmesh-17s).
func TestGroupIDsAndNamesIn(t *testing.T) {
	seen := map[string]bool{}
	for _, title := range order {
		id := GroupID(title)
		if id == "" {
			t.Errorf("no id for %q", title)
			continue
		}
		if seen[id] {
			t.Errorf("duplicate id %q", id)
		}
		seen[id] = true
		if !KnownGroupID(id) {
			t.Errorf("%q is not known", id)
		}
	}

	names, ok := NamesIn("services")
	if !ok {
		t.Fatal("services is not a known group")
	}
	found := false
	for _, n := range names {
		if n == "login" {
			found = true
		}
		if groups[n] != Services {
			t.Errorf("%q is listed under services but sits in %q", n, groups[n])
		}
	}
	if !found {
		t.Fatalf("services = %v, login is missing", names)
	}
	if _, ok := NamesIn("no-such-group"); ok {
		t.Fatal("an unknown group must be refused")
	}
}
