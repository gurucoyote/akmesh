package net

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestServer starts a server with the endpoints these tests need. Testing
// against httptest instead of the internet keeps the checks reproducible.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hallo welt")
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		// The 302 carries a body of its own, so a test can tell "stopped at the
		// redirect" from "printed nothing at all".
		w.Header().Set("Location", "/hello")
		w.WriteHeader(http.StatusFound)
		fmt.Fprint(w, "umleitung")
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(1 << 20)
		fmt.Fprintf(w, "method=%s header=%s auth=%s form=%s file=%s cookie=%s",
			r.Method,
			r.Header.Get("X-Test"),
			r.Header.Get("Authorization"),
			r.PostFormValue("feld"),
			multipartValue(r),
			cookieValue(r, "sitzung"))
	})
	// /ua answers with what the client announced, so a test can pin the default
	// User-Agent and Accept (acmesh-ijz).
	mux.HandleFunc("/ua", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "user-agent=%s|accept=%s", r.Header.Get("User-Agent"), r.Header.Get("Accept"))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		fmt.Fprint(w, "spaet")
	})
	// ServeContent honours Range requests, which a plain write does not.
	mux.HandleFunc("/bytes", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "bytes.txt", time.Now(), strings.NewReader("0123456789"))
	})
	return httptest.NewServer(mux)
}

func multipartValue(r *http.Request) string {
	if r.MultipartForm == nil {
		return ""
	}
	values := r.MultipartForm.Value["datei"]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// TestCurlSilentKeepsBody is the regression that matters most: the tool that
// motivated this work swallowed the body when -s was given.
func TestCurlSilentKeepsBody(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, code := capture(t, runCurl, []string{"-s", server.URL + "/hello"})
	if code != 0 {
		t.Fatalf("curl -s exit %d", code)
	}
	if out != "hallo welt" {
		t.Fatalf("curl -s printed %q, want the body", out)
	}
}

// TestCurlCombinedShortFlags checks -fsSL, the shape agents write.
func TestCurlCombinedShortFlags(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, code := capture(t, runCurl, []string{"-fsSL", server.URL + "/redirect"})
	if code != 0 {
		t.Fatalf("-fsSL exit %d", code)
	}
	if out != "hallo welt" {
		t.Fatalf("-fsSL printed %q, redirect not followed", out)
	}
}

func TestCurlNoFollowWithoutL(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, code := capture(t, runCurl, []string{"-s", "-o", "-", server.URL + "/redirect"})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	// The body of the 302 itself, not nothing: a curl that prints no body at all
	// would also satisfy a bare "must not contain the target" check.
	if out != "umleitung" {
		t.Fatalf("without -L the answer must be the redirect body, got %q", out)
	}
}

func TestCurlPostFormAndHeader(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, _ := capture(t, runCurl, []string{
		"-s", "-H", "X-Test: wert", "-d", "feld=inhalt", server.URL + "/echo",
	})
	if !strings.Contains(out, "header=wert") {
		t.Fatalf("header missing: %q", out)
	}
	if !strings.Contains(out, "method=POST") {
		t.Fatalf("-d must produce a POST: %q", out)
	}
	if !strings.Contains(out, "form=inhalt") {
		t.Fatalf("form value missing: %q", out)
	}
}

func TestCurlBasicAuth(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, _ := capture(t, runCurl, []string{"-s", "-u", "martin:geheim", server.URL + "/echo"})
	if !strings.Contains(out, "Basic bWFydGluOmdlaGVpbQ==") {
		t.Fatalf("basic auth header missing: %q", out)
	}
}

func TestCurlMultipartUpload(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "datei.txt")
	if err := os.WriteFile(path, []byte("inhalt-der-datei"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _ := capture(t, runCurl, []string{
		"-s", "-F", "datei=@" + path, "-F", "feld=formwert", server.URL + "/echo",
	})
	if !strings.Contains(out, "form=formwert") {
		t.Fatalf("multipart field missing: %q", out)
	}
}

func TestCurlCookieJar(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	jar := filepath.Join(dir, "cookies.txt")
	out, code := capture(t, runCurl, []string{
		"-s", "-b", "sitzung=abc123", "-c", jar, server.URL + "/echo",
	})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "cookie=abc123") {
		t.Fatalf("cookie not sent: %q", out)
	}
	data, err := os.ReadFile(jar)
	if err != nil {
		t.Fatalf("jar not written: %v", err)
	}
	if !strings.Contains(string(data), "sitzung") {
		t.Fatalf("jar content: %q", data)
	}
}

func TestCurlFailsOnHTTPError(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	_, code := capture(t, runCurl, []string{"-s", "-f", server.URL + "/gibtsnicht"})
	if code != exitHTTPError {
		t.Fatalf("curl -f on 404 = %d, want %d", code, exitHTTPError)
	}
	// Without -f the same request is a success, as in curl.
	_, code = capture(t, runCurl, []string{"-s", server.URL + "/gibtsnicht"})
	if code != 0 {
		t.Fatalf("curl without -f on 404 = %d, want 0", code)
	}
}

func TestCurlTimeout(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	_, code := capture(t, runCurl, []string{"-s", "--max-time", "1", server.URL + "/slow"})
	// 300ms of sleep fits into one second: this must succeed.
	if code != 0 {
		t.Fatalf("fast request failed with %d", code)
	}
	// The endpoint sleeps 300 ms against a 100 ms budget, so the only correct
	// answer is curl's timeout code. Accepting a connect error here as well
	// would hide a timeout that is never enforced.
	_, code = capture(t, runCurl, []string{"-s", "--max-time", "0.1", server.URL + "/slow"})
	if code != exitTimeout {
		t.Fatalf("slow request returned %d, want %d (timeout)", code, exitTimeout)
	}
}

func TestCurlOutputFile(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	_, code := capture(t, runCurl, []string{"-s", "-o", target, server.URL + "/hello"})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("output not written: %v", err)
	}
	if string(data) != "hallo welt" {
		t.Fatalf("file content %q", data)
	}
}

