package login

import (
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"hash"
	"strconv"
	"strings"
)

// shaCrypt implements the SHA-crypt scheme of Ulrich Drepper (the $5$ and $6$
// hashes in /etc/shadow). It is written here rather than taken from a library
// because the pure-Go library that exists writes a wrong hash for a setting
// with "rounds=" (it emits a double "$" and a different digest), and rounds are
// legal in /etc/shadow. The test compares against openssl 3.5.5 and perl's
// crypt, so this code is checked against the system, not against itself.
const cryptAlphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// shaCrypt computes the hash for password using the setting (for example
// "$6$saltstring" or "$6$rounds=10000$salt$").
func shaCrypt(password, setting string) (string, error) {
	is512 := strings.HasPrefix(setting, "$6$")
	if !is512 && !strings.HasPrefix(setting, "$5$") {
		return "", errors.New("not a SHA-crypt setting")
	}
	rest := setting[3:]
	rounds := 5000
	explicitRounds := false
	if strings.HasPrefix(rest, "rounds=") {
		n := rest[len("rounds="):]
		before, after, ok := strings.Cut(n, "$")
		if !ok {
			return "", errors.New("bad rounds field")
		}
		v, err := strconv.Atoi(before)
		if err != nil {
			return "", errors.New("bad rounds field")
		}
		rounds = clampRounds(v)
		explicitRounds = true
		rest = after
	}
	if i := strings.IndexByte(rest, '$'); i >= 0 {
		rest = rest[:i]
	}
	salt := rest
	if len(salt) > 16 {
		salt = salt[:16]
	}
	if salt == "" {
		return "", errors.New("empty salt")
	}
	digest := shaCryptDigest([]byte(password), []byte(salt), rounds, is512)

	var b strings.Builder
	if is512 {
		b.WriteString("$6$")
	} else {
		b.WriteString("$5$")
	}
	if explicitRounds {
		b.WriteString("rounds=")
		b.WriteString(strconv.Itoa(rounds))
		b.WriteString("$")
	}
	b.WriteString(salt)
	b.WriteString("$")
	b.WriteString(cryptBase64(digest, is512))
	return b.String(), nil
}

func clampRounds(r int) int {
	if r < 1000 {
		return 1000
	}
	if r > 999999999 {
		return 999999999
	}
	return r
}

// shaCryptDigest is steps 1 to 5 of the reference implementation.
func shaCryptDigest(p, s []byte, rounds int, is512 bool) []byte {
	newHash := func() hash.Hash {
		if is512 {
			return sha512.New()
		}
		return sha256.New()
	}
	hs := func(parts ...[]byte) []byte {
		h := newHash()
		for _, part := range parts {
			h.Write(part)
		}
		return h.Sum(nil)
	}
	plen := len(p)

	// 1. B = H(P + S + P)
	b := hs(p, s, p)

	// 2. A = H(P + S + B repeated until the length of P)
	repeat := func(src []byte, n int) []byte {
		if len(src) == 0 {
			return nil
		}
		out := make([]byte, 0, n)
		for len(out) < n {
			out = append(out, src...)
		}
		return out[:n]
	}
	// The reference keeps one context: P + S + B, then for every bit of len(P)
	// either the whole B or the key, and hashes once at the end. Hashing after
	// every bit would be a different scheme.
	feed := append(append([]byte{}, p...), s...)
	feed = append(feed, repeat(b, plen)...)
	for i := plen; i > 0; i >>= 1 {
		if i&1 == 1 {
			feed = append(feed, b...)
		} else {
			feed = append(feed, p...)
		}
	}
	a := hs(feed)

	// The sequence is written out that many times, not cut to that many bytes:
	// step 3 hashes P len(P) *times*, step 4 hashes S 16+A[0] *times*.
	times := func(src []byte, n int) []byte {
		out := make([]byte, 0, len(src)*n)
		for range n {
			out = append(out, src...)
		}
		return out
	}

	// 3. P' = H(P written len(P) times), repeated to the length of P
	pBytes := repeat(hs(times(p, plen)), plen)

	// 4. S' = H(S written 16+A[0] times), repeated to the length of S
	sBytes := repeat(hs(times(s, 16+int(a[0]))), len(s))

	// 5. the rounds loop
	for i := range rounds {
		var parts [][]byte
		if i&1 == 1 {
			parts = append(parts, pBytes)
		} else {
			parts = append(parts, a)
		}
		if i%3 != 0 {
			parts = append(parts, sBytes)
		}
		if i%7 != 0 {
			parts = append(parts, pBytes)
		}
		if i&1 == 1 {
			parts = append(parts, a)
		} else {
			parts = append(parts, pBytes)
		}
		a = hs(parts...)
	}
	return a
}

// cryptBase64 encodes with the byte order of the reference implementation: the
// resulting string is not a plain base64 of the digest.
func cryptBase64(d []byte, is512 bool) string {
	type triple struct {
		b2, b1, b0, n int
	}
	var order []triple
	if is512 {
		order = []triple{
			{0, 21, 42, 4}, {22, 43, 1, 4}, {44, 2, 23, 4}, {3, 24, 45, 4},
			{25, 46, 4, 4}, {47, 5, 26, 4}, {6, 27, 48, 4}, {28, 49, 7, 4},
			{50, 8, 29, 4}, {9, 30, 51, 4}, {31, 52, 10, 4}, {53, 11, 32, 4},
			{12, 33, 54, 4}, {34, 55, 13, 4}, {56, 14, 35, 4}, {15, 36, 57, 4},
			{37, 58, 16, 4}, {59, 17, 38, 4}, {18, 39, 60, 4}, {40, 61, 19, 4},
			{62, 20, 41, 4}, {-1, -1, 63, 2},
		}
	} else {
		order = []triple{
			{0, 10, 20, 4}, {21, 1, 11, 4}, {12, 22, 2, 4}, {3, 13, 23, 4},
			{24, 4, 14, 4}, {15, 25, 5, 4}, {6, 16, 26, 4}, {27, 7, 17, 4},
			{18, 28, 8, 4}, {9, 19, 29, 4}, {-1, 31, 30, 3},
		}
	}
	var sb strings.Builder
	// An index of -1 means a zero byte: the reference passes literal 0 in the
	// last group, not the byte at position 0.
	at := func(i int) int {
		if i < 0 {
			return 0
		}
		return int(d[i])
	}
	for _, t := range order {
		w := at(t.b2)<<16 | at(t.b1)<<8 | at(t.b0)
		for i := 0; i < t.n; i++ {
			sb.WriteByte(cryptAlphabet[w&0x3f])
			w >>= 6
		}
	}
	return sb.String()
}
