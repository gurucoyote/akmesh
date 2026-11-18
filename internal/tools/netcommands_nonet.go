//go:build nonet

package tools

// With the nonet tag the networking tools are absent, not merely disabled: the
// files that implement them are compiled out.
var netCommands = map[string]Func{}
