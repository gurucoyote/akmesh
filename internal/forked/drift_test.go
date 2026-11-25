package forked

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A fork is a copy, so it drifts. The manifest records which u-root commit and
// which upstream file each copy came from; this test makes a moved pin visible
// instead of letting the copy silently age.
type manifest struct {
	Forks map[string]struct {
		Commit string `json:"commit"`
		Files  []struct {
			Upstream string `json:"upstream"`
			SHA256   string `json:"sha256"`
		} `json:"files"`
	} `json:"forks"`
}

// pinnedCommit reads UROOT_COMMIT out of the build script, the one place the
// checkout pin is written down.
func pinnedCommit(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../scripts/build-bb.sh")
	if err != nil {
		t.Fatalf("read build script: %v", err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if after, ok := strings.CutPrefix(line, "UROOT_COMMIT="); ok {
			return strings.TrimSpace(after)
		}
	}
	t.Fatal("UROOT_COMMIT not found in scripts/build-bb.sh")
	return ""
}

// TestManifestMatchesPin fails when the build pin moved without the forks being
// reviewed. Fix: look at the new upstream files, update the copy if needed, then
// record the new commit and hashes (scripts/fork-uroot.sh does the recording).
func TestManifestMatchesPin(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(m.Forks) == 0 {
		t.Fatal("manifest lists no forks; a fork without an entry cannot be checked")
	}
	pin := pinnedCommit(t)
	for name, fork := range m.Forks {
		if fork.Commit != pin {
			t.Errorf("fork %s was taken from %s but the build pins %s: review the copy, then update manifest.json", name, fork.Commit, pin)
		}
		if len(fork.Files) == 0 {
			t.Errorf("fork %s lists no upstream files", name)
		}
	}
}

// TestForkProvenance: every copied file names its origin, so a reader of the
// file alone knows it is not ours.
func TestForkProvenance(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, err := filepath.Glob(filepath.Join(e.Name(), "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			// A file is either a copy that names its origin, or our own
			// addition that says so. Forcing "Forked from u-root" onto a file
			// we wrote would be a false claim (acmesh-m6x).
			text := string(data)
			if !strings.Contains(text, "// Forked from u-root ") &&
				!strings.Contains(text, "// Local addition, no upstream source: ") {
				t.Errorf("%s neither names its origin nor says it is ours", file)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no forked file found, so nothing was checked")
	}
}

// TestUpstreamHashesMatchCheckout runs only when the pinned checkout is there.
// It catches a manifest whose hash was recorded from a modified or wrong file.
func TestUpstreamHashesMatchCheckout(t *testing.T) {
	uroot := "../../.build/uroot"
	if _, err := os.Stat(filepath.Join(uroot, ".git")); err != nil {
		t.Skip("no u-root checkout, hashes cannot be recomputed")
	}
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	pin := pinnedCommit(t)
	out, err := exec.Command("git", "-C", uroot, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Skipf("git rev-parse failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != pin {
		t.Skipf("checkout is at %s, not at the pin; the hash check would compare the wrong source", strings.TrimSpace(string(out)))
	}
	for name, fork := range m.Forks {
		for _, f := range fork.Files {
			content, err := os.ReadFile(filepath.Join(uroot, f.Upstream))
			if err != nil {
				t.Errorf("fork %s: upstream file %s is gone: %v", name, f.Upstream, err)
				continue
			}
			sum := sha256.Sum256(content)
			if got := hex.EncodeToString(sum[:]); got != f.SHA256 {
				t.Errorf("fork %s: %s changed upstream (recorded %s, got %s): review the copy", name, f.Upstream, f.SHA256, got)
			}
		}
	}
}
