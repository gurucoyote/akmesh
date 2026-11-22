package id

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestRunsWithoutAccountDatabase is the minimal-image case: /etc/passwd and
// /etc/group are absent, which must not stop the answer. GNU id prints the bare
// numbers there; -n prints the number, complains, and exits nonzero.
func TestRunsWithoutAccountDatabase(t *testing.T) {
	const missingPasswd = "/nonexistent-akmesh/passwd"
	const missingGroup = "/nonexistent-akmesh/group"

	var out, errOut bytes.Buffer
	missing, err := run(&out, &errOut, "", &flags{}, missingPasswd, missingGroup)
	if err != nil {
		t.Fatalf("run without passwd/group: %v", err)
	}
	if missing {
		t.Fatalf("no -n given, so no name can be missing")
	}
	got := strings.TrimSpace(out.String())
	if !strings.HasPrefix(got, "uid="+strconv.Itoa(os.Geteuid())) {
		t.Fatalf("output = %q, want the numeric uid first", got)
	}
	if strings.Contains(got, "(") {
		t.Fatalf("output = %q, want no name in parentheses without a database", got)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}

	out.Reset()
	errOut.Reset()
	f := &flags{user: true, name: true}
	missing, err = run(&out, &errOut, "", f, missingPasswd, missingGroup)
	if err != nil {
		t.Fatalf("run -un without passwd: %v", err)
	}
	if !missing {
		t.Fatalf("-un without passwd must report the missing name")
	}
	if got := strings.TrimSpace(out.String()); got != strconv.Itoa(os.Geteuid()) {
		t.Fatalf("-un printed %q, want the number", got)
	}
	if !strings.Contains(errOut.String(), "cannot find name for user ID") {
		t.Fatalf("stderr = %q, want the GNU complaint", errOut.String())
	}
}

// TestKnownAccountsStillPrintNames guards the normal case: the fork must not
// lose the names it can find.
func TestKnownAccountsStillPrintNames(t *testing.T) {
	dir := t.TempDir()
	passwd := dir + "/passwd"
	group := dir + "/group"
	if err := os.WriteFile(passwd, []byte("root:x:0:0:root:/root:/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(group, []byte("root:x:0:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	f := &flags{user: true, name: true, real: false}
	if _, err := run(&out, &errOut, "root", f, passwd, group); err != nil {
		t.Fatalf("run for root: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "root" {
		t.Fatalf("-un root = %q, want root", got)
	}
}

// TestCombinedShortFlags covers id -un, the spelling agents use.
func TestCombinedShortFlags(t *testing.T) {
	got := strings.Join(expandShort([]string{"-un", "root", "-Gn"}), " ")
	if got != "-u -n root -G -n" {
		t.Fatalf("expandShort = %q", got)
	}
	// An unknown letter keeps the word whole so the parser still complains.
	if got := strings.Join(expandShort([]string{"-uZ"}), " "); got != "-uZ" {
		t.Fatalf("expandShort(-uZ) = %q, want the word unchanged", got)
	}
}
