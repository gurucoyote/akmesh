//go:build !windows

package sh

import (
	"os"
	"testing"
)

// TestInterrupterCancelsStatement: a signal cancels the context of the
// statement that is running, and the report says so exactly once.
func TestInterrupterCancelsStatement(t *testing.T) {
	it := newInterrupter()
	defer it.stop()

	ctx, done := it.begin()
	select {
	case <-ctx.Done():
		t.Fatal("statement context cancelled before any signal")
	default:
	}
	it.handle(os.Interrupt)
	select {
	case <-ctx.Done():
	default:
		t.Fatal("statement context was not cancelled by the signal")
	}
	if !done() {
		t.Fatal("interrupt not reported to the loop")
	}
	// The mark is consumed: a second report must be false, or the shell would
	// print a spurious newline after the next statement.
	if done() {
		t.Fatal("interrupt reported twice")
	}
}

// TestInterrupterKeepsSignalAtPrompt: a signal that arrives while the shell is
// reading a line is kept for consume, so the loop can start a fresh prompt
// instead of dying.
func TestInterrupterKeepsSignalAtPrompt(t *testing.T) {
	it := newInterrupter()
	defer it.stop()

	it.handle(os.Interrupt)
	if !it.consume() {
		t.Fatal("signal at the prompt was not recorded")
	}
	if it.consume() {
		t.Fatal("prompt interrupt consumed twice")
	}
}

// TestInterruptedBySignal: an exit status of 130 is the terminal's SIGINT, and
// the loop must not print it as an error.
func TestInterruptedBySignal(t *testing.T) {
	if interruptedBySignal(nil) {
		t.Fatal("nil error treated as an interrupt")
	}
}
