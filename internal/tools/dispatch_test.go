package tools

import (
	"reflect"
	"strings"
	"testing"
)

// TestNoCommandSharesAnotherImplementation catches the copy-paste the
// entry-point test cannot see: cmd/stat/main.go may call Run("stat") correctly
// while the table maps "stat" to runSync. Every assertion in registry_test.go
// passes and the binary runs the wrong tool.
//
// reflect gives the code pointer of a function, so closures built from one
// literal compare equal. The checksums are such a family (runSum(kind)); they
// are covered by TestSumsAreWiredToTheirOwnAlgorithm instead of being waved
// through here.
func TestNoCommandSharesAnotherImplementation(t *testing.T) {
	aliases := map[string]string{
		"gosh": "sh", // gosh is the shell under the name the old command list carried
	}
	sums := map[string]bool{"md5sum": true, "sha1sum": true, "sha256sum": true, "sha512sum": true}

	owner := map[uintptr]string{}
	for name, fn := range Commands {
		ptr := reflect.ValueOf(fn).Pointer()
		first, ok := owner[ptr]
		if !ok {
			owner[ptr] = name
			continue
		}
		if aliases[name] == first || aliases[first] == name {
			continue
		}
		if sums[name] && sums[first] {
			continue
		}
		t.Errorf("%q and %q share one implementation, so one of them runs the other command",
			first, name)
	}
}

// TestSumsAreWiredToTheirOwnAlgorithm is the claim the pointer test cannot make
// for closures: each name in the table must produce its own digest. The values
// are the published test vectors for "abc", not values this code produced.
func TestSumsAreWiredToTheirOwnAlgorithm(t *testing.T) {
	want := map[string]string{
		"md5sum":    "900150983cd24fb0d6963f7d28e17f72",
		"sha1sum":   "a9993e364706816aba3e25717850c26c9cd0d89d",
		"sha256sum": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		"sha512sum": "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a" +
			"2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
	}
	for name, digest := range want {
		fn, ok := Commands[name]
		if !ok {
			t.Fatalf("%q is missing from the table", name)
		}
		out, code := captureIn(t, fn, nil, "abc")
		if code != 0 {
			t.Errorf("%q: exit %d", name, code)
			continue
		}
		if got := strings.Fields(out); len(got) == 0 || got[0] != digest {
			t.Errorf("%q printed %q, want the %s digest of \"abc\"", name, out, name)
		}
	}
}
