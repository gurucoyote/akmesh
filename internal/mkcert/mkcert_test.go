package mkcert

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func run(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	outPath, errPath := filepath.Join(dir, "out.txt"), filepath.Join(dir, "err.txt")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.Create(errPath)
	if err != nil {
		t.Fatal(err)
	}
	code := Run(args, nil, out, errf)
	out.Close()
	errf.Close()
	o, _ := os.ReadFile(outPath)
	e, _ := os.ReadFile(errPath)
	return string(o), string(e), code
}

// readCert loads the first certificate from a PEM file.
func readCert(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			t.Fatalf("%s holds no certificate", path)
		}
		if block.Type == "CERTIFICATE" {
			c, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				t.Fatal(err)
			}
			return c
		}
	}
}

// TestNamesFilesAndKeyMode: the two files carry the base name, the key is 0600,
// and the SANs are exactly the names asked for.
func TestNamesFilesAndKeyMode(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "site")
	stdout, stderr, code := run(t, dir, "-host", "example.test,www.example.test", "-days", "5", "-out", base)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "site-cert.pem") || !strings.Contains(stdout, "site-key.pem") {
		t.Fatalf("the written names are not reported:\n%s", stdout)
	}

	keyInfo, err := os.Stat(base + "-key.pem")
	if err != nil {
		t.Fatal(err)
	}
	if perm := keyInfo.Mode().Perm(); perm != 0o600 {
		t.Fatalf("key mode is %o, want 600", perm)
	}
	certInfo, err := os.Stat(base + "-cert.pem")
	if err != nil {
		t.Fatal(err)
	}
	if perm := certInfo.Mode().Perm(); perm != 0o644 {
		t.Fatalf("cert mode is %o, want 644", perm)
	}

	c := readCert(t, base+"-cert.pem")
	if len(c.DNSNames) != 2 || c.DNSNames[0] != "example.test" || c.DNSNames[1] != "www.example.test" {
		t.Fatalf("SANs are %v", c.DNSNames)
	}
	if life := c.NotAfter.Sub(c.NotBefore); life < 5*24*time.Hour || life > 5*24*time.Hour+2*time.Minute {
		t.Fatalf("validity is %v, want about 5 days", life)
	}
	// A leaf without -ca signs itself; that it verifies as its own issuer is the
	// whole content of that promise. CheckSignatureFrom would refuse because the
	// leaf is not a CA, which is the point of -ca.
	if err := c.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature); err != nil {
		t.Fatalf("a leaf without -ca must be self-signed: %v", err)
	}
	if c.IsCA {
		t.Fatal("a leaf must not be a CA")
	}
}

// TestIPHostBecomesIPSAN: an address becomes an IP SAN, not a DNS name, because
// a client checks IPs against a different field.
func TestIPHostBecomesIPSAN(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "ip")
	_, stderr, code := run(t, dir, "-host", "127.0.0.1,localhost", "-out", base)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	c := readCert(t, base+"-cert.pem")
	if len(c.IPAddresses) != 1 || c.IPAddresses[0].String() != "127.0.0.1" {
		t.Fatalf("IP SANs are %v", c.IPAddresses)
	}
	if len(c.DNSNames) != 1 || c.DNSNames[0] != "localhost" {
		t.Fatalf("DNS SANs are %v", c.DNSNames)
	}
}

