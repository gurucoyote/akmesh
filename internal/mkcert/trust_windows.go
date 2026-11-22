//go:build windows

// Local addition, no upstream source: installing a local CA into the user's
// root store (acmesh-816).

package mkcert

import (
	"fmt"
	"os/exec"

	"akmesh/internal/appname"
)

// installCA adds the certificate to the current user's root store. That is what
// makes it trusted for that user's programs; a machine-wide store needs
// administrator rights and is not what a test CA should ask for.
func installCA(base, certPath string) error {
	return certutil("-addstore", "-user", "Root", certPath, fmt.Sprintf("installed %s into the user's root store", certPath))
}

func uninstallCA(base string) error {
	// certutil has no remove, so the message says what to do instead of
	// pretending the certificate is gone.
	return fmt.Errorf("removing a root needs certlm.msc or certmgr.msc by hand; the stored name is "+appname.Name+"-%s", base)
}

func certutil(args ...string) error {
	tool, err := exec.LookPath("certutil")
	if err != nil {
		return fmt.Errorf("certutil is not on PATH: %v", err)
	}
	msg := args[len(args)-1]
	out, err := exec.Command(tool, args[:len(args)-1]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("certutil failed: %v: %s", err, out)
	}
	fmt.Println(msg)
	return nil
}
