package net

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	out, code := retryCapture(t, runCurl, []string{"-s", "--resolve", "test1.local:" + port + ":127.0.0.1", target})
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

// TestCACertReplacesTheSystemRoots: --cacert is how a local CA is trusted for
// one request without touching the system store (acmesh-816).
func TestCACertReplacesTheSystemRoots(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	}))
	defer server.Close()

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	cert := server.Certificate()
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := capture(t, runCurl, []string{"-s", "--cacert", caPath, server.URL})
	if code != 0 {
		t.Fatalf("exit %d, out %q", code, out)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("body %q", out)
	}

	// A file without a certificate is an error, not a silent fallback to the
	// system store: that would make a test pass for the wrong reason.
	junk := filepath.Join(dir, "junk.pem")
	if err := os.WriteFile(junk, []byte("not a certificate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, code := capture(t, runCurl, []string{"-s", "--cacert", junk, server.URL}); code == 0 {
		t.Fatal("a file without a certificate was accepted")
	}
	if _, code := capture(t, runCurl, []string{"-s", "--cacert", filepath.Join(dir, "nope.pem"), server.URL}); code == 0 {
		t.Fatal("a missing --cacert file was accepted")
	}
}

// TestWgetCanResolveAndTrustALocalCA: wget has to be able to do the same job as
// curl for a local name, with its own GNU spellings (acmesh-730 follow-up).
func TestWgetCanResolveAndTrustALocalCA(t *testing.T) {
	// A server whose certificate really carries test1.local: httptest's own cert
	// is for 127.0.0.1, and a name mismatch would make this test pass for the
	// wrong reason.
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("wget-ok\n"))
	}))
	cert, caPEM := selfSignedFor(t, "test1.local")
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port := u.Port()

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caPath, caPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	target := "https://test1.local:" + port + "/"
	out, code := retryCapture(t, runWget, []string{"-q", "-O", "-", "--ca-certificate", caPath, "--resolve", "test1.local:" + port + ":127.0.0.1", target})
	if code != 0 {
		t.Fatalf("exit %d, out %q", code, out)
	}
	if strings.TrimSpace(out) != "wget-ok" {
		t.Fatalf("body %q", out)
	}
	// Without the root the name is unknown: the request must fail loudly.
	if _, code := capture(t, runWget, []string{"-q", "-O", "-", "--resolve", "test1.local:" + port + ":127.0.0.1", target}); code == 0 {
		t.Fatal("wget trusted a chain nobody vouched for")
	}
}

// selfSignedFor makes a leaf certificate for one name and returns it with its
// PEM, which serves as the trusted root in the test.
func selfSignedFor(t *testing.T, name string) (tls.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{name},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key},
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// retryCapture runs a command up to three times while it fails to connect. Under
// WSL a dial occasionally dies while the whole test suite runs ("accept4 failed
// 110"), and a test that reports that as a wrong --resolve would be lying about
// the code.
func retryCapture(t *testing.T, fn CmdFunc, args []string) (string, int) {
	t.Helper()
	var out string
	var code int
	for attempt := 0; attempt < 3; attempt++ {
		out, code = capture(t, fn, args)
		if code == 0 {
			return out, code
		}
	}
	return out, code
}
