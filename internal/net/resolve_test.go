package net

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestResolveConnectsToTheGivenAddress: --resolve sends the request to the
// address while the URL keeps the name, which is how test1.local is checked
// without an /etc/hosts entry (acmesh-816).
func TestResolveConnectsToTheGivenAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The name has to survive on our side: Host is what the server sees.
		if r.Host == "" || !strings.HasPrefix(r.Host, "test1.local:") {
			t.Errorf("Host is %q, want test1.local:…", r.Host)
		}
		w.Write([]byte("resolved\n"))
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port := u.Port()

	target := "http://test1.local:" + port + "/"
	out, code := capture(t, runCurl, []string{"-s", "--resolve", "test1.local:" + port + ":127.0.0.1", target})
	if code != 0 {
		t.Fatalf("exit %d, out %q", code, out)
	}
	if strings.TrimSpace(out) != "resolved" {
		t.Fatalf("body %q", out)
	}

	// Without --resolve the name does not exist, and that is an error, not a
	// silent connection somewhere else.
	if _, code := capture(t, runCurl, []string{"-s", "-m", "2", target}); code == 0 {
		t.Fatal("test1.local was reached without --resolve")
	}
}

// TestResolveRefusesForeignSyntax: only HOST:PORT:ADDRESS is understood.
func TestResolveRefusesForeignSyntax(t *testing.T) {
	for _, v := range []string{"host", "host:80", "host:80:", ":80:1.2.3.4", "host:80:1.2.3.4,81", "host:80:+1.2.3.4"} {
		if _, code := capture(t, runCurl, []string{"-s", "--resolve", v, "http://example.invalid/"}); code != 2 {
			t.Fatalf("%q: exit %d, want 2", v, code)
		}
	}
}
