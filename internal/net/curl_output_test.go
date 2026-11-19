package net

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCurlIncludePrintsStatusLine: -i must show a status line an agent can
// parse, not the -v shape with "< " in front. Before acmesh-ly7 the first line
// was "< 503 Service Unavailable", so a parser looking for "HTTP/" found
// nothing.
func TestCurlIncludePrintsStatusLine(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, code := capture(t, runCurl, []string{"-s", "-i", server.URL + "/hello"})
	if code != 0 {
		t.Fatalf("curl -s -i exit %d", code)
	}
	lines := strings.SplitN(out, "\n", 2)
	if !strings.HasPrefix(lines[0], "HTTP/1.1 200 OK") {
		t.Fatalf("-i first line = %q, want the status line", lines[0])
	}
	if strings.Contains(out, "< HTTP/") {
		t.Fatalf("-i still prints the verbose prefix: %q", out)
	}
	if !strings.Contains(out, "content-type: text/plain") {
		t.Fatalf("-i headers are missing or not lowercase: %q", out)
	}
	if !strings.HasSuffix(out, "\nhallo welt") {
		t.Fatalf("-i must put the body after the headers, got %q", out)
	}
	// A header line has exactly one colon-separated shape, so a body line must
	// not have been pulled in by the header loop.
	if strings.Contains(out, "< ") {
		t.Fatalf("-i output still carries verbose markers: %q", out)
	}
}

// TestCurlVerboseKeepsThePrefix: -v still uses "< " so the two modes stay
// distinguishable, which is what curl does.
func TestCurlVerboseKeepsThePrefix(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	// -v writes to stderr, so only the body is on stdout.
	out, code := capture(t, runCurl, []string{"-s", "-v", server.URL + "/hello"})
	if code != 0 {
		t.Fatalf("curl -s -v exit %d", code)
	}
	if out != "hallo welt" {
		t.Fatalf("-v must not put headers on stdout, got %q", out)
	}
}

// TestCurlWriteOut: -w answers with the values scripts grep for, and it prints
// to stdout even when -o sends the body to a file.
func TestCurlWriteOut(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "body.txt")
	out, code := capture(t, runCurl, []string{
		"-s", "-o", target,
		"-w", "%{http_code} %{size_download} %{content_type}\\n",
		server.URL + "/hello",
	})
	if code != 0 {
		t.Fatalf("curl -w exit %d", code)
	}
	if out != "200 10 text/plain; charset=utf-8\n" {
		t.Fatalf("-w printed %q, want the status, the size and the type", out)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("body file: %v", err)
	}
	if string(body) != "hallo welt" {
		t.Fatalf("body file holds %q", body)
	}

	out, _ = capture(t, runCurl, []string{
		"-s", "-L", "-o", os.DevNull, "-w", "%{url_effective}", server.URL + "/redirect",
	})
	if out != server.URL+"/hello" {
		t.Fatalf("url_effective = %q, want the final URL after -L", out)
	}
}

// TestWriteOutKeepsUnknownVariables: a typo is reported the way curl reports it
// and the token stays in the output, so a script does not silently lose it.
func TestWriteOutKeepsUnknownVariables(t *testing.T) {
	dir := t.TempDir()
	errPath := filepath.Join(dir, "stderr")
	f, err := os.Create(errPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := expandWriteOut("a%{bogus}b%{http_code}", writeOutVars{code: 204}, f)
	if got != "a%{bogus}b204" {
		t.Fatalf("expandWriteOut = %q", got)
	}
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
	msg, err := os.ReadFile(errPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(msg), "unknown --write-out variable: 'bogus'") {
		t.Fatalf("stderr %q misses the warning", msg)
	}
}

// TestWriteOutEscapes: \n, \t and \\ behave as in curl's format strings.
func TestWriteOutEscapes(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "s"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := expandWriteOut("a\\tb\\nc\\\\d", writeOutVars{}, f)
	if got != "a\tb\nc\\d" {
		t.Fatalf("escapes = %q", got)
	}
}
