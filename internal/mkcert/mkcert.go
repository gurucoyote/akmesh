// Package mkcert issues test certificates: a leaf for the names you give, and on
// request a small CA of your own that signs it.
//
// It writes PEM only, in the clear, and says so in its help: no PKCS#12, no
// passphrase, no hardware token. What it cannot do it refuses in plain words
// instead of failing somewhere in a library.
package mkcert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"akmesh/internal/winmode"
)

// caLife is how long a CA made here lasts: a test CA is not renewed every month,
// and a leaf that outlives it would be unusable.
const caLife = 10 * 365 * 24 * time.Hour

type options struct {
	hosts   []string
	days    int
	keyType string
	out     string
	ca      string
}

func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	o := options{days: 365, keyType: "ecdsa"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		value := func() (string, bool) {
			if j := strings.IndexByte(a, '='); j > 0 {
				return a[j+1:], true
			}
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "mkcert: option '%s' requires an argument\n", a)
				return "", false
			}
			i++
			return args[i], true
		}
		name := a
		if j := strings.IndexByte(a, '='); j > 0 {
			name = a[:j]
		}
		switch name {
		case "-host", "--host", "-H":
			v, ok := value()
			if !ok {
				return 1
			}
			for _, h := range strings.Split(v, ",") {
				if h = strings.TrimSpace(h); h != "" {
					o.hosts = append(o.hosts, h)
				}
			}
		case "-days", "--days":
			v, ok := value()
			if !ok {
				return 1
			}
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 || n > 3650 {
				fmt.Fprintf(stderr, "mkcert: -days needs a number between 1 and 3650, got '%s'\n", v)
				return 1
			}
			o.days = n
		case "-key-type", "--key-type", "-key":
			v, ok := value()
			if !ok {
				return 1
			}
			switch v {
			case "ecdsa", "rsa", "ed25519":
				o.keyType = v
			default:
				fmt.Fprintf(stderr, "mkcert: -key-type must be ecdsa, rsa or ed25519, got '%s'\n", v)
				return 1
			}
		case "-out", "--out", "-o":
			v, ok := value()
			if !ok {
				return 1
			}
			o.out = v
		case "-ca", "--ca":
			v, ok := value()
			if !ok {
				return 1
			}
			o.ca = v
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(stderr, "mkcert: unrecognized option '%s'\n", a)
				fmt.Fprintln(stderr, "Try 'mkcert --help' for more information.")
				return 1
			}
			fmt.Fprintf(stderr, "mkcert: unexpected operand '%s'\n", a)
			return 1
		}
	}
	if len(o.hosts) == 0 {
		fmt.Fprintln(stderr, "mkcert: no name to certify, use -host NAME[,NAME...]")
		return 1
	}
	if o.ca != "" && strings.HasSuffix(o.ca, ".pem") {
		// A CA is addressed by its base name, so the two files can be found.
		o.ca = strings.TrimSuffix(o.ca, ".pem")
	}
	if o.out == "" {
		o.out = strings.NewReplacer("*", "wildcard", ":", "_", "/", "_").Replace(o.hosts[0])
	}

	if err := issue(o, stdout); err != nil {
		fmt.Fprintf(stderr, "mkcert: %v\n", err)
		return 1
	}
	return 0
}

// issue makes the leaf, and the CA first when one is asked for or needed.
func issue(o options, stdout *os.File) error {
	var parent *x509.Certificate
	var parentKey crypto.Signer

	if o.ca != "" {
		caCert, caKey, err := loadOrCreateCA(o.ca)
		if err != nil {
			return err
		}
		parent, parentKey = caCert, caKey
	} else {
		// Without -ca the certificate signs itself; it is honest about that and
		// still carries the names, which is what a quick test needs.
		parent = nil
	}

	signer, err := newKey(o.keyType)
	if err != nil {
		return err
	}

	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: o.hosts[0], Organization: []string{"akmesh mkcert"}},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Duration(o.days) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range o.hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	signWith, signKey := tmpl, crypto.Signer(signer)
	if parent != nil {
		signWith, signKey = parent, parentKey
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signWith, signer.Public(), signKey)
	if err != nil {
		return err
	}

	certPath := o.out + "-cert.pem"
	keyPath := o.out + "-key.pem"
	if err := writePEM(certPath, "CERTIFICATE", der, 0o644); err != nil {
		return err
	}
	// 0600 before anything is written: the key must never exist as a readable
	// file, not even for a moment.
	if err := writeKey(keyPath, signer); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s\nwrote %s\n", certPath, keyPath)
	if parent != nil && isSelfSigned(parent) {
		fmt.Fprintf(stdout, "signed by %s-cert.pem\n", o.ca)
	}
	return nil
}

