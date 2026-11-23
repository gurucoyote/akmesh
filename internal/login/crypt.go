//go:build !windows

package login

import (
	"crypto/subtle"
	"errors"
	"strings"
)

// ErrUnsupportedHash is returned for a hash this build cannot check. It is a
// separate answer from "wrong password": the second one is the user's problem,
// the first one is ours.
var ErrUnsupportedHash = errors.New("unsupported password hash scheme")

// verifyPassword reports whether password matches the hash from /etc/shadow.
//
// Supported are $6$ (sha512-crypt) and $5$ (sha256-crypt), the two formats a
// Linux system writes today. Everything else is named as unsupported: bcrypt
// ($2$), yescrypt ($y$, the Debian default), md5-crypt ($1$) and the old DES
// hashes without a prefix.
func verifyPassword(hash, password string) (bool, error) {
	if !strings.HasPrefix(hash, "$6$") && !strings.HasPrefix(hash, "$5$") {
		return false, ErrUnsupportedHash
	}
	got, err := shaCrypt(password, hash)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(hash)) == 1, nil
}
