//go:build !windows && !darwin

// Local addition, no upstream source: installing a local CA into the system
// trust store (acmesh-816).

package mkcert

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// trustDir is where the distribution looks for extra roots. A variable so the
// test can point it somewhere harmless instead of writing into the system.
var trustDir = "/usr/local/share/ca-certificates"

// installCA copies the CA certificate into the trust store and refreshes it.
// Without root the copy fails, and that is reported as such: doing nothing
// quietly would leave a certificate nobody trusts while claiming success.
func installCA(base, certPath string) error {
	dir := trustDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %v (installing a root needs root)", dir, err)
	}
	dst := filepath.Join(dir, "akmesh-"+filepath.Base(base)+".crt")
	data, err := os.ReadFile(certPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %v (installing a root needs root)", dst, err)
	}
	if err := updateTrust(); err != nil {
		return err
	}
	fmt.Printf("installed %s as %s and refreshed the trust store\n", certPath, dst)
	return nil
}

// uninstallCA removes it again; a CA that is not there is not an error, only
// worth saying.
func uninstallCA(base string) error {
	dst := filepath.Join(trustDir, "akmesh-"+filepath.Base(base)+".crt")
	if err := os.Remove(dst); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("%s is not installed, nothing to remove\n", dst)
			return nil
		}
		return fmt.Errorf("cannot remove %s: %v (installing a root needs root)", dst, err)
	}
	if err := updateTrust(); err != nil {
		return err
	}
	fmt.Printf("removed %s and refreshed the trust store\n", dst)
	return nil
}

// updateTrust is a variable so a test can see it happen without touching the
// system's trust store.
var updateTrust = updateTrustSystem

// updateTrustSystem is the distribution's own step. Debian and Alpine differ, so
// both are tried and the first that exists wins.
func updateTrustSystem() error {
	for _, cmd := range []string{"update-ca-certificates", "update-ca-trust"} {
		path, err := exec.LookPath(cmd)
		if err != nil {
			continue
		}
		out, err := exec.Command(path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s failed: %v: %s", cmd, err, out)
		}
		return nil
	}
	return fmt.Errorf("neither update-ca-certificates nor update-ca-trust is on PATH")
}
