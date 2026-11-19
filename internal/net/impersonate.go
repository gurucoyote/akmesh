//go:build !nonet

package net

// --impersonate: hand the TLS ClientHello to uTLS, so the handshake looks like
// a named browser instead of Go's. Measured on 2026-11-19: the minimal client
// grows by 1.4 MB, akmesh by 1.27 MB, and the published Chrome JA4 comes out
// byte-exact. The HTTP/2 fingerprint does not change - it stays Go's, and this
// client speaks HTTP/1.1 anyway (acmesh-agy).
//
// One deliberate edit to the preset: the ALPN list is cut down to http/1.1 and
// the encrypted ALPS extension is dropped. Otherwise the handshake would
// promise h2 and then speak HTTP/1.1, which is exactly the mismatch a
// fingerprinting service looks for. Measured before this edit: ja4 said "h2"
// in the ALPN field while the request ran on HTTP/1.1.
//
// curl itself is not among the profiles: its handshake comes from OpenSSL or
// NSS, which uTLS does not reimplement. Asking for it says so.

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
)

// impersonationProfile is the accepted spelling, with the uTLS id behind it.
func impersonationProfile(name string) (utls.ClientHelloID, bool) {
	switch strings.ToLower(name) {
	case "chrome":
		return utls.HelloChrome_Auto, true
	case "edge", "chromium":
		return utls.HelloEdge_Auto, true
	case "firefox":
		return utls.HelloFirefox_Auto, true
	case "safari":
		return utls.HelloSafari_Auto, true
	case "go", "golang":
		return utls.HelloGolang, true
	}
	return utls.ClientHelloID{}, false
}

// httpOneSpec turns a browser preset into a spec that only offers http/1.1 and
// carries no HTTP/2 settings.
func httpOneSpec(id utls.ClientHelloID) (*utls.ClientHelloSpec, error) {
	spec, err := utls.UTLSIdToSpec(id)
	if err != nil {
		return nil, fmt.Errorf("preset %s: %v", id.Str(), err)
	}
	kept := make([]utls.TLSExtension, 0, len(spec.Extensions))
	for _, ext := range spec.Extensions {
		switch e := ext.(type) {
		case *utls.ALPNExtension:
			kept = append(kept, &utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}})
		case *utls.ApplicationSettingsExtension, *utls.ApplicationSettingsExtensionNew:
			// ALPS carries HTTP/2 SETTINGS; without h2 it would be a lie.
			continue
		case *utls.GenericExtension:
			// Older uTLS keeps ALPS as raw data under extension 17613.
			if e.Id == 17613 {
				continue
			}
			kept = append(kept, ext)
		default:
			kept = append(kept, ext)
		}
	}
	spec.Extensions = kept
	return &spec, nil
}

// impersonateDialer builds the DialTLSContext function that speaks the chosen
// ClientHello.
func impersonateDialer(name string, insecure bool, timeout time.Duration) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	id, ok := impersonationProfile(name)
	if !ok {
		return nil, fmt.Errorf("unknown impersonation profile %q (chrome, edge, firefox, safari, go)", name)
	}
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := &net.Dialer{Timeout: 30 * time.Second}
		if timeout > 0 {
			d.Timeout = timeout
		}
		raw, err := d.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			raw.Close()
			return nil, err
		}
		// The spec is rebuilt per connection, because ApplyPreset may mutate it.
		spec, err := httpOneSpec(id)
		if err != nil {
			raw.Close()
			return nil, err
		}
		conn := utls.UClient(raw, &utls.Config{
			ServerName:         host,
			NextProtos:         []string{"http/1.1"},
			InsecureSkipVerify: insecure,
		}, utls.HelloCustom)
		if err := conn.ApplyPreset(spec); err != nil {
			raw.Close()
			return nil, err
		}
		if err := conn.HandshakeContext(ctx); err != nil {
			raw.Close()
			return nil, err
		}
		return conn, nil
	}
	return dial, nil
}
