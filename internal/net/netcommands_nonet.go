//go:build nonet

package net

// With the nonet tag the networking tools are absent, not merely disabled: the
// files that implement them are compiled out.
var handlers = map[string]CmdFunc{}