func TestCurlRange(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, code := capture(t, runCurl, []string{"-s", "-r", "2-4", server.URL + "/bytes"})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out != "234" {
		t.Fatalf("range gave %q, want 234", out)
	}
}

// TestSchemeIsRefused checks that file:// cannot be used to read the local disk.
func TestSchemeIsRefused(t *testing.T) {
	_, code := capture(t, runCurl, []string{"-s", "file:///etc/passwd"})
	if code != 3 {
		t.Fatalf("file:// returned %d, want 3", code)
	}
}

func TestWgetFront(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	out, code := capture(t, runWget, []string{"-q", "-O", "-", server.URL + "/hello"})
	if code != 0 || out != "hallo welt" {
		t.Fatalf("wget -O - = %d, %q", code, out)
	}
	out, _ = capture(t, runWget, []string{"-q", "-O", "-", "--post-data", "feld=w", server.URL + "/echo"})
	if !strings.Contains(out, "method=POST") {
		t.Fatalf("--post-data did not POST: %q", out)
	}
	out, _ = capture(t, runWget, []string{"-q", "-S", "-O", "-", server.URL + "/hello"})
	if !strings.Contains(out, "200 OK") {
		t.Fatalf("-S must show the status line: %q", out)
	}
	// Recursive download is refused with a message, not silently ignored.
	_, code = capture(t, runWget, []string{"-r", server.URL + "/hello"})
	if code != 2 {
		t.Fatalf("-r returned %d, want 2", code)
	}
}

func TestCurlRejectsUnknownFlags(t *testing.T) {
	// -Z is -Z since acmesh-730, so the unknown flag has to be another one.
	if _, code := capture(t, runCurl, []string{"-Y", "http://example.com"}); code != 2 {
		t.Fatalf("unknown short flag = %d, want 2", code)
	}
	if _, code := capture(t, runCurl, []string{"--nope", "http://example.com"}); code != 2 {
		t.Fatalf("unknown long flag = %d, want 2", code)
	}
	if _, code := capture(t, runWget, []string{"-N", "http://example.com"}); code != 2 {
		t.Fatalf("wget -N = %d, want 2", code)
	}
}
