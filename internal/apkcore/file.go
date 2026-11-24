package apkcore

import (
	"fmt"
	"strconv"
	"strings"
)

// ParsePackageFile reads a .apk that lies on disk ("apk add ./pkg.apk") into a
// Package. trusted reports whether the package signature verified against one of
// the keys; an unsigned or foreign-signed file is left to the caller, which
// requires --allow-untrusted, exactly as for an index.
func ParsePackageFile(data []byte, src string, trusted []*Key) (*Package, bool, error) {
	streams, err := splitStreams(data)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %v", src, err)
	}
	var pkginfo string
	var signature *stream
	var control *stream
	found := false
	for i := range streams {
		s := &streams[i]
		if i == 0 && s.isSignature() {
			signature = s
			continue
		}
		if body, ok := s.entries[".PKGINFO"]; ok {
			pkginfo = string(body)
			control = s
			found = true
		}
	}
	if !found || control == nil {
		return nil, false, fmt.Errorf("%s: no .PKGINFO inside the file, not an apk package", src)
	}
	ok := false
	if signature != nil && len(trusted) > 0 {
		verified, _, err := verifySignature(signature.entries, control.raw, trusted)
		if err != nil {
			return nil, false, fmt.Errorf("%s: %v", src, err)
		}
		ok = len(verified) > 0
	}
	p := &Package{
		Name:        field(pkginfo, "pkgname"),
		Version:     field(pkginfo, "pkgver"),
		Arch:        field(pkginfo, "arch"),
		Description: field(pkginfo, "pkgdesc"),
		URL:         field(pkginfo, "url"),
		License:     field(pkginfo, "license"),
		Origin:      field(pkginfo, "origin"),
		Maintainer:  field(pkginfo, "maintainer"),
		Commit:      field(pkginfo, "commit"),
		RepoName:    src,
	}
	if p.Name == "" || p.Version == "" {
		return nil, false, fmt.Errorf("%s: .PKGINFO has no pkgname/pkginfo version", src)
	}
	if v := field(pkginfo, "size"); v != "" {
		p.Size, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := field(pkginfo, "builddate"); v != "" {
		p.BuildTime, _ = strconv.ParseInt(v, 10, 64)
	}
	for _, line := range strings.Split(pkginfo, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "depend = "):
			p.Depends = append(p.Depends, strings.TrimSpace(strings.TrimPrefix(line, "depend = ")))
		case strings.HasPrefix(line, "provides = "):
			p.Provides = append(p.Provides, strings.TrimSpace(strings.TrimPrefix(line, "provides = ")))
		case strings.HasPrefix(line, "replaces = "):
			p.Replaces = append(p.Replaces, strings.TrimSpace(strings.TrimPrefix(line, "replaces = ")))
		}
	}
	return p, ok, nil
}
