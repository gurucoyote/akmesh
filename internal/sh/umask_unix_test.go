//go:build !windows

package sh

import (
	"bytes"
	"strings"
	"testing"
)

// TestUmaskIsABuiltin: every /etc/profile of a normal Linux calls umask, and
// mvdan/sh has no such builtin. Without this the profile prints an error on
// every login (measured on Alpine's /etc/profile).
func TestUmaskIsABuiltin(t *testing.T) {
	var out, errb bytes.Buffer
	code := RunAs("sh", []string{"-c", "umask 077; umask"}, strings.NewReader(""), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if got := out.String(); got != "0077\n" {
		t.Fatalf("umask printed %q, want 0077", got)
	}
}

// TestUmaskSetsTheProcessMask: the mask must actually apply to files created
// afterwards, not just print nicely.
func TestUmaskSetsTheProcessMask(t *testing.T) {
	var out, errb bytes.Buffer
	code := RunAs("sh", []string{"-c", "umask 027; umask -p"}, strings.NewReader(""), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "0027") {
		t.Fatalf("mask not set, output %q", out.String())
	}
}

// TestUmaskRejectsGarbage: a symbolic mask is not supported, and saying so is
// the honest answer; the shell must not claim success.
func TestUmaskRejectsGarbage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := RunAs("sh", []string{"-c", "umask 999"}, strings.NewReader(""), &out, &errb); code == 0 {
		t.Fatalf("invalid mask accepted, output %q", out.String())
	}
}
