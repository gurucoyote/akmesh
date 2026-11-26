//go:build windows

package sh

import "golang.org/x/sys/windows"

// elevated reports whether this process holds an elevated token, which is what
// "als Administrator ausfuehren" gives it. Windows has no root and no uid: the
// same account just carries a token with more rights, so a prompt that always
// showed "$" hid the one thing worth knowing (26.11.2026).
//
// The answer is asked of the kernel, not guessed from the environment.
func elevated() bool {
	token := windows.GetCurrentProcessToken()
	return token.IsElevated()
}
