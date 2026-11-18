package httpd

import (
	"strings"
	"testing"
)

// TestDetachCmdNameForTheMulticall: inside the acmesh binary the dispatcher has
// already replaced os.Args[0] with the command name, so the child needs the
// path of the running binary plus that name. Without the name the child would
// start the wrong command, and without the path exec searches PATH for "httpd",
// which is the bug in acmesh-rds.
func TestDetachCmdNameForTheMulticall(t *testing.T) {
	got := detachArgv("/root/acmesh/acmesh", "httpd", []string{"-p", "127.0.0.1:8099", "-h", "."})
	want := []string{"httpd", "-f", "-p", "127.0.0.1:8099", "-h", "."}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %q want %q", got, want)
	}
}

// TestDetachStandaloneNeedsNoName: a binary that is not a multicall binary
// (built from cmd/httpd alone, or renamed by hand) is its own command, and
// adding "httpd" as an argument would make it look for a file called httpd.
func TestDetachStandaloneNeedsNoName(t *testing.T) {
	got := detachArgv("/tmp/hd", "/tmp/hd", []string{"-p", "8099"})
	want := []string{"-f", "-p", "8099"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %q want %q", got, want)
	}
}

// TestDetachKeepsForegroundFlagOut: the child must not receive a second -f if
// the caller already gave one, or the flag list grows on every restart.
func TestDetachKeepsForegroundFlagOut(t *testing.T) {
	got := detachArgv("/root/acmesh/acmesh", "httpd", []string{"-f", "-p", "8099"})
	want := []string{"httpd", "-f", "-p", "8099"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %q want %q", got, want)
	}
}
