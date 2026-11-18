package policy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestParseRejectsMistakes covers the mistakes that would quietly open more than
// the author intended, or refuse everything.
func TestParseRejectsMistakes(t *testing.T) {
	cases := map[string]string{
		"unknown field":   `{"version":1,"read":["/tmp"],"reed":["/etc"]}`,
		"bad version":     `{"version":2,"read":["/tmp"]}`,
		"no paths":        `{"version":1}`,
		"relative path":   `{"version":1,"read":["tmp"]}`,
		"relative write":  `{"version":1,"write":["tmp/work"]}`,
		"relative exec":   `{"version":1,"read":["/tmp"],"exec":["bin"]}`,
		"not json at all": `version: 1`,
	}
	for name, body := range cases {
		if _, err := Parse([]byte(body), "test"); err == nil {
			t.Fatalf("%s: accepted, want an error", name)
		}
	}
}

func TestParseAcceptsAValidPolicy(t *testing.T) {
	body := `{"version":1,"read":["/tmp/work"],"write":["/tmp/work"],
	          "network":["example.com","api.example.com:443"],"system_access":false}`
	p, err := Parse([]byte(body), "test.json")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p.Source() != "test.json" {
		t.Fatalf("source %q", p.Source())
	}
	found := false
	for _, item := range p.ReadPaths() {
		if item == "/tmp/work" {
			found = true
		}
	}
	if !found {
		t.Fatalf("read paths %v do not contain the configured path", p.ReadPaths())
	}
	// The running binary is always added, otherwise a shell inside the policy
	// could not start a single command.
	if len(p.ExecPaths()) == 0 {
		t.Fatalf("exec paths %v, want the running binary in there", p.ExecPaths())
	}
}

// TestHostMatching pins the network gate: exact names, ports and one wildcard.
func TestHostMatching(t *testing.T) {
	p, err := Parse([]byte(`{"version":1,"read":["/tmp"],
	  "network":["example.com","api.example.com:443","*.testbild.tv"]}`), "test")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		host string
		want bool
	}{
		{"example.com", true},
		{"example.com:443", true},
		{"api.example.com:443", true},
		{"api.example.com:8443", false},
		{"other.example.com", false},
		{"testbild.tv", false},
		{"kleinkai.testbild.tv", true},
		{"kleinkai.testbild.tv:443", true},
		{"evil.com", false},
	}
	for _, c := range cases {
		if got := p.AllowHost(c.host); got != c.want {
			t.Fatalf("AllowHost(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

// TestNoPolicyIsUnrestricted checks the plain busybox use case.
func TestNoPolicyIsUnrestricted(t *testing.T) {
	if Active() != nil {
		t.Skip("a policy is active in this process")
	}
	if err := CheckHost("anything.example"); err != nil {
		t.Fatalf("without a policy the network must be free, got %v", err)
	}
}

// TestInitFromEnvFailsClosed checks that a broken policy file stops the process
// instead of silently running unconfined.
func TestInitFromEnvFailsClosed(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(broken, []byte(`{"version":9}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ACMESH_POLICY", broken)
	if err := InitFromEnv(); err == nil {
		t.Fatalf("broken policy accepted")
	}
	if InitError() == nil {
		t.Fatalf("InitError did not report the failure")
	}
	if Active() != nil {
		t.Fatalf("a policy is active although loading failed")
	}
	// Restore the unrestricted state for the other tests.
	t.Setenv("ACMESH_POLICY", "")
	if err := InitFromEnv(); err != nil {
		t.Fatalf("clearing the policy failed: %v", err)
	}
}

// TestSupportedReportsKernel pins the shape the policy command prints, so the
// explanation a harness reads names the mechanism instead of a vague "ok".
func TestSupportedReportsKernel(t *testing.T) {
	ok, detail := Supported()
	if ok {
		if m, err := regexp.MatchString(`^landlock ABI [0-9]+$`, detail); err != nil || !m {
			t.Fatalf("detail %q, want \"landlock ABI <n>\"", detail)
		}
		return
	}
	// Without Landlock the detail must say why, not stay empty.
	if !strings.Contains(detail, "landlock") {
		t.Fatalf("unsupported kernel reports %q, want a reason", detail)
	}
}

// TestWriteOnlyPolicyIsAccepted pins the documented "write implies read": a
// policy may name only write paths, and Enforce hands that rule the read rights
// as well. This test cannot call Enforce - it would confine the test process
// for good - so the kernel side is proven live in scripts/test-ext.sh.
func TestWriteOnlyPolicyIsAccepted(t *testing.T) {
	p, err := Parse([]byte(`{"version":1,"write":["/tmp/work"],"system_access":false}`), "test")
	if err != nil {
		t.Fatalf("a write-only policy must load: %v", err)
	}
	writes := p.WritePaths()
	if len(writes) != 1 || writes[0] != "/tmp/work" {
		t.Fatalf("write paths %v, want [/tmp/work]", writes)
	}
}
