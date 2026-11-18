//go:build nonet

package tools

// netCommands stays empty with the nonet tag: the wrappers and with them
// internal/net are compiled out.
var netCommands = map[string]Func{}
