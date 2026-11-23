//go:build !windows

package blkid

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeImage lays out a file of the given size and lets the caller fill in the
// bytes of a superblock. Real images are not needed to prove the probe: the
// offsets are what BusyBox reads too.
func writeImage(t *testing.T, size int, fill func([]byte)) string {
	t.Helper()
	buf := make([]byte, size)
	fill(buf)
	path := filepath.Join(t.TempDir(), "image")
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// uuidBytes is the 16 byte uuid used in every case below.
func uuidBytes() []byte {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(0x10 + i)
	}
	return b
}

const wantUUID = "10111213-1415-1617-1819-1a1b1c1d1e1f"

func TestProbeExt4(t *testing.T) {
	path := writeImage(t, 4096, func(b []byte) {
		sb := b[1024:]
		binary.LittleEndian.PutUint16(sb[0x38:], 0xEF53)
		binary.LittleEndian.PutUint32(sb[0x60:], 0x40) // EXTENTS
		copy(sb[0x68:], uuidBytes())
	})
	p, err := Probe(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "ext4" || p.UUID != wantUUID {
		t.Fatalf("got %+v, want ext4 %s", p, wantUUID)
	}
}

func TestProbeExt3AndExt2(t *testing.T) {
	ext3 := writeImage(t, 4096, func(b []byte) {
		sb := b[1024:]
		binary.LittleEndian.PutUint16(sb[0x38:], 0xEF53)
		binary.LittleEndian.PutUint32(sb[0x5C:], 0x4) // HAS_JOURNAL
		copy(sb[0x68:], uuidBytes())
	})
	if p, err := Probe(ext3); err != nil || p.Type != "ext3" {
		t.Fatalf("ext3: %+v %v", p, err)
	}
	ext2 := writeImage(t, 4096, func(b []byte) {
		sb := b[1024:]
		binary.LittleEndian.PutUint16(sb[0x38:], 0xEF53)
		copy(sb[0x68:], uuidBytes())
	})
	if p, err := Probe(ext2); err != nil || p.Type != "ext2" {
		t.Fatalf("ext2: %+v %v", p, err)
	}
}

func TestProbeXFS(t *testing.T) {
	path := writeImage(t, 4096, func(b []byte) {
		copy(b[:4], "XFSB")
		copy(b[32:48], uuidBytes())
	})
	p, err := Probe(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "xfs" || p.UUID != wantUUID {
		t.Fatalf("got %+v, want xfs %s", p, wantUUID)
	}
}

func TestProbeVFAT(t *testing.T) {
	path := writeImage(t, 4096, func(b []byte) {
		copy(b[54:57], "FAT")
		copy(b[57:59], "16")
		b[510], b[511] = 0x55, 0xAA
		binary.LittleEndian.PutUint32(b[67:], 0x1234ABCD)
	})
	p, err := Probe(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "vfat" || p.UUID != "1234-ABCD" {
		t.Fatalf("got %+v, want vfat 1234-ABCD", p)
	}
}

func TestProbeSwap(t *testing.T) {
	path := writeImage(t, 4096, func(b []byte) {
		copy(b[4086:], "SWAPSPACE2")
		copy(b[1036:1052], uuidBytes())
	})
	p, err := Probe(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "swap" || p.UUID != wantUUID {
		t.Fatalf("got %+v, want swap %s", p, wantUUID)
	}
}

func TestProbeUnknownAndLine(t *testing.T) {
	path := writeImage(t, 4096, func(b []byte) {})
	if _, err := Probe(path); err == nil {
		t.Fatal("an empty image must not claim a filesystem")
	}
	if got := line("/dev/null", probe{}); got != "/dev/null:" {
		t.Fatalf("line without fields = %q, want /dev/null:", got)
	}
	if got := line("/dev/sda1", probe{UUID: "ab", Type: "ext4"}); got != `/dev/sda1: UUID="ab" TYPE="ext4"` {
		t.Fatalf("line = %q", got)
	}
}

// TestRunWithArguments prints the named device, or fails with 2 for one that
// cannot be read, the way BusyBox does.
func TestRunWithArguments(t *testing.T) {
	path := writeImage(t, 4096, func(b []byte) {
		sb := b[1024:]
		binary.LittleEndian.PutUint16(sb[0x38:], 0xEF53)
		copy(sb[0x68:], uuidBytes())
	})
	var out, errb bytes.Buffer
	if code := Run("blkid", []string{path}, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), `TYPE="ext2"`) || !strings.Contains(out.String(), wantUUID) {
		t.Fatalf("output %q", out.String())
	}
	out.Reset()
	errb.Reset()
	if code := Run("blkid", []string{filepath.Join(t.TempDir(), "missing")}, &out, &errb); code != 2 {
		t.Fatalf("missing device: exit %d, want 2", code)
	}
}
