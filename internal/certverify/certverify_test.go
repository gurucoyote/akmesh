package certverify

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pair is a certificate and the files that hold it.
type pair struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// makeCert builds a certificate for names, signed by parent, valid from
// notBefore for a day.
func makeCert(t *testing.T, cn string, names []string, isCA bool, notBefore time.Time, parent *pair) pair {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(24 * time.Hour),
		DNSNames:              names,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	parentCert, parentKey := tmpl, key
	if parent != nil {
		parentCert, parentKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parentCert, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return pair{cert: c, key: key}
}

func write(t *testing.T, dir, name string, certs ...*x509.Certificate) string {
	t.Helper()
	path := filepath.Join(dir, name)
	var buf []byte
	for _, c := range certs {
		buf = append(buf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	outPath, errPath := filepath.Join(dir, "out"), filepath.Join(dir, "err")
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

// TestValidLeafAgainstItsCA: the happy path, and the exit code an agent checks.
func TestValidLeafAgainstItsCA(t *testing.T) {
	dir := t.TempDir()
	ca := makeCert(t, "Test CA", nil, true, time.Now().Add(-time.Hour), nil)
	leaf := makeCert(t, "leaf.test", []string{"leaf.test"}, false, time.Now().Add(-time.Hour), &ca)
	caPath := write(t, dir, "ca.pem", ca.cert)
	leafPath := write(t, dir, "leaf.pem", leaf.cert)

	out, errOut, code := run(t, "-ca", caPath, "-host", "leaf.test", leafPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "OK: leaf.test chains to a trusted root") {
		t.Fatalf("output is %q", out)
	}

	// Without -host the chain is checked, the name is not.
	if _, errOut, code := run(t, "-ca", caPath, leafPath); code != 0 {
		t.Fatalf("without -host: exit %d, stderr %q", code, errOut)
	}
}

// TestExpiredCertificate: an expired leaf fails and says so.
func TestExpiredCertificate(t *testing.T) {
	dir := t.TempDir()
	ca := makeCert(t, "Test CA", nil, true, time.Now().Add(-72*time.Hour), nil)
	leaf := makeCert(t, "old.test", []string{"old.test"}, false, time.Now().Add(-48*time.Hour), &ca)
	caPath := write(t, dir, "ca.pem", ca.cert)
	leafPath := write(t, dir, "old.pem", leaf.cert)

	_, errOut, code := run(t, "-ca", caPath, "-host", "old.test", leafPath)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "expired") {
		t.Fatalf("the reason is not named: %q", errOut)
	}
}

// TestWrongHost: the chain is fine, the name is not.
func TestWrongHost(t *testing.T) {
	dir := t.TempDir()
	ca := makeCert(t, "Test CA", nil, true, time.Now().Add(-time.Hour), nil)
	leaf := makeCert(t, "good.test", []string{"good.test"}, false, time.Now().Add(-time.Hour), &ca)
	caPath := write(t, dir, "ca.pem", ca.cert)
	leafPath := write(t, dir, "leaf.pem", leaf.cert)

	_, errOut, code := run(t, "-ca", caPath, "-host", "other.test", leafPath)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "other.test") {
		t.Fatalf("the host name is not named: %q", errOut)
	}
}

// TestUnknownAuthority: the leaf chains to a CA that is not the one given.
func TestUnknownAuthority(t *testing.T) {
	dir := t.TempDir()
	ourCA := makeCert(t, "Our CA", nil, true, time.Now().Add(-time.Hour), nil)
	otherCA := makeCert(t, "Other CA", nil, true, time.Now().Add(-time.Hour), nil)
	leaf := makeCert(t, "leaf.test", []string{"leaf.test"}, false, time.Now().Add(-time.Hour), &otherCA)
	ourPath := write(t, dir, "our.pem", ourCA.cert)
	leafPath := write(t, dir, "leaf.pem", leaf.cert)

	_, errOut, code := run(t, "-ca", ourPath, leafPath)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "unknown authority") {
		t.Fatalf("the reason is not named: %q", errOut)
	}
}

// TestChainFile: the issuer is given as a second file, and the leaf verifies
// through it up to the CA.
func TestChainFile(t *testing.T) {
	dir := t.TempDir()
	root := makeCert(t, "Root CA", nil, true, time.Now().Add(-time.Hour), nil)
	inter := makeCert(t, "Intermediate", nil, true, time.Now().Add(-time.Hour), &root)
	leaf := makeCert(t, "chain.test", []string{"chain.test"}, false, time.Now().Add(-time.Hour), &inter)
	rootPath := write(t, dir, "root.pem", root.cert)
	interPath := write(t, dir, "inter.pem", inter.cert)
	leafPath := write(t, dir, "leaf.pem", leaf.cert)

	_, errOut, code := run(t, "-ca", rootPath, "-host", "chain.test", leafPath, interPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	// Without the intermediate the chain is incomplete, and the message says so.
	_, errOut, code = run(t, "-ca", rootPath, "-host", "chain.test", leafPath)
	if code != 1 || !strings.Contains(errOut, "unknown authority") {
		t.Fatalf("without the intermediate: exit %d, stderr %q", code, errOut)
	}
}

// TestExitCodesAndMessages pins 2 for usage errors and 1 for a failed check.
func TestExitCodesAndMessages(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{}, "missing certificate to verify"},
		{[]string{"-bogus", "x"}, "unrecognized option '-bogus'"},
		{[]string{"-ca"}, "option '-ca' requires an argument"},
		{[]string{"-host"}, "option '-host' requires an argument"},
	} {
		_, errOut, code := run(t, c.args...)
		if code != 2 || !strings.Contains(errOut, c.want) {
			t.Fatalf("%v: exit %d, stderr %q, want %q", c.args, code, errOut, c.want)
		}
	}

	dir := t.TempDir()
	junk := filepath.Join(dir, "junk.pem")
	if err := os.WriteFile(junk, []byte("kein Zertifikat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := run(t, junk)
	if code != 1 || !strings.Contains(errOut, "no certificate found") {
		t.Fatalf("junk: exit %d, stderr %q", code, errOut)
	}

	missing := filepath.Join(dir, "nope.pem")
	_, errOut, code = run(t, missing)
	if code != 1 || !strings.Contains(errOut, "No such file") && !strings.Contains(errOut, "no such file") {
		t.Fatalf("missing file: exit %d, stderr %q", code, errOut)
	}
}
