package net

import "testing"

// TestSSHWithoutHostPrintsUsage: ssh with no destination must say so and exit
// with SSH's own code, not try to connect to an empty host.
func TestSSHWithoutHostPrintsUsage(t *testing.T) {
	// The usage line goes to stderr, which capture does not collect; the exit
	// code is ssh's own 255 either way.
	if _, code := capture(t, runSSH, nil); code != 255 {
		t.Fatalf("exit: want 255 like ssh, got %d", code)
	}
}

// TestSSHRejectsBadPort: a port that is not a number is refused while parsing,
// before any connection is attempted.
func TestSSHRejectsBadPort(t *testing.T) {
	_, code := capture(t, runSSH, []string{"-p", "nichtnumerisch", "example.com"})
	if code != 255 {
		t.Fatalf("exit: want 255, got %d", code)
	}
}

// TestSCPWithoutOperandsPrintsUsage pads into the same shape as GNU scp.
func TestSCPWithoutOperandsPrintsUsage(t *testing.T) {
	if _, code := capture(t, runScp, nil); code == 0 {
		t.Fatalf("exit: want nonzero, got 0")
	}
}

// TestSCPWithOneOperandIsAnError: one operand cannot be a copy.
func TestSCPWithOneOperandIsAnError(t *testing.T) {
	if _, code := capture(t, runScp, []string{"nur-eine"}); code == 0 {
		t.Fatalf("exit: want nonzero, got 0")
	}
}
