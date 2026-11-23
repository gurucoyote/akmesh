//go:build !windows

package akinit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// errFake is the spawn failure of the backoff test.
var errFake = errors.New("fake spawn failure")

func writeTab(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inittab")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustParse(t *testing.T, path string) []entry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	entries, err := ParseInittab(f)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return string(b)
}

// zombiesOf counts the Z lines among this process's children, read from
// /proc/<pid>/stat like ps does - a child that was never waited for stays
// there with state Z.
func zombiesOf(t *testing.T) int {
	t.Helper()
	self := os.Getpid()
	dirs, err := os.ReadDir("/proc")
	if err != nil {
		t.Skipf("no /proc: %v", err)
	}
	selfStr := strconv.Itoa(self)
	n := 0
	var pids []int
	for _, d := range dirs {
		if _, err := strconv.Atoi(d.Name()); err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", d.Name(), "stat"))
		if err != nil {
			continue
		}
		// state and ppid sit after the parenthesised command name
		s := string(raw)
		close := strings.LastIndex(s, ")")
		if close < 0 {
			continue
		}
		fields := strings.Fields(s[close+1:])
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "Z" && fields[1] == selfStr {
			n++
			if pid, err := strconv.Atoi(d.Name()); err == nil {
				pids = append(pids, pid)
			}
		}
	}
	if n > 0 {
		sort.Ints(pids)
		t.Logf("zombies of this process: %v", pids)
	}
	return n
}

// ensure the fake spawn in the backoff test has the right type
var _ = fmt.Sprintf
var _ func(entry) (*exec.Cmd, error)
