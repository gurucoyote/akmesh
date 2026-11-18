package tools

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestCommandTableMatchesEntryPoints checks the wiring the combined binary
// depends on. The table dispatches a name to a Func, but the busybox only
// reaches that Func through cmd/<name>/main.go; a table entry without that
// package, or a main that calls Run with another name, is a command the agent
// cannot call or a command that silently runs a different tool.
func TestCommandTableMatchesEntryPoints(t *testing.T) {
	root := filepath.Join("..", "..", "cmd")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read cmd directory: %v", err)
	}

	for name := range Commands {
		path := filepath.Join(root, name, "main.go")
		src, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("the table dispatches %q but cmd/%s/main.go is missing: "+
				"the combined binary cannot reach it", name, name)
			continue
		}
		want := `tools.Run("` + name + `")`
		if !strings.Contains(string(src), want) {
			t.Errorf("cmd/%s/main.go does not call %s: a copy-paste would dispatch another command",
				name, want)
		}
	}

	for _, e := range entries {
		name := e.Name()
		if name == "ext" {
			// The standalone multicall binary itself, not a command.
			continue
		}
		if _, ok := Commands[name]; ok {
			continue
		}
		if len(netCommands) == 0 && isNetCommand(name) {
			// The network commands are registered in init from
			// netcommands_net.go and are absent in a nonet build on purpose,
			// while the package still exists.
			continue
		}
		t.Errorf("cmd/%s exists but the table has no %q: Main answers \"unknown command\"", name, name)
	}
}

func isNetCommand(name string) bool {
	switch name {
	case "curl", "wget", "ssh", "scp":
		return true
	}
	return false
}

// TestNamesIsCompleteAndSorted keeps ext --list honest: it is what an agent
// reads to find out which commands exist in this build.
func TestNamesIsCompleteAndSorted(t *testing.T) {
	names := Names()
	if len(names) != len(Commands) {
		t.Fatalf("Names() returned %d names for %d commands", len(names), len(Commands))
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("Names() is not sorted: %v", names)
	}
	for _, name := range names {
		if name == "" {
			t.Fatal("empty command name in the table")
		}
		if _, ok := Commands[name]; !ok {
			t.Fatalf("Names() lists %q which the table does not have", name)
		}
	}
}
