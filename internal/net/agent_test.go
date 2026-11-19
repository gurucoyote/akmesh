package net

import (
	"strings"
	"testing"
)

// TestDefaultUserAgentIsOurs: the client must not announce itself as
// "Go-http-client". Measured on amazon.de: that exact string got a 503 while
// every other agent string got the page (acmesh-ijz).
func TestDefaultUserAgentIsOurs(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, code := capture(t, runCurl, []string{"-s", server.URL + "/ua"})
	if code != 0 {
		t.Fatalf("curl exit %d", code)
	}
	if strings.Contains(out, "Go-http-client") {
		t.Fatalf("curl still announces Go: %q", out)
	}
	if !strings.Contains(out, "user-agent="+agentCurl) {
		t.Fatalf("curl user-agent = %q, want %q", out, agentCurl)
	}
	if !strings.Contains(out, "accept=*/*") {
		t.Fatalf("curl did not send Accept: */*, got %q", out)
	}
}

// TestWgetDefaultUserAgent: wget says it behaves like wget, not like curl.
func TestWgetDefaultUserAgent(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, code := capture(t, runWget, []string{"-q", server.URL + "/ua"})
	if code != 0 {
		t.Fatalf("wget exit %d", code)
	}
	if !strings.Contains(out, "user-agent="+agentWget) {
		t.Fatalf("wget user-agent = %q, want %q", out, agentWget)
	}
	if strings.Contains(out, "Go-http-client") {
		t.Fatalf("wget still announces Go: %q", out)
	}
}

// TestUserAgentFlagStillWins: -A and -U override the default, which scripts
// rely on.
func TestUserAgentFlagStillWins(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, code := capture(t, runCurl, []string{"-s", "-A", "eigener-agent/1", server.URL + "/ua"})
	if code != 0 {
		t.Fatalf("curl -A exit %d", code)
	}
	if !strings.Contains(out, "user-agent=eigener-agent/1") {
		t.Fatalf("curl -A was ignored: %q", out)
	}

	out, code = capture(t, runWget, []string{"-q", "-U", "wget-eigen/2", server.URL + "/ua"})
	if code != 0 {
		t.Fatalf("wget -U exit %d", code)
	}
	if !strings.Contains(out, "user-agent=wget-eigen/2") {
		t.Fatalf("wget -U was ignored: %q", out)
	}
}

// TestHeaderFlagOverridesAccept: -H wins over the default Accept, so an agent
// that asks for JSON gets JSON negotiation.
func TestHeaderFlagOverridesAccept(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, code := capture(t, runCurl, []string{"-s", "-H", "Accept: application/json", server.URL + "/ua"})
	if code != 0 {
		t.Fatalf("curl -H exit %d", code)
	}
	if !strings.Contains(out, "accept=application/json") {
		t.Fatalf("-H Accept was overridden by the default: %q", out)
	}
}
