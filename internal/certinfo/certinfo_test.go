package certinfo

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
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

// makeCert builds a certificate that is valid for names, signed by parent (or
// self-signed when parent is nil).
func makeCert(t *testing.T, cn string, names []string, isCA bool, notch time.Duration, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn, Organization: []string{"akmesh test"}},
		NotBefore:             time.Now().Add(notch),
		NotAfter:              time.Now().Add(notch + 24*time.Hour),
		DNSNames:              names,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	if parent == nil {
		parent, parentKey = tmpl, key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return c, key, der
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

// TestFileShowsEveryBlock: a bundle with two certificates prints both, in the
// order they appear.
func TestFileShowsEveryBlock(t *testing.T) {
	ca, caKey, _ := makeCert(t, "Test CA", nil, true, -time.Hour, nil, nil)
	_, _, leafDER := makeCert(t, "leaf.example", []string{"leaf.example"}, false, -time.Hour, ca, caKey)

	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.pem")
	var buf []byte
	buf = append(buf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})...)
	buf = append(buf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})...)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := run(t, path)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if n := strings.Count(out, "Subject: "); n != 2 {
		t.Fatalf("want two subjects, got %d in:\n%s", n, out)
	}
	if !strings.Contains(out, "CN=Test CA") || !strings.Contains(out, "CN=leaf.example") {
		t.Fatalf("subjects missing:\n%s", out)
	}
	if strings.Index(out, "Test CA") > strings.Index(out, "leaf.example") {
		t.Fatalf("the order of the blocks changed:\n%s", out)
	}
	if !strings.Contains(out, "CA: yes") {
		t.Fatalf("the CA flag is missing:\n%s", out)
	}
	if !strings.Contains(out, "SHA-256: ") || strings.Count(out, ":") < 10 {
		t.Fatalf("the fingerprint is missing:\n%s", out)
	}
}

// TestServerChainLeafFirst: against a TLS server the leaf comes first, then the
// issuer, which is what a reader walking the chain expects.
func TestServerChainLeafFirst(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	addr := server.Listener.Addr().String()

	out, errOut, code := run(t, addr)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	first := out
	if i := strings.Index(out, "\nSubject: "); i > 0 {
		first = out[:i]
	}
	want := server.Certificate().Subject.CommonName
	if !strings.Contains(first, want) {
		t.Fatalf("first block is not the leaf (want %q):\n%s", want, out)
	}
	// The leaf is proven by its fingerprint, not by the CA flag: Go's httptest
	// certificate is itself a CA, so IsCA says nothing about position.
	sum := sha256.Sum256(server.Certificate().Raw)
	var parts []string
	for _, b := range sum {
		parts = append(parts, strings.ToUpper(hex.EncodeToString([]byte{b})))
	}
	if !strings.Contains(first, "SHA-256: "+strings.Join(parts, ":")) {
		t.Fatalf("the first block is not the server's certificate:\n%s", first)
	}
	if !strings.Contains(first, "IP: 127.0.0.1") {
		t.Fatalf("the IP SAN is missing:\n%s", first)
	}
}

// TestJSONOutput: -j gives machine-readable output, and the fingerprint is the
// same string as in the text form.
func TestJSONOutput(t *testing.T) {
	_, _, der := makeCert(t, "json.example", []string{"json.example", "www.json.example"}, false, 0, nil, nil)
	path := filepath.Join(t.TempDir(), "one.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatal(err)
	}

	textOut, _, code := run(t, path)
	if code != 0 {
		t.Fatalf("text run: exit %d", code)
	}
	out, errOut, code := run(t, "-j", path)
	if code != 0 {
		t.Fatalf("json run: exit %d, stderr %q", code, errOut)
	}
	var doc struct {
		Source string `json:"source"`
		Certs  []cert `json:"certs"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if doc.Source != "file" || len(doc.Certs) != 1 {
		t.Fatalf("source %q, certs %d", doc.Source, len(doc.Certs))
	}
	if len(doc.Certs[0].DNSNames) != 2 {
		t.Fatalf("DNS names lost: %v", doc.Certs[0].DNSNames)
	}
	fp := strings.SplitN(textOut, "SHA-256: ", 2)[1]
	if !strings.HasPrefix(fp, doc.Certs[0].SHA256) {
		t.Fatalf("fingerprints differ: text %q, json %q", fp, doc.Certs[0].SHA256)
	}
}

// TestDERFile: a single DER certificate, as Windows exports .cer, is read too.
func TestDERFile(t *testing.T) {
	_, _, der := makeCert(t, "der.example", nil, false, 0, nil, nil)
	path := filepath.Join(t.TempDir(), "one.cer")
	if err := os.WriteFile(path, der, 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := run(t, path)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "CN=der.example") {
		t.Fatalf("DER certificate not read:\n%s", out)
	}
}

// TestRefusals pins the messages and exit codes; an agent reads them.
func TestRefusals(t *testing.T) {
	_, errOut, code := run(t)
	if code != 1 || !strings.Contains(errOut, "missing file or host") {
		t.Fatalf("no operand: exit %d, stderr %q", code, errOut)
	}

	_, errOut, code = run(t, "-z", "x")
	if code != 1 || !strings.Contains(errOut, "unrecognized option '-z'") {
		t.Fatalf("bad option: exit %d, stderr %q", code, errOut)
	}

	empty := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(empty, []byte("nicht base64\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code = run(t, empty)
	if code != 1 || !strings.Contains(errOut, "no certificate found") {
		t.Fatalf("garbage file: exit %d, stderr %q", code, errOut)
	}

	// A name that is neither a file nor reachable has to name both readings,
	// otherwise a mistyped file name looks like a network problem.
	// A name that does not resolve fails in milliseconds; 127.0.0.1:1 waits for
	// the full dial timeout here (WSL filters the refusal), which would make the
	// suite ten seconds slower for nothing.
	_, errOut, code = run(t, "no-such-host.invalid:443")
	if code != 1 || !strings.Contains(errOut, "no such file and not reachable as a host") {
		t.Fatalf("unreachable host: exit %d, stderr %q", code, errOut)
	}
}

// TestTLSStateHasCertificates guards the assumption the host path rests on.
func TestTLSStateHasCertificates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	conn, err := tls.Dial("tcp", server.Listener.Addr().String(), &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if len(conn.ConnectionState().PeerCertificates) == 0 {
		t.Fatal("the TLS server sent no certificate")
	}
}

// TestResolveReadsTheCertificateForALocalName: --resolve connects to the given
// address but keeps the name, so test1.local can be read without /etc/hosts
// (acmesh-730 follow-up).
func TestResolveReadsTheCertificateForALocalName(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port := u.Port()

	out, code := runCertinfo(t, "--resolve", "test1.local:"+port+":127.0.0.1", "test1.local:"+port)
	if code != 0 {
		t.Fatalf("exit %d, out %q", code, out)
	}
	if !strings.Contains(out, "Subject:") {
		t.Fatalf("no certificate read: %q", out)
	}

	// A malformed spec is refused, not guessed at.
	for _, bad := range []string{"test1.local", "test1.local:443:", ":::"} {
		if _, code := runCertinfo(t, "--resolve", bad, "test1.local:443"); code == 0 {
			t.Fatalf("--resolve %q was accepted", bad)
		}
	}
}

// runCertinfo runs the command with its streams captured.
func runCertinfo(t *testing.T, args ...string) (string, int) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errFile, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errFile.Close()
	code := Run(args, nil, out, errFile)
	data, err := os.ReadFile(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data), code
}
