package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommandTableMatchesEntryPoints checks the wiring the acmesh binary depends
// on. The table dispatches a name to a Func, but the binary only reaches that
// Func through cmd/<name>/main.go; a table entry without that package, or a main
// that calls Run with another name, is a command the agent cannot call or a
// command that silently runs a different tool.
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
		// Three shapes are allowed since acmesh-mq1: the main calls the table
		// (tools.Run("name")), it calls a package named after the command
		// (cp.Run(...)), or it calls such a package with the bare algorithm
		// name (sum.Run("sha256")). A copy-paste of another command's main is
		// still caught, because then none of these appear.
		accepted := []string{
			`Run("` + name + `"`,
			name + `.Run(`,
			`Run("` + strings.TrimSuffix(name, "sum") + `"`,
		}
		found := false
		for _, want := range accepted {
			if strings.Contains(string(src), want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("cmd/%s/main.go calls none of %q: a copy-paste would dispatch another command",
				name, accepted)
		}
	}

	for _, e := range entries {
		name := e.Name()
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

// TestRunUnknownCommandNamesTheProgram pins the message a mistyped command
// produces: it must name the binary the caller started, not a program that does
// not exist on disk.
func TestRunUnknownCommandNamesTheProgram(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stderr")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create stderr: %v", err)
	}
	oldStderr, oldArgs := os.Stderr, os.Args
	os.Stderr, os.Args = f, []string{"/usr/local/bin/acmesh.exe"}
	code := Run("gibtsnicht")
	os.Stderr, os.Args = oldStderr, oldArgs
	f.Close()

	if code != 2 {
		t.Fatalf("unknown command exit %d, want 2", code)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `acmesh.exe: unknown command "gibtsnicht"`
	if !strings.Contains(string(data), want) {
		t.Fatalf("message %q, want it to contain %q", data, want)
	}
}

// TestNoEmptyCommandName keeps a typo in the table from becoming a command the
// binary answers to and prints as a blank line.
func TestNoEmptyCommandName(t *testing.T) {
	for name := range Commands {
		if name == "" || strings.TrimSpace(name) != name {
			t.Fatalf("bad command name %q in the table", name)
		}
	}
}
