//go:build nocookiejar && !nonet

package net

import "net/http"

// newCookieJar reports "no jar" where net/http/cookiejar is not available (for
// example in TinyGo). Cookie options then say so instead of failing the build.
func newCookieJar() http.CookieJar { return nil }
