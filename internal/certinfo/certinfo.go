// Package certinfo prints what a certificate says and who signed it: the first
// of the three certificate commands. It only reads; whether a chain is valid is
// the business of verify.
//
// Nothing is hidden: a field Windows or a server omits is left out, not filled
// with a plausible value.
package certinfo

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// cert is one certificate in the output. The names follow what openssl calls
// the fields, so a reader who knows -text finds them again.
type cert struct {
	Subject            string   `json:"subject"`
	Issuer             string   `json:"issuer"`
	Serial             string   `json:"serial"`
	NotBefore          string   `json:"not_before"`
	NotAfter           string   `json:"not_after"`
	PublicKeyAlgorithm string   `json:"public_key"`
	SignatureAlgorithm string   `json:"signature"`
	IsCA               bool     `json:"is_ca"`
	DNSNames           []string `json:"dns_names,omitempty"`
	IPAddresses        []string `json:"ip_addresses,omitempty"`
	SHA256             string   `json:"sha256"`
}

func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	asJSON := false
	var subject string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-j", "--json":
			asJSON = true
		case "--":
			if i+1 < len(args) {
				subject = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(stderr, "certinfo: unrecognized option '%s'\n", args[i])
				fmt.Fprintln(stderr, "Try 'certinfo --help' for more information.")
				return 1
			}
			if subject != "" {
				fmt.Fprintf(stderr, "certinfo: extra operand '%s'\n", args[i])
				return 1
			}
			subject = args[i]
		}
	}
	if subject == "" {
		fmt.Fprintln(stderr, "certinfo: missing file or host")
		fmt.Fprintln(stderr, "Try 'certinfo --help' for more information.")
		return 1
	}

	certs, source, err := load(subject)
	if err != nil {
		fmt.Fprintf(stderr, "certinfo: %v\n", err)
		return 1
	}
	if asJSON {
		if err := writeJSON(stdout, source, certs); err != nil {
			fmt.Fprintf(stderr, "certinfo: %v\n", err)
			return 1
		}
		return 0
	}
	for i, c := range certs {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		writeText(stdout, c)
	}
	return 0
}

// load decides by the operand: a path that exists is a file, everything else is
// a host. A host that cannot be dialled says so with both readings, because
// "connection refused" alone hides a mistyped file name.
func load(subject string) ([]cert, string, error) {
	if _, err := os.Stat(subject); err == nil {
		data, err := os.ReadFile(subject)
		if err != nil {
			return nil, "", err
		}
		certs, err := parse(data)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %v", subject, err)
		}
		return certs, "file", nil
	}
	host, port := subject, "443"
	if strings.Contains(subject, ":") {
		h, p, err := net.SplitHostPort(subject)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %v", subject, err)
		}
		host, port = h, p
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp",
		net.JoinHostPort(host, port), &tls.Config{ServerName: host, InsecureSkipVerify: true})
	if err != nil {
		return nil, "", fmt.Errorf("%s: no such file and not reachable as a host: %v", subject, err)
	}
	defer conn.Close()
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, "", fmt.Errorf("%s: the server sent no certificate", subject)
	}
	var out []cert
	for _, c := range state.PeerCertificates {
		out = append(out, describe(c))
	}
	return out, "host", nil
}

// parse reads every PEM block in order; a file without PEM is tried as DER, so a
// single .cer from Windows works too.
func parse(data []byte) ([]cert, error) {
	var out []cert
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		out = append(out, describe(c))
	}
	if len(out) > 0 {
		return out, nil
	}
	if c, err := x509.ParseCertificate(data); err == nil {
		return []cert{describe(c)}, nil
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	return nil, fmt.Errorf("no certificate found in PEM or DER form")
}

func describe(c *x509.Certificate) cert {
	sum := sha256.Sum256(c.Raw)
	fingerprint := make([]string, len(sum))
	for i, b := range sum {
		fingerprint[i] = strings.ToUpper(hex.EncodeToString([]byte{b}))
	}
	ips := make([]string, 0, len(c.IPAddresses))
	for _, ip := range c.IPAddresses {
		ips = append(ips, ip.String())
	}
	return cert{
		Subject:            c.Subject.String(),
		Issuer:             c.Issuer.String(),
		Serial:             strings.ToUpper(c.SerialNumber.Text(16)),
		NotBefore:          c.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:           c.NotAfter.UTC().Format(time.RFC3339),
		PublicKeyAlgorithm: c.PublicKeyAlgorithm.String(),
		SignatureAlgorithm: c.SignatureAlgorithm.String(),
		IsCA:               c.IsCA,
		DNSNames:           c.DNSNames,
		IPAddresses:        ips,
		SHA256:             strings.Join(fingerprint, ":"),
	}
}

func writeText(w io.Writer, c cert) {
	fmt.Fprintf(w, "Subject: %s\n", c.Subject)
	fmt.Fprintf(w, "Issuer: %s\n", c.Issuer)
	fmt.Fprintf(w, "Serial: %s\n", c.Serial)
	fmt.Fprintf(w, "Validity: %s to %s\n", c.NotBefore, c.NotAfter)
	fmt.Fprintf(w, "Public key: %s\n", c.PublicKeyAlgorithm)
	fmt.Fprintf(w, "Signature: %s\n", c.SignatureAlgorithm)
	fmt.Fprintf(w, "CA: %s\n", yesNo(c.IsCA))
	if len(c.DNSNames) > 0 {
		fmt.Fprintf(w, "DNS: %s\n", strings.Join(c.DNSNames, ", "))
	}
	if len(c.IPAddresses) > 0 {
		fmt.Fprintf(w, "IP: %s\n", strings.Join(c.IPAddresses, ", "))
	}
	fmt.Fprintf(w, "SHA-256: %s\n", c.SHA256)
}

func writeJSON(w io.Writer, source string, certs []cert) error {
	doc := struct {
		Source string `json:"source"`
		Certs  []cert `json:"certs"`
	}{source, certs}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
