//go:build darwin

// Local addition, no upstream source: installing a local CA into the login
// keychain (acmesh-816).
//
// Not measured: this project is built and tested on Linux and Windows. The
// command follows what mkcert and Kiri do, but nobody here has run it.

package mkcert

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"akmesh/internal/appname"
)

func keychain() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library/Keychains/login.keychain-db")
}

func installCA(base, certPath string) error {
	kc := keychain()
	if kc == "" {
		return fmt.Errorf("cannot find the login keychain")
	}
	out, err := exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", kc, certPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("security add-trusted-cert failed: %v: %s", err, out)
	}
	fmt.Printf("installed %s into %s as a trusted root for websites\n", certPath, kc)
	return nil
}

func uninstallCA(base string) error {
	return fmt.Errorf("removing a root needs Keychain Access by hand; the certificate is named "+appname.Name+"-%s", base)
}
