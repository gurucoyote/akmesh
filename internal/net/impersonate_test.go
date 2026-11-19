package net

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	utls "github.com/refraction-networking/utls"
)

// TestImpersonateProfiles pins the accepted spellings and the profile behind
// each one. A typo must not silently fall back to Go's own handshake, which
// would look like the feature works while nothing changed.
func TestImpersonateProfiles(t *testing.T) {
	want := map[string]utls.ClientHelloID{
		"chrome":   utls.HelloChrome_Auto,
		"Chrome":   utls.HelloChrome_Auto,
		"edge":     utls.HelloEdge_Auto,
		"chromium": utls.HelloEdge_Auto,
		"firefox":  utls.HelloFirefox_Auto,
		"safari":   utls.HelloSafari_Auto,
		"go":       utls.HelloGolang,
		"golang":   utls.HelloGolang,
	}
	for name, id := range want {
		got, ok := impersonationProfile(name)
		if !ok {
			t.Fatalf("profile %q is not accepted", name)
		}
		if got.Client != id.Client || got.Version != id.Version {
			t.Fatalf("profile %q maps to %v, want %v", name, got, id)
		}
	}
	for _, name := range []string{"curl", "netscape", ""} {
		if _, ok := impersonationProfile(name); ok {
			t.Fatalf("profile %q should not be accepted", name)
		}
	}
}

// TestImpersonateRejectsUnknownProfile: the flag has to say what it accepts
// and fail, not run with Go's handshake instead.
func TestImpersonateRejectsUnknownProfile(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	_, code := capture(t, runCurl, []string{"-s", "--impersonate", "netscape", server.URL + "/hello"})
	if code == 0 {
		t.Fatal("curl accepted an unknown impersonation profile")
	}
}

// TestImpersonateChromeCompletesARequest: the uTLS handshake must work against a
// real TLS server, and the body must arrive. -k because httptest signs with its
// own CA.
func TestImpersonateChromeCompletesARequest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 1 {
			t.Errorf("impersonate spoke HTTP/%d, want HTTP/1.1", r.ProtoMajor)
		}
		w.Write([]byte("hallo ueber uTLS"))
	}))
	defer server.Close()

	out, code := capture(t, runCurl, []string{"-s", "-k", "--impersonate", "chrome", server.URL + "/"})
	if code != 0 {
		t.Fatalf("curl --impersonate chrome exit %d", code)
	}
	if out != "hallo ueber uTLS" {
		t.Fatalf("body = %q", out)
	}
}

// TestImpersonateRefusesProxy: a proxy plus a foreign ClientHello is not
// implemented; the flag must say so rather than quietly hand the request to Go.
func TestImpersonateRefusesProxy(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	_, errOut, code := captureStreams(t, runCurl, []string{"-s", "--impersonate", "chrome", "--proxy", "http://127.0.0.1:9", server.URL + "/hello"})
	if code == 0 {
		t.Fatal("curl accepted --impersonate with --proxy")
	}
	if !strings.Contains(errOut, "cannot be combined") {
		t.Fatalf("stderr %q misses the reason", errOut)
	}
}

// captureStreams is capture plus stderr, for the checks that inspect a message.
func captureStreams(t *testing.T, fn CmdFunc, args []string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errFile, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	defer errFile.Close()

	code := fn(args, os.Stdin, out, errFile)
	read := func(f *os.File) string {
		if _, err := f.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	return read(out), read(errFile), code
}
