//go:build !linux

package shared

// ReadPasswd has no account database to read outside linux, so the map stays
// empty and callers print the numeric id, as they do for an unknown user.
func ReadPasswd() map[string]string { return map[string]string{} }
