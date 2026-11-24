package apkcore

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// VerifyPackage checks a .apk against the index. The two hashes were measured
// on a real abuild package (24.11.2026, acmesh-xwcm):
//
//	C:Q1... in the index   sha1 over the RAW control gzip member, base64
//	datahash in .PKGINFO   sha256 over the RAW data gzip member, hex
//
// The whole file is NOT what apk hashes, which is why a naive sha1 of the file
// looked like a mismatch on a package that was perfectly fine.
func VerifyPackage(data []byte, p *Package) error {
	streams, err := splitStreams(data)
	if err != nil {
		return fmt.Errorf("%s: %v", p.FullName(), err)
	}
	var control, payload *stream
	var pkginfo string
	for i := range streams {
		s := &streams[i]
		if s.isSignature() {
			continue
		}
		if body, ok := s.entries[".PKGINFO"]; ok {
			control = s
			pkginfo = string(body)
			continue
		}
		payload = s
	}
	if control == nil {
		return fmt.Errorf("%s: no control segment", p.FullName())
	}
	if p.Hash != "" {
		sum := sha1.Sum(control.raw)
		got := "Q1" + base64.StdEncoding.EncodeToString(sum[:])
		if got != p.Hash {
			return fmt.Errorf("%s: checksum mismatch: index says %s, control segment is %s", p.FullName(), p.Hash, got)
		}
	}
	if payload != nil {
		if want := field(pkginfo, "datahash"); want != "" {
			sum := sha256.Sum256(payload.raw)
			got := fmt.Sprintf("%x", sum)
			if got != want {
				return fmt.Errorf("%s: data mismatch: package says %s, data segment is %s", p.FullName(), want, got)
			}
		}
	}
	return nil
}

// field reads one "key = value" line out of .PKGINFO.
func field(pkginfo, key string) string {
	for _, line := range strings.Split(pkginfo, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}
