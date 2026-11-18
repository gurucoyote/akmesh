//go:build !nonet && !nocookiejar

package net

import (
	"net/http"
	"net/http/cookiejar"
)

// newCookieJar returns the standard jar, the one curl -b and -c expect.
func newCookieJar() http.CookieJar {
	jar, _ := cookiejar.New(nil)
	return jar
}
