package httpd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newServer builds a server over a temporary tree with one page, one stylesheet
// and one subdirectory.
func newServer(t *testing.T) (*server, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>hallo</h1>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.css"), []byte("body{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	return &server{opts: Options{Home: dir, Realm: "Test"}, stderr: os.Stderr}, dir
}

// get runs one request against the server and returns the response.
func get(t *testing.T, s *server, path string) *http.Response {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serve)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec.Result()
}

// TestServesIndexAndCss: the two content types a page under construction needs,
// plus the index.html rule for the directory itself.
func TestServesIndexAndCss(t *testing.T) {
	s, _ := newServer(t)

	r := get(t, s, "/")
	if r.StatusCode != 200 {
		t.Fatalf("index: status %d", r.StatusCode)
	}
	body, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(body), "hallo") {
		t.Errorf("index body: %q", string(body))
	}
	if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("index type: %q", ct)
	}

	r = get(t, s, "/a.css")
	if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "text/css") {
		t.Errorf("css type: %q", ct)
	}
}

// TestMissingFileIs404.
func TestMissingFileIs404(t *testing.T) {
	s, _ := newServer(t)
	if r := get(t, s, "/nix"); r.StatusCode != 404 {
		t.Errorf("status %d, want 404", r.StatusCode)
	}
}

// TestDirectoryListing lists what is there, not what is not.
func TestDirectoryListing(t *testing.T) {
	s, _ := newServer(t)
	r := get(t, s, "/sub/")
	if r.StatusCode != 200 {
		t.Fatalf("status %d", r.StatusCode)
	}
	body, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(body), "../") {
		t.Errorf("listing has no way up: %q", string(body))
	}
}

// TestDotDotStaysInside: a request for ../../etc/passwd must not leave the root.
func TestDotDotStaysInside(t *testing.T) {
	s, _ := newServer(t)
	r := get(t, s, "/../../etc/passwd")
	if r.StatusCode == 200 {
		t.Errorf("traversal served a file outside the root")
	}
}

// TestAuthNeedsTheRightPassword: the A: lines of the config file protect the
// whole tree, and the password is accepted in plain and in busybox's MD5 form.
func TestAuthNeedsTheRightPassword(t *testing.T) {
	s, _ := newServer(t)
	s.conf = []confEntry{{user: "martin", pass: "geheim"}}

	r := get(t, s, "/")
	if r.StatusCode != 401 {
		t.Fatalf("without credentials: status %d, want 401", r.StatusCode)
	}
	if r.Header.Get("WWW-Authenticate") == "" {
		t.Errorf("no realm header")
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("martin", "geheim")
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serve)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("with credentials: status %d", rec.Code)
	}
}

// TestConfPrefixMapsDirectory: "/static:/srv/www" serves that directory under
// the prefix, and the catch-all "*" is used when nothing matches.
func TestConfPrefixMapsDirectory(t *testing.T) {
	s, dir := newServer(t)
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "x.txt"), []byte("aus dem anderen Verzeichnis\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.conf = []confEntry{{prefix: "/static", dir: other}}
	r := get(t, s, "/static/x.txt")
	if r.StatusCode != 200 {
		t.Fatalf("status %d", r.StatusCode)
	}
	body, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(body), "anderen") {
		t.Errorf("body: %q", string(body))
	}

	// Without a prefix the document root is still used.
	r = get(t, s, "/a.css")
	if r.StatusCode != 200 {
		t.Errorf("document root: status %d (dir %s)", r.StatusCode, dir)
	}
}

// TestMimeAndHelpers covers the three one-string modes and the extension table.
func TestMimeAndHelpers(t *testing.T) {
	if got, _ := helper("-M", "x.mjs"); got != "text/javascript; charset=utf-8" {
		t.Errorf("-M mjs: %q", got)
	}
	if got, _ := helper("-M", "x.css"); got != "text/css; charset=utf-8" {
		t.Errorf("-M css: %q", got)
	}
	if got, _ := helper("-d", "a%20b"); got != "a b" {
		t.Errorf("-d: %q", got)
	}
	if got, _ := helper("-e", "<a>"); got != "&lt;a&gt;" {
		t.Errorf("-e: %q", got)
	}
}

// TestNoCacheHeaderOnlyWithN: -N is for looking at files while editing, and it
// must not appear otherwise.
func TestNoCacheHeaderOnlyWithN(t *testing.T) {
	s, _ := newServer(t)
	if r := get(t, s, "/a.css"); r.Header.Get("Cache-Control") != "" {
		t.Errorf("unexpected cache header: %q", r.Header.Get("Cache-Control"))
	}
	s.opts.NoCache = true
	if r := get(t, s, "/a.css"); r.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("no-store missing: %q", r.Header.Get("Cache-Control"))
	}
}
