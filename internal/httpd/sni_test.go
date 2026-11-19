package httpd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testCA writes a CA pair, optionally constrained to the given suffixes.
func testCA(t *testing.T, domains ...string) (certPath, keyPath string, ca *x509.Certificate) {
	t.Helper()
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		PermittedDNSDomains:   domains,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, "ca-cert.pem")
	keyPath = filepath.Join(dir, "ca-key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatal(err)
	}
	kb, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kb}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath, ca
}

// TestSNISignerIssuesOneLeafPerName: test1.local gets its own leaf, with the name
// in the SAN, 390 days, and a chain that verifies against the CA (acmesh-816).
func TestSNISignerIssuesOneLeafPerName(t *testing.T) {
	certPath, keyPath, ca := testCA(t, ".local")
	s, err := newSNISigner(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.getCertificate(&tls.ClientHelloInfo{ServerName: "test1.local"})
	if err != nil {
		t.Fatalf("test1.local was refused: %v", err)
	}
	leaf, err := x509.ParseCertificate(got.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != "test1.local" {
		t.Fatalf("SAN %v", leaf.DNSNames)
	}
	days := leaf.NotAfter.Sub(leaf.NotBefore).Hours() / 24
	if days < 389 || days > 391 {
		t.Fatalf("leaf lives %.1f days, want about 390", days)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: "test1.local", Roots: pool}); err != nil {
		t.Fatalf("the leaf does not chain to its CA: %v", err)
	}

	// A second name gets a second leaf, and the first is cached, not reissued.
	second, err := s.getCertificate(&tls.ClientHelloInfo{ServerName: "test2.local"})
	if err != nil {
		t.Fatal(err)
	}
	if string(second.Certificate[0]) == string(got.Certificate[0]) {
		t.Fatal("two names share one certificate")
	}
	again, err := s.getCertificate(&tls.ClientHelloInfo{ServerName: "test1.local"})
	if err != nil {
		t.Fatal(err)
	}
	if string(again.Certificate[0]) != string(got.Certificate[0]) {
		t.Fatal("the leaf for test1.local was not reused")
	}
}

// TestSNISignerRefusesForeignNames: the constraint is what makes the root safe,
// so a name outside it fails in the signer itself, with the reason.
func TestSNISignerRefusesForeignNames(t *testing.T) {
	certPath, keyPath, _ := testCA(t, ".local")
	s, err := newSNISigner(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"example.com", "notlocal", "local"} {
		if _, err := s.getCertificate(&tls.ClientHelloInfo{ServerName: name}); err == nil {
			t.Fatalf("%s was signed, though the CA may only sign .local", name)
		}
	}
	if _, err := s.getCertificate(&tls.ClientHelloInfo{}); err == nil {
		t.Fatal("a handshake without ServerName was signed")
	}
	// Case and a trailing dot are the same name, as in every DNS comparison.
	if _, err := s.getCertificate(&tls.ClientHelloInfo{ServerName: "TEST1.Local."}); err != nil {
		t.Fatalf("TEST1.Local. was refused: %v", err)
	}
}

// TestSNISignerRefusesNonCA: a leaf key handed to --tls-ca is an error, not a
// chain that nothing verifies.
func TestSNISignerRefusesNonCA(t *testing.T) {
	certPath, keyPath, _ := testCA(t, ".local")
	// Rewrite the certificate as a leaf by signing it as not-a-CA.
	dir := t.TempDir()
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "leaf"}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"a.local"}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, leafKey.Public(), leafKey)
	if err != nil {
		t.Fatal(err)
	}
	leafPath := filepath.Join(dir, "leaf-cert.pem")
	if err := os.WriteFile(leafPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newSNISigner(leafPath, keyPath); err == nil || !strings.Contains(err.Error(), "not a CA") {
		t.Fatalf("a leaf was accepted as a CA: %v (cert %s)", err, certPath)
	}
}

// TestSNISignerHandshake: a real TLS handshake through the config, so the wiring
// (GetCertificate, MinVersion) is exercised, not only the signer.
func TestSNISignerHandshake(t *testing.T) {
	certPath, keyPath, ca := testCA(t, ".local")
	s, err := newSNISigner(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	pool := x509.NewCertPool()
	pool.AddCert(ca)
	go func() {
		tc := tls.Server(serverConn, s.tlsConfig())
		if err := tc.Handshake(); err != nil {
			return // the client reports the same failure
		}
		tc.Close()
	}()
	tc := tls.Client(clientConn, &tls.Config{ServerName: "test1.local", RootCAs: pool})
	if err := tc.Handshake(); err != nil {
		t.Fatalf("handshake for test1.local failed: %v", err)
	}
	tc.Close()
}
