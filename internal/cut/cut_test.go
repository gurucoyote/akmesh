package cut

import (
	"fmt"
	"testing"
)

// TestParseList covers the cut list syntax that scripts actually use.
func TestParseList(t *testing.T) {
	cases := []struct {
		list string
		want []span
	}{
		{"1", []span{{1, 1}}},
		{"1,3", []span{{1, 1}, {3, 3}}},
		{"2-4", []span{{2, 4}}},
		{"5-", []span{{5, 1 << 30}}},
	}
	for _, c := range cases {
		got, err := parseList(c.list)
		if err != nil {
			t.Fatalf("parseList(%q): %v", c.list, err)
		}
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Fatalf("parseList(%q) = %v, want %v", c.list, got, c.want)
		}
	}
	for _, bad := range []string{"", "x", "4-2", "0"} {
		if _, err := parseList(bad); err == nil {
			t.Fatalf("parseList(%q) should fail", bad)
		}
	}
}
