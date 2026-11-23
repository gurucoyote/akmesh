//go:build !windows

// Package blkid names the filesystem on a block device the way BusyBox does:
// path, UUID and TYPE, read from the superblock.
//
// Why our own command: u-root's blkid prints the device paths with UUID and TYPE
// only when pkg/mount/block happened to learn them, which on a plain device is
// never, so the line was a bare path (acmesh-u38). BusyBox prints
//
//	/dev/sda1: UUID="8f3a..." TYPE="ext4"
//
// which is what fstab writers and agents parse. The probe here reads the magic
// of the common filesystems and their UUIDs; an unknown filesystem still prints
// the path, as BusyBox does.
package blkid

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// probe is what one device told us. Empty fields stay out of the line.
type probe struct {
	UUID string
	Type string
}

// Run is the command. With no argument it lists the block devices; with paths it
// probes exactly those, like BusyBox's "blkid /dev/sda1".
func Run(name string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(stdout, "usage: blkid [DEVICE...]")
		fmt.Fprintln(stdout, "prints DEVICE: UUID=\"...\" TYPE=\"...\" from the superblock")
		fmt.Fprintln(stdout, "without arguments every block device from /sys/class/block")
		return 0
	}
	paths := args
	if len(paths) == 0 {
		var err error
		paths, err = blockDevices()
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return 1
		}
	}
	status := 0
	for _, path := range paths {
		p, err := Probe(path)
		if err != nil {
			// BusyBox exits 2 when a named device cannot be read, and stays
			// quiet when a listed one has no filesystem.
			if len(args) > 0 {
				fmt.Fprintf(stderr, "%s: %s: %v\n", name, path, err)
				status = 2
				continue
			}
			continue
		}
		fmt.Fprintln(stdout, line(path, p))
	}
	return status
}

// line is the BusyBox format: the path, then the fields it found.
func line(path string, p probe) string {
	var b strings.Builder
	b.WriteString(path)
	b.WriteString(":")
	if p.UUID != "" {
		fmt.Fprintf(&b, " UUID=%q", p.UUID)
	}
	if p.Type != "" {
		fmt.Fprintf(&b, " TYPE=%q", p.Type)
	}
	return b.String()
}

// blockDevices lists the device nodes of the block devices in /sys/class/block.
func blockDevices() ([]string, error) {
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		out = append(out, "/dev/"+e.Name())
	}
	sort.Strings(out)
	return out, nil
}

// Probe reads the superblock of path and says what filesystem it is.
func Probe(path string) (probe, error) {
	f, err := os.Open(path)
	if err != nil {
		return probe{}, err
	}
	defer f.Close()
	if p, ok := probeExt(f); ok {
		return p, nil
	}
	if p, ok := probeXFS(f); ok {
		return p, nil
	}
	if p, ok := probeFAT(f); ok {
		return p, nil
	}
	if p, ok := probeSwap(f); ok {
		return p, nil
	}
	return probe{}, fmt.Errorf("no filesystem found")
}

// readAt reads exactly len(buf) bytes at off.
func readAt(r io.ReaderAt, off int64, buf []byte) bool {
	n, err := r.ReadAt(buf, off)
	return err == nil && n == len(buf)
}

// probeExt reads the ext2/3/4 superblock, which sits at 1024 no matter the
// filesystem's block size.
func probeExt(r io.ReaderAt) (probe, bool) {
	const (
		sbOffset  = 1024
		magicOff  = 0x38 // s_magic within the superblock (0xEF53)
		uuidOff   = 0x68 // s_uuid within the superblock
		compatOff = 0x5C // s_feature_compat
		incomOff  = 0x60 // s_feature_incompat
	)
	buf := make([]byte, 0x100)
	if !readAt(r, sbOffset, buf) {
		return probe{}, false
	}
	if binary.LittleEndian.Uint16(buf[magicOff:]) != 0xEF53 {
		return probe{}, false
	}
	compat := binary.LittleEndian.Uint32(buf[compatOff:])
	incompat := binary.LittleEndian.Uint32(buf[incomOff:])
	kind := "ext2"
	switch {
	case incompat&0x40 != 0 || incompat&0x80 != 0 || incompat&0x200 != 0:
		// EXTENTS, 64BIT or FLEX_BG only ext4 has.
		kind = "ext4"
	case compat&0x4 != 0:
		// HAS_JOURNAL: ext3.
		kind = "ext3"
	}
	return probe{UUID: uuidFrom(buf[uuidOff : uuidOff+16]), Type: kind}, true
}

// probeXFS reads the XFS superblock at offset 0.
func probeXFS(r io.ReaderAt) (probe, bool) {
	buf := make([]byte, 128)
	if !readAt(r, 0, buf) {
		return probe{}, false
	}
	if string(buf[:4]) != "XFSB" {
		return probe{}, false
	}
	// sb_uuid sits at offset 32, big endian layout but a plain byte array.
	return probe{UUID: uuidFrom(buf[32:48]), Type: "xfs"}, true
}

// probeFAT reads the MSWIN4.1 boot sector and its volume serial.
func probeFAT(r io.ReaderAt) (probe, bool) {
	buf := make([]byte, 512)
	if !readAt(r, 0, buf) {
		return probe{}, false
	}
	if buf[510] != 0x55 || buf[511] != 0xAA {
		return probe{}, false
	}
	// FAT12/16 spell it at 54, FAT32 at 82; BusyBox calls all three vfat.
	isFAT := string(buf[54:57]) == "FAT" || string(buf[82:85]) == "FAT"
	if !isFAT {
		return probe{}, false
	}
	serial := binary.LittleEndian.Uint32(buf[67:71])
	// BusyBox prints the serial as 1234-ABCD, upper case.
	uuid := fmt.Sprintf("%04X-%04X", serial>>16, serial&0xFFFF)
	return probe{UUID: uuid, Type: "vfat"}, true
}

// probeSwap looks for the swap signature in the last ten bytes of the first
// page and reads the UUID next to the header.
func probeSwap(r io.ReaderAt) (probe, bool) {
	page := make([]byte, 4096)
	if !readAt(r, 0, page) {
		return probe{}, false
	}
	if string(page[4086:4096]) != "SWAPSPACE2" && string(page[4086:4096]) != "SWAP-SPACE" {
		return probe{}, false
	}
	// sws_uuid at offset 1036 of the swap header.
	return probe{UUID: uuidFrom(page[1036:1052]), Type: "swap"}, true
}

// uuidFrom formats 16 bytes as the 8-4-4-4-12 string BusyBox prints.
func uuidFrom(b []byte) string {
	if len(b) < 16 {
		return ""
	}
	for _, c := range b[:16] {
		if c != 0 {
			h := hex.EncodeToString(b[:16])
			return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
		}
	}
	return ""
}
