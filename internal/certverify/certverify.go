// Package certverify checks a certificate chain, the third of the certificate
// commands: given a certificate and, optionally, the CA it should lead to, it
// says yes or names the reason for no.
//
// What it does not do, and says in its help: OCSP, CRL and Certificate
// Transparency are not consulted, so a revoked certificate passes here.
package certverify

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"
)

func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var caFiles []string
	host := ""
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-ca" || a == "--ca":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "verify: option '-ca' requires an argument")
				return 2
			}
			i++
			caFiles = append(caFiles, args[i])
		case a == "-host" || a == "--host":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "verify: option '-host' requires an argument")
				return 2
			}
			i++
			host = args[i]
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(stderr, "verify: unrecognized option '%s'\n", a)
			fmt.Fprintln(stderr, "Try 'verify --help' for more information.")
			return 2
		default:
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "verify: missing certificate to verify")
		fmt.Fprintln(stderr, "Try 'verify --help' for more information.")
		return 2
	}

	leaf, err := firstCert(files[0])
	if err != nil {
		fmt.Fprintf(stderr, "verify: %s: %v\n", files[0], err)
		return 1
	}
	intermediates := x509.NewCertPool()
	for _, f := range files[1:] {
		certs, err := allCerts(f)
		if err != nil {
			fmt.Fprintf(stderr, "verify: %s: %v\n", f, err)
			return 1
		}
		for _, c := range certs {
			intermediates.AddCert(c)
		}
	}

	var roots *x509.CertPool
	if len(caFiles) > 0 {
		roots = x509.NewCertPool()
		for _, f := range caFiles {
			certs, err := allCerts(f)
			if err != nil {
				fmt.Fprintf(stderr, "verify: %s: %v\n", f, err)
				return 1
			}
			for _, c := range certs {
				roots.AddCert(c)
			}
		}
	} else {
		// System roots are the default, like curl without -k or --cacert.
		pool, err := x509.SystemCertPool()
		if err != nil {
			fmt.Fprintf(stderr, "verify: system roots: %v\n", err)
			return 1
		}
		roots = pool
	}

	_, err = leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		DNSName:       host,
		CurrentTime:   time.Now(),
	})
	if err != nil {
		// The reason is what an agent acts on, so it is printed as it comes and
		// not folded into "verification failed".
		fmt.Fprintf(stderr, "verify: %v\n", err)
		return 1
	}
	if host != "" && host != leaf.Subject.CommonName {
		fmt.Fprintf(stdout, "OK: %s matches %s and chains to a trusted root\n", leaf.Subject.CommonName, host)
	} else {
		fmt.Fprintf(stdout, "OK: %s chains to a trusted root\n", leaf.Subject.CommonName)
	}
	return 0
}

func firstCert(path string) (*x509.Certificate, error) {
	certs, err := allCerts(path)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificate found")
	}
	return certs[0], nil
}

// allCerts reads every CERTIFICATE block in order; a file without PEM is tried
// as DER, so a .cer straight from Windows works.
func allCerts(path string) ([]*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []*x509.Certificate
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
		out = append(out, c)
	}
	if len(out) > 0 {
		return out, nil
	}
	if c, err := x509.ParseCertificate(data); err == nil {
		return []*x509.Certificate{c}, nil
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, fmt.Errorf("empty input")
	}
	return nil, fmt.Errorf("no certificate found in PEM or DER form")
}
