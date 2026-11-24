// Package apkcore reads and installs apk packages and their index without the
// C apk-tools and without the apko library: measured 24.11.2026, apko costs
// about 18.4 MB stripped, which doubles this binary (acmesh-xwcm).
//
// The format is the whole trick: a .apk and an APKINDEX.tar.gz are both
// concatenated gzip streams holding tar archives.
//
//	.apk           signature stream (optional), control stream, data stream
//	APKINDEX.tar.gz signature stream (optional), index stream
//
// Reading is therefore: walk the streams, look at the names inside, keep the
// payloads. The signature is an RSA block over the bytes AFTER the signature
// stream, named .SIGN.RSA.<keyname>.rsa.pub.
package apkcore

import (
	"archive/tar"
	"bytes"
	"compress/flate"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Package is one record of an APKINDEX.
type Package struct {
	Name         string
	Version      string
	Arch         string
	Size         int64
	Installed    int64
	Hash         string // Q1... (sha1 of the control+data stream)
	Description  string
	URL          string
	License      string
	Origin       string
	Maintainer   string
	BuildTime    int64
	Commit       string
	Depends      []string
	Provides     []string
	InstallIf    []string
	Replaces     []string
	DataHash     string
	ProviderPrio int
	RepoName     string // the repository it came from, for messages
}

// FullName is what apk prints: name-version.
func (p Package) FullName() string { return p.Name + "-" + p.Version }

// Index is a parsed APKINDEX plus where it came from.
type Index struct {
	Repo string
	// Tag is the channel label a repository carries in /etc/apk/repositories
	// ("@stable https://..."), empty for an untagged repository.
	Tag      string
	Packages []*Package
	Keys     []string // key names whose signature verified
	// Untrusted is true when the index carries a signature by a key we do not
	// hold: apk says UNTRUSTED and refuses. Without this, the presence of a
	// signature block was mistaken for a verified signature (found in the
	// battery, 24.11.2026).
	Untrusted bool
	Unsigned  bool
}

// ParseIndex reads the APKINDEX inside an APKINDEX.tar.gz. trusted is the set
// of RSA keys a signature may be checked against; when it is empty and the
// index is signed, the signature stays unverified and the caller decides.
func ParseIndex(data []byte, repo string, trusted []*Key) (*Index, error) {
	streams, err := splitStreams(data)
	if err != nil {
		return nil, err
	}
	idx := &Index{Repo: repo}
	var body []byte
	for i, s := range streams {
		if i == 0 && s.isSignature() {
			payload := data[len(s.raw):]
			verified, named, err := verifySignature(s.entries, payload, trusted)
			if err != nil {
				return nil, err
			}
			idx.Keys = verified
			if len(verified) == 0 && len(named) > 0 {
				idx.Untrusted = true
			}
			continue
		}
		for name, content := range s.entries {
			switch name {
			case "APKINDEX":
				body = content
			case "DESCRIPTION":
				// Not used; apk only prints it for `apk policy`.
			}
		}
	}
	if body == nil {
		return nil, fmt.Errorf("%s: no APKINDEX inside the archive", repo)
	}
	idx.Unsigned = len(idx.Keys) == 0 && !idx.Untrusted
	pkgs, err := parseIndexBody(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", repo, err)
	}
	for _, p := range pkgs {
		p.RepoName = repo
	}
	idx.Packages = pkgs
	return idx, nil
}

// parseIndexBody reads the colon-prefixed records apk uses: P:name, V:version,
// with a blank line between packages.
func parseIndexBody(body []byte) ([]*Package, error) {
	var out []*Package
	var cur *Package
	flush := func() {
		if cur != nil && cur.Name != "" {
			out = append(out, cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		if len(line) < 2 || line[1] != ':' {
			continue
		}
		if cur == nil {
			cur = &Package{}
		}
		val := line[2:]
		switch line[0] {
		case 'P':
			cur.Name = val
		case 'V':
			cur.Version = val
		case 'A':
			cur.Arch = val
		case 'S':
			cur.Size, _ = strconv.ParseInt(val, 10, 64)
		case 'I':
			cur.Installed, _ = strconv.ParseInt(val, 10, 64)
		case 'C':
			cur.Hash = val
		case 'T':
			cur.Description = val
		case 'U':
			cur.URL = val
		case 'L':
			cur.License = val
		case 'o':
			cur.Origin = val
		case 'm':
			cur.Maintainer = val
		case 't':
			cur.BuildTime, _ = strconv.ParseInt(val, 10, 64)
		case 'c':
			cur.Commit = val
		case 'D':
			cur.Depends = splitWords(val)
		case 'p':
			cur.Provides = splitWords(val)
		case 'i':
			cur.InstallIf = splitWords(val)
		case 'r':
			cur.Replaces = splitWords(val)
		case 'k':
			cur.ProviderPrio, _ = strconv.Atoi(val)
		case 'Z':
			cur.DataHash = val
		}
	}
	flush()
	return out, nil
}

func splitWords(s string) []string {
	return strings.Fields(s)
}

// ---- versions -------------------------------------------------------------

// CompareVersions orders versions the way apk does in the cases that matter
// for a repository of our own: digit runs compare numerically, everything else
// bytewise, and the release suffix (-r0) is its own part. It returns -1, 0, 1.
func CompareVersions(a, b string) int {
	va, ra := splitRelease(a)
	vb, rb := splitRelease(b)
	if c := compareParts(va, vb); c != 0 {
		return c
	}
	return compareParts(ra, rb)
}

func splitRelease(v string) (string, string) {
	if i := strings.LastIndex(v, "-r"); i >= 0 {
		if _, err := strconv.Atoi(v[i+2:]); err == nil {
			return v[:i], v[i+2:]
		}
	}
	return v, ""
}

func compareParts(a, b string) int {
	if a == b {
		return 0
	}
	ta := tokenize(a)
	tb := tokenize(b)
	for i := 0; i < len(ta) && i < len(tb); i++ {
		ca, cb := ta[i], tb[i]
		na, era := strconv.Atoi(ca)
		nb, erb := strconv.Atoi(cb)
		switch {
		case era == nil && erb == nil:
			if na != nb {
				if na < nb {
					return -1
				}
				return 1
			}
		case era == nil:
			return 1 // a number beats a letter, apk's rule
		case erb == nil:
			return -1
		default:
			if ca != cb {
				if ca < cb {
					return -1
				}
				return 1
			}
		}
	}
	// A trailing suffix decides: _alpha, _beta, _pre and _rc are older than the
	// release they hang on, _p is a patch and newer (apk's ordering).
	if len(ta) != len(tb) {
		longer, sign := ta, 1
		if len(ta) < len(tb) {
			longer, sign = tb, -1
		}
		extra := longer[min(len(ta), len(tb))]
		switch strings.ToLower(extra) {
		case "alpha", "beta", "pre", "rc", "cvs", "svn", "git", "hg":
			return -sign
		case "p":
			return sign
		}
		if len(ta) < len(tb) {
			return -1
		}
		return 1
	}
	return 0
}

func tokenize(v string) []string {
	var out []string
	var cur strings.Builder
	prevDigit := false
	for i, r := range v {
		digit := r >= '0' && r <= '9'
		alpha := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if i > 0 && (digit != prevDigit) && (digit || alpha) && (prevDigit || true) {
			if cur.Len() > 0 && (digit != prevDigit) {
				out = append(out, cur.String())
				cur.Reset()
			}
		}
		if r == '.' || r == '-' || r == '_' {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			prevDigit = false
			continue
		}
		cur.WriteRune(r)
		prevDigit = digit
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// ---- keys and signatures --------------------------------------------------

// Key is one RSA public key from /etc/apk/keys.
type Key struct {
	Name string // the file name, which is the name a signature refers to
	Pub  *rsa.PublicKey
}

// LoadKey parses a PEM public key the way apk keeps it in /etc/apk/keys: either
// "PUBLIC KEY" (PKIX) or "RSA PUBLIC KEY" (PKCS1).
func LoadKey(name string, data []byte) (*Key, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("%s: not a PEM key", name)
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rk, ok := pub.(*rsa.PublicKey); ok {
			return &Key{Name: name, Pub: rk}, nil
		}
		return nil, fmt.Errorf("%s: not an RSA key", name)
	}
	if rk, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return &Key{Name: name, Pub: rk}, nil
	}
	return nil, fmt.Errorf("%s: unreadable key", name)
}

// verifySignature checks the .SIGN.RSA.<keyname> block against the payload and
// returns the key names that matched. apk-tools signs with SHA1 and verifies
// SHA256 too, so both are tried; the block is the raw RSA signature, not base64.
func verifySignature(entries map[string][]byte, payload []byte, trusted []*Key) (verified []string, named []string, err error) {
	var names []string
	for name, sig := range entries {
		keyname, ok := strings.CutPrefix(name, ".SIGN.RSA.")
		if !ok {
			// Another block type (e.g. .SIGN.RSA256.) means a newer format we do
			// not claim to understand: say so instead of accepting it.
			return nil, nil, fmt.Errorf("unsupported signature block %s", name)
		}
		names = append(names, keyname)
		key := findKey(trusted, keyname)
		if key == nil {
			continue // unknown key: the caller decides, apk says UNTRUSTED
		}
		if err := checkRSA(key.Pub, payload, sig); err != nil {
			return nil, nil, fmt.Errorf("%s: %v", keyname, err)
		}
		return []string{keyname}, names, nil
	}
	// Signed by a key we do not have: the caller reports UNTRUSTED.
	return nil, names, nil
}

func findKey(keys []*Key, name string) *Key {
	for _, k := range keys {
		if k.Name == name {
			return k
		}
	}
	return nil
}

// checkRSA accepts the two digest forms apk uses (sha1, sha256).
func checkRSA(pub *rsa.PublicKey, payload, sig []byte) error {
	s1 := sha1.Sum(payload)
	if rsa.VerifyPKCS1v15(pub, crypto.SHA1, s1[:], sig) == nil {
		return nil
	}
	s256 := sha256.Sum256(payload)
	if rsa.VerifyPKCS1v15(pub, crypto.SHA256, s256[:], sig) == nil {
		return nil
	}
	return fmt.Errorf("bad signature")
}

// ---- streams --------------------------------------------------------------

type stream struct {
	raw     []byte            // the bytes of this gzip member, for hashing the rest
	entries map[string][]byte // file name to content, in archive order lost
	order   []string
}

func (s stream) isSignature() bool {
	for name := range s.entries {
		if strings.HasPrefix(name, ".SIGN.") {
			return true
		}
	}
	return false
}

// splitStreams walks the concatenated gzip members of a package or index.
//
// It reads the gzip header by hand and decompresses with flate instead of
// gzip.Reader: gzip.Reader wraps its input in a bufio and reads AHEAD, so
// "how many bytes did this member use" came out too large and the data hash
// pointed at the wrong bytes. A real abuild package showed it (24.11.2026);
// the unit fixtures with a single member did not.
func splitStreams(data []byte) ([]stream, error) {
	var out []stream
	pos := 0
	for pos < len(data) {
		hdrLen, err := gzipHeaderLen(data[pos:])
		if err != nil {
			if len(out) > 0 && len(data)-pos < 4 {
				break // trailing padding, apk tolerates it
			}
			return nil, err
		}
		body, compLen, err := inflateMember(data[pos+hdrLen:])
		if err != nil {
			return nil, err
		}
		total := hdrLen + compLen + 8 // 8 bytes gzip trailer (crc32, isize)
		if pos+total > len(data) {
			return nil, fmt.Errorf("truncated gzip member")
		}
		s := stream{}
		entries, order, err := readTar(body)
		if err != nil {
			return nil, err
		}
		s.entries = entries
		s.order = order
		s.raw = data[pos : pos+total]
		out = append(out, s)
		pos += total
	}
	return out, nil
}

// gzipHeaderLen returns the length of the header of the member at b[0:].
func gzipHeaderLen(b []byte) (int, error) {
	if len(b) < 10 || b[0] != 0x1f || b[1] != 0x8b || b[2] != 8 {
		return 0, fmt.Errorf("not a gzip member")
	}
	flags := b[3]
	n := 10
	if flags&0x04 != 0 { // FEXTRA
		if len(b) < n+2 {
			return 0, fmt.Errorf("truncated gzip header")
		}
		xlen := int(b[n]) | int(b[n+1])<<8
		n += 2 + xlen
	}
	for _, mask := range []byte{0x08, 0x10} { // FNAME, FCOMMENT
		if flags&mask != 0 {
			for n < len(b) && b[n] != 0 {
				n++
			}
			n++
		}
	}
	if flags&0x02 != 0 { // FHCRC
		n += 2
	}
	if n > len(b) {
		return 0, fmt.Errorf("truncated gzip header")
	}
	return n, nil
}

// inflateMember decompresses one deflate stream and reports how many
// compressed bytes it used. bytes.Reader implements flate.Reader, so flate
// reads without a buffer of its own and the count is exact.
func inflateMember(b []byte) ([]byte, int, error) {
	r := bytes.NewReader(b)
	fr := flate.NewReader(r)
	defer fr.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, fr); err != nil {
		return nil, 0, fmt.Errorf("deflate: %v", err)
	}
	return buf.Bytes(), len(b) - r.Len(), nil
}

// StreamInfo is one gzip member of a package or index, as apkbuild needs it.
type StreamInfo struct {
	Entries map[string][]byte
	Raw     []byte
}

// Split exposes the member boundaries: apkbuild hashes and signs exactly these
// bytes, so it must not guess them a second time.
func Split(data []byte) ([]StreamInfo, error) {
	streams, err := splitStreams(data)
	if err != nil {
		return nil, err
	}
	out := make([]StreamInfo, 0, len(streams))
	for _, s := range streams {
		out = append(out, StreamInfo{Entries: s.entries, Raw: s.raw})
	}
	return out, nil
}

// HeaderLen reports the gzip header length of the member at b[0:]. apkbuild
// writes the same format, so it uses this instead of a second copy.
func HeaderLen(b []byte) (int, error) { return gzipHeaderLen(b) }

// Inflate decompresses one deflate stream and reports the compressed bytes it
// used, without the 8 bytes of gzip trailer.
func Inflate(b []byte) ([]byte, int, error) { return inflateMember(b) }

// readTar returns the regular files of one tar stream plus their order.
func readTar(body []byte) (map[string][]byte, []string, error) {
	entries := map[string][]byte{}
	var order []string
	tr := tar.NewReader(bytes.NewReader(body))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("tar: %v", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, nil, err
		}
		entries[hdr.Name] = data
		order = append(order, hdr.Name)
	}
	return entries, order, nil
}

// SortedNames returns a stream's file names in a stable order, for messages.
func (s stream) SortedNames() []string {
	out := append([]string(nil), s.order...)
	sort.Strings(out)
	return out
}

// encodeBase64 helper kept for the signature test in the docs example.
func base64Block(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