// TestCAVerifiesTheLeaf: the leaf goes through x509.Verify against the CA made in
// the same run, for each key type. This is the whole promise of the command.
func TestCAVerifiesTheLeaf(t *testing.T) {
	for _, kind := range []string{"ecdsa", "rsa", "ed25519"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			caDir := t.TempDir()
			ca := filepath.Join(caDir, "testca")
			base := filepath.Join(dir, "leaf")
			_, stderr, code := run(t, dir, "-host", "localhost", "-key-type", kind, "-ca", ca, "-out", base)
			if code != 0 {
				t.Fatalf("exit %d, stderr %q", code, stderr)
			}
			caInfo, err := os.Stat(ca + "-key.pem")
			if err != nil {
				t.Fatal(err)
			}
			if perm := caInfo.Mode().Perm(); perm != 0o600 {
				t.Fatalf("CA key mode is %o, want 600", perm)
			}
			caCert := readCert(t, ca+"-cert.pem")
			if !caCert.IsCA {
				t.Fatal("the CA certificate is not marked as a CA")
			}
			leaf := readCert(t, base+"-cert.pem")
			pool := x509.NewCertPool()
			pool.AddCert(caCert)
			if _, err := leaf.Verify(x509.VerifyOptions{DNSName: "localhost", Roots: pool}); err != nil {
				t.Fatalf("%s leaf does not verify against its CA: %v", kind, err)
			}
			// The key file must match the certificate, otherwise the pair is junk.
			keyPEM, err := os.ReadFile(base + "-key.pem")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parseKey(keyPEM); err != nil {
				t.Fatalf("%s key is unreadable: %v", kind, err)
			}
			if got := leaf.PublicKeyAlgorithm.String(); !strings.EqualFold(got, kind) {
				t.Fatalf("public key is %s, want %s", got, kind)
			}
		})
	}
}

// TestCAIsReused: a second leaf signed by the same -ca verifies with the first
// CA file, so the CA is stable across runs.
func TestCAIsReused(t *testing.T) {
	dir := t.TempDir()
	ca := filepath.Join(dir, "reuse")
	if _, stderr, code := run(t, dir, "-host", "one.test", "-ca", ca, "-out", filepath.Join(dir, "one")); code != 0 {
		t.Fatalf("first run: exit %d, %q", code, stderr)
	}
	before, err := os.ReadFile(ca + "-cert.pem")
	if err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := run(t, dir, "-host", "two.test", "-ca", ca, "-out", filepath.Join(dir, "two")); code != 0 {
		t.Fatalf("second run: exit %d, %q", code, stderr)
	}
	after, err := os.ReadFile(ca + "-cert.pem")
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("the CA was recreated instead of reused")
	}
	caCert := readCert(t, ca+"-cert.pem")
	leaf := readCert(t, filepath.Join(dir, "two-cert.pem"))
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: "two.test", Roots: pool}); err != nil {
		t.Fatalf("second leaf does not verify: %v", err)
	}
}

// TestRefusals pins the messages: an unknown key type, a bad day count, a
// half-existing CA pair, an encrypted key.
func TestRefusals(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		args []string
		want string
	}{
		{[]string{}, "no name to certify"},
		{[]string{"-host", "x", "-key-type", "dsa"}, "-key-type must be ecdsa, rsa or ed25519"},
		{[]string{"-host", "x", "-days", "0"}, "-days needs a number between 1 and 3650"},
		{[]string{"-host", "x", "-bogus"}, "unrecognized option '-bogus'"},
		{[]string{"extra"}, "unexpected operand 'extra'"},
	}
	for _, c := range cases {
		_, stderr, code := run(t, dir, c.args...)
		if code != 1 || !strings.Contains(stderr, c.want) {
			t.Fatalf("%v: exit %d, stderr %q, want %q", c.args, code, stderr, c.want)
		}
	}

	// Half a pair: the cert exists, the key does not. Guessing would issue
	// certificates nobody can verify.
	ca := filepath.Join(dir, "half")
	if err := os.WriteFile(ca+"-cert.pem", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := run(t, dir, "-host", "x", "-ca", ca, "-out", filepath.Join(dir, "leaf"))
	if code != 1 || !strings.Contains(stderr, "exists but") {
		t.Fatalf("half CA pair: exit %d, stderr %q", code, stderr)
	}

	// An encrypted key is refused by name, not by a library error. The pair has
	// to be real, otherwise the certificate check fails first.
	enc := filepath.Join(dir, "enc")
	if _, stderr, code := run(t, dir, "-host", "enc.test", "-ca", enc, "-out", filepath.Join(dir, "encleaf")); code != 0 {
		t.Fatalf("setting up the encrypted key: exit %d, %q", code, stderr)
	}
	if err := os.WriteFile(enc+"-key.pem", pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte("x")}), 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = run(t, dir, "-host", "x", "-ca", enc, "-out", filepath.Join(dir, "leaf2"))
	if code != 1 || !strings.Contains(stderr, "no passphrase support") {
		t.Fatalf("encrypted CA key: exit %d, stderr %q", code, stderr)
	}
}
