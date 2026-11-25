//go:build nonet

package toolnet

// Commands gains nothing here with the nonet tag: the socket wrappers and with
// them internal/net are compiled out. The certificate commands stay in table.go.