// loadOrCreateCA reads BASE-cert.pem and BASE-key.pem, or makes both. An existing
// half pair is an error: guessing which half is meant would issue certificates
// nobody can verify.
func loadOrCreateCA(base string) (*x509.Certificate, crypto.Signer, error) {
	certPath, keyPath := base+"-cert.pem", base+"-key.pem"
	certPEM, certErr := os.ReadFile(certPath)
	keyPEM, keyErr := os.ReadFile(keyPath)
	switch {
	case certErr == nil && keyErr == nil:
		cert, err := parseFirstCert(certPEM)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", certPath, err)
		}
		key, err := parseKey(keyPEM)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", keyPath, err)
		}
		return cert, key, nil
	case certErr != nil && keyErr != nil:
		// Fall through to creating the pair.
	case certErr != nil:
		return nil, nil, fmt.Errorf("%s exists but %s does not", keyPath, certPath)
	default:
		return nil, nil, fmt.Errorf("%s exists but %s does not", certPath, keyPath)
	}

	signer, err := newKey("ecdsa")
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "akmesh test CA " + filepath.Base(base), Organization: []string{"akmesh mkcert"}},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(caLife),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        false,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, signer.Public(), signer)
	if err != nil {
		return nil, nil, err
	}
	if err := writePEM(certPath, "CERTIFICATE", der, 0o644); err != nil {
		return nil, nil, err
	}
	if err := writeKey(keyPath, signer); err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return cert, signer, nil
}

func newKey(kind string) (crypto.Signer, error) {
	switch kind {
	case "rsa":
		return rsa.GenerateKey(rand.Reader, 2048)
	case "ed25519":
		_, k, err := ed25519.GenerateKey(rand.Reader)
		return k, err
	default:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
}

// serial is a random 128-bit number: guessing it is what turns a test CA into a
// liability, so it is not a counter.
func serial() *big.Int {
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return n
}

func writePEM(path, kind string, der []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if err := pem.Encode(f, &pem.Block{Type: kind, Bytes: der}); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// os.Chmod above only toggles the read-only attribute on Windows; the mode
	// itself is recorded the way chmod records it (acmesh-m6x).
	return winmode.SetQuiet(path, perm)
}

// writeKey writes the private key as PKCS#8, unencrypted: the standard library
// cannot encrypt it today, and pretending otherwise would be the dangerous part.
// The 0600 is real on Windows too, through the same ACL winmode records for
// chmod (acmesh-m6x); on a filesystem without DACLs nothing can be done, and
// that is the one case that stays quiet.
func writeKey(path string, key crypto.Signer) error {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := writePEM(path, "PRIVATE KEY", der, 0o600); err != nil {
		return err
	}
	return winmode.SetQuiet(path, 0o600)
}

func parseFirstCert(data []byte) (*x509.Certificate, error) {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("no certificate found")
		}
		if block.Type == "CERTIFICATE" {
			return x509.ParseCertificate(block.Bytes)
		}
	}
}

func parseKey(data []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if block.Type == "ENCRYPTED PRIVATE KEY" {
		return nil, fmt.Errorf("this key is encrypted, which mkcert cannot read: no passphrase support")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		if k, e2 := x509.ParsePKCS1PrivateKey(block.Bytes); e2 == nil {
			return k, nil
		}
		if k, e2 := x509.ParseECPrivateKey(block.Bytes); e2 == nil {
			return k, nil
		}
		return nil, err
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("this key type cannot sign")
	}
	return signer, nil
}

func isSelfSigned(c *x509.Certificate) bool {
	return c.Issuer.String() == c.Subject.String()
}
