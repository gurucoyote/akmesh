package env

import (
	"strings"
	"testing"
)

// TestEnvDedupeKeepsLast checks the rule that the last assignment wins, which
// is what makes "env A=1 A=2 cmd" print 2.
func TestEnvDedupeKeepsLast(t *testing.T) {
	env := []string{"A=1", "B=2", "A=3"}
	got := dedupeEnv(env, "A")
	want := []string{"B=2", "A=3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("dedupeEnv = %v, want %v", got, want)
	}
}
