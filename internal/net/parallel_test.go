package net

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestParallelRepeatHitsTheServerThatOften: --repeat counts, -Z parallelises and
// tells the truth about it (acmesh-730).
func TestParallelRepeatHitsTheServerThatOften(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Write([]byte("ok\n"))
	}))
	defer server.Close()

	_, code := capture(t, runCurl, []string{"-s", "-Z", "--repeat", "120", "--parallel-max", "20", server.URL + "/"})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := atomic.LoadInt64(&hits); got != 120 {
		t.Fatalf("the server saw %d requests, want 120", got)
	}
}

// TestParallelIsFasterThanSequential: with a slow server, 20 parallel requests
// have to take clearly less than 20 sequential ones. This is what proves the
// goroutines are real and not a loop dressed up as a flood.
func TestParallelIsFasterThanSequential(t *testing.T) {
	const n = 20
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte("ok\n"))
	}))
	defer server.Close()

	start := time.Now()
	if _, code := capture(t, runCurl, []string{"-s", "-Z", "--repeat", "20", "--parallel-max", "20", server.URL + "/"}); code != 0 {
		t.Fatalf("parallel exit %d", code)
	}
	parallel := time.Since(start)

	start = time.Now()
	if _, code := capture(t, runCurl, []string{"-s", "-Z", "--repeat", "20", "--parallel-max", "1", server.URL + "/"}); code != 0 {
		t.Fatalf("sequential exit %d", code)
	}
	sequential := time.Since(start)

	if parallel*2 > sequential {
		t.Fatalf("20 parallel took %v, 20 one at a time %v - not parallel", parallel, sequential)
	}
}

// TestParallelSummary: the summary names the count and the status codes, and a
// 500 under the answers makes the exit non-zero with --fail.
func TestParallelSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "bad") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte("ok\n"))
	}))
	defer server.Close()

	dir := t.TempDir()
	out, _ := os.Create(filepath.Join(dir, "out"))
	errf, _ := os.Create(filepath.Join(dir, "err"))
	code := runCurl([]string{"-Z", "--repeat", "4", server.URL + "/good"}, nil, out, errf)
	out.Close()
	errf.Close()
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	msg, _ := os.ReadFile(filepath.Join(dir, "err"))
	if !strings.Contains(string(msg), "4 requests, 0 failed") || !strings.Contains(string(msg), "200: 4") {
		t.Fatalf("summary %q", msg)
	}

	out2, _ := os.Create(filepath.Join(dir, "out2"))
	err2, _ := os.Create(filepath.Join(dir, "err2"))
	code = runCurl([]string{"-s", "-f", "-Z", "--repeat", "3", server.URL + "/bad"}, nil, out2, err2)
	out2.Close()
	err2.Close()
	if code != exitHTTPError {
		t.Fatalf("a 500 under --fail gave exit %d, want %d", code, exitHTTPError)
	}
}

// TestParallelRefusals: the numbers are checked, not guessed.
func TestParallelRefusals(t *testing.T) {
	cases := [][]string{
		{"-s", "-Z"},
		{"-s", "-Z", "--parallel-max", "0", "http://example.invalid/"},
		{"-s", "-Z", "--parallel-max", "100000", "http://example.invalid/"},
		{"-s", "-Z", "--repeat", "0", "http://example.invalid/"},
		{"-s", "-Z", "--repeat", "abc", "http://example.invalid/"},
	}
	for _, args := range cases {
		if _, code := capture(t, runCurl, args); code != 2 {
			t.Fatalf("%v: exit %d, want 2", args, code)
		}
	}
}
