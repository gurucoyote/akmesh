//go:build windows

package apkscript

// mknodNull is not available on Windows; the target tree simply keeps no device.
func mknodNull(path string) error {
	return nil
}
