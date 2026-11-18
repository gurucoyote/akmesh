package shared

import (
	"testing"
)

// TestParseSize covers the size syntax of truncate and split.
func TestParseSize(t *testing.T) {
	if n, err := ParseSize("2K", 0); err != nil || n != 2048 {
		t.Fatalf("2K = %d, %v", n, err)
	}
	if n, err := ParseSize("+5", 10); err != nil || n != 15 {
		t.Fatalf("+5 from 10 = %d, %v", n, err)
	}
	if n, err := ParseSize("-5", 10); err != nil || n != 5 {
		t.Fatalf("-5 from 10 = %d, %v", n, err)
	}
}
