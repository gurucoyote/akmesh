// Local addition, no upstream source: TLS certificates signed per SNI name from
// a local CA (acmesh-816).
//
// The point is a name like test1.local, which no public CA will ever certify:
// the machine trusts one local root, and httpd signs a leaf for whatever name the
// client asked for in the handshake. That is what Kiri does for *.kiri.local, and
// what makes a test host reachable over HTTPS without mkcert on every port.
package httpd

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"
)

// leafLife is how long a signed leaf lasts. 390 days stays under the 825-day cap
// macOS puts on server certificates, and a leaf that outlives its CA is useless
// anyway - our CAs last ten years.
const leafLife = 390 * 24 * time.Hour

// sniSigner signs a leaf per name, on first use, and keeps it while the process
// runs.
type sniSigner struct {
	ca  *x509.Certificate
	key crypto.Signer

	mu    sync.Mutex
	cache map[string]*tls.Certificate
}

// newSNISigner reads the CA pair. A file that is not a CA is refused: silently
// signing with a leaf key would produce chains nothing verifies.
func newSNISigner(certPath, keyPath string) (*sniSigner, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("%s: no PEM certificate", certPath)
	}
	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", certPath, err)
	}
	if !ca.IsCA {
		return nil, fmt.Errorf("%s is not a CA certificate; --tls-ca needs a root with its key", certPath)
	}
	signer, err := parseSigner(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", keyPath, err)
	}
	return &sniSigner{ca: ca, key: signer, cache: map[string]*tls.Certificate{}}, nil
}

// tlsConfig is the config handed to the server.
func (s *sniSigner) tlsConfig() *tls.Config {
	return &tls.Config{GetCertificate: s.getCertificate, MinVersion: tls.VersionTLS12}
}

// getCertificate is the handshake hook: sign for the requested name, or refuse.
func (s *sniSigner) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	name := strings.ToLower(strings.TrimSuffix(hello.ServerName, "."))
	if name == "" {
		return nil, fmt.Errorf("no ServerName in the handshake, and a local CA cannot sign for 'anything'")
	}
	if !s.permitted(name) {
		return nil, fmt.Errorf("%s is not covered by the CA's name constraints (%s)",
			name, strings.Join(s.ca.PermittedDNSDomains, ", "))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.cache[name]; ok {
		return c, nil
	}
	c, err := s.sign(name)
	if err != nil {
		return nil, err
	}
	s.cache[name] = c
	return c, nil
}

// permitted checks the name against the CA's constraints, the same rule a client
// applies when it verifies the chain - so the refusal comes with a sentence
// instead of a handshake error nobody can read.
func (s *sniSigner) permitted(name string) bool {
	permitted := s.ca.PermittedDNSDomains
	if len(permitted) == 0 {
		return true // an unconstrained CA signs what it is asked for
	}
	for _, suffix := range permitted {
		suffix = strings.ToLower(suffix)
		if strings.HasSuffix(name, suffix) {
			// ".local" must not match "notlocal": the dot is the boundary.
			if strings.HasPrefix(suffix, ".") || name == suffix {
				return true
			}
		}
	}
	return false
}

// sign makes one leaf for one name: ECDSA P-256, the name as SAN, 390 days.
func (s *sniSigner) sign(name string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkixName(name),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(leafLife),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{name},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, s.ca, key.Public(), s.key)
	if err != nil {
		return nil, err
	}
	return &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}

// pkixName is the subject a leaf carries. The name also sits in the SAN, which
// is what a client checks; the CN stays for the tools that still show it.
func pkixName(name string) pkix.Name {
	return pkix.Name{CommonName: name, Organization: []string{"akmesh httpd local CA"}}
}

// parseSigner reads the PKCS#8, EC or PKCS#1 key a PEM file holds.
func parseSigner(keyPEM []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM private key")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if signer, ok := k.(crypto.Signer); ok {
			return signer, nil
		}
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, fmt.Errorf("unreadable private key")
}
