package rgcmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Archives, the way ugrep reads them: one archive file is not one stream but
// a tree of members, each searched on its own and named as
// archive.zip{member}. The labels, the stage counter and the limits were
// measured against ugrep 7.8.5 on 2026-11-24; docs/design/rg-archive.md keeps
// the reasoning and docs/measurements.md the runs.
//
// Stage counting, as measured: a container named on the command line is always
// opened (a.txt.gz is searched as its content, t.tar.gz gives its tar members).
// What those members are costs a stage: with --zmax=1 a zip inside a zip is
// searched as packed bytes, with --zmax=2 it is opened. A gzip inside a tar
// behaves the same way.
const (
	maxMemberBytes  = 64 << 20  // one member, decompressed
	maxArchiveBytes = 512 << 20 // all members of one archive together
	maxMembers      = 100000    // members of one archive
)

type containerKind int

const (
	kindNone containerKind = iota
	kindGzip
	kindBzip2
	kindZip
	kindTar
)

// containerKindOf decides by the magic bytes first: an archive keeps its
// format even when the name lies. The name only helps for a tar without the
// ustar marker, which old writers emit.
func containerKindOf(data []byte, name string) containerKind {
	switch {
	case len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b:
		return kindGzip
	case bytes.HasPrefix(data, []byte("BZh")):
		return kindBzip2
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		return kindZip
	case len(data) >= 262 && string(data[257:262]) == "ustar":
		return kindTar
	}
	if strings.EqualFold(filepath.Ext(name), ".tar") {
		return kindTar
	}
	return kindNone
}

// member is one searchable stream: the label that is printed, the file name
// the preprocessor decides on, and the bytes.
type member struct {
	label string
	name  string
	data  []byte
}

// expander walks one path and hands every searchable stream to visit. Soft
// problems (a link, an encrypted member, a limit) go to warn and the walk goes
// on; only an unreadable archive is an error, because that is what rg reports
// as a read error.
type expander struct {
	zmax  int
	warn  func(string)
	visit func(m member)

	seenBytes   int64
	seenMembers int
}

// readArchiveFile reads a container from disk, bounded by the archive limit.
func (e *expander) readFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxArchiveBytes {
		e.warn(fmt.Sprintf("%s: archive larger than %d bytes, not searched", path, int64(maxArchiveBytes)))
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(f, maxArchiveBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxArchiveBytes {
		e.warn(fmt.Sprintf("%s: archive larger than %d bytes, not searched", path, int64(maxArchiveBytes)))
		return nil, nil
	}
	return data, nil
}

// expand searches path as a container. handled is false when path is not one
// or when it is only compressed: then plain carries the decompressed bytes and
// the caller searches them like a normal file (a.txt.gz prints bare lines, as
// ugrep does), keeping BOM, --pre and the binary rules.
func (e *expander) expand(path string) (handled bool, plain []byte, err error) {
	head, err := readHead(path, 512)
	if err != nil {
		return false, nil, err
	}
	kind := containerKindOf(head, path)
	if kind == kindNone {
		return false, nil, nil
	}
	data, err := e.readFile(path)
	if err != nil {
		return true, nil, err
	}
	if data == nil {
		return true, nil, nil
	}
	if kind == kindTar || kind == kindZip {
		return true, nil, e.container(data, path, "", 1)
	}
	inner, err := inflate(kind, data)
	if err != nil {
		return true, nil, err
	}
	if len(inner) > maxArchiveBytes {
		e.warn(fmt.Sprintf("%s: more than %d bytes after decompression, not searched", path, int64(maxArchiveBytes)))
		return true, nil, nil
	}
	name := filepath.Base(strings.TrimSuffix(path, filepath.Ext(path)))
	if k := containerKindOf(inner, name); k == kindTar || k == kindZip {
		return true, nil, e.container(inner, path, "", 1)
	}
	return false, inner, nil
}

// readHead reads at most n bytes from the start of path.
func readHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	read, err := io.ReadFull(f, buf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return buf[:read], nil
	}
	if err != nil {
		return nil, err
	}
	return buf[:read], nil
}

// decompress unwraps gzip or bzip2 and either walks the tar or zip inside or
// visits the plain content. label stays the outer name; inner is the accumulated
// name inside the braces.
func (e *expander) decompress(data []byte, outer, label, innerName string, level int) error {
	plain, err := inflate(containerKindOf(data, outer), data)
	if err != nil {
		return err
	}
	if len(plain) > maxArchiveBytes {
		e.warn(fmt.Sprintf("%s: more than %d bytes after decompression, not searched", label, int64(maxArchiveBytes)))
		return nil
	}
	switch containerKindOf(plain, innerName) {
	case kindTar, kindZip:
		return e.container(plain, outer, innerOf(label, outer), level)
	}
	e.visit(member{label: label, name: innerName, data: plain})
	return nil
}

// inflate decompresses gzip or bzip2 bytes.
func inflate(kind containerKind, data []byte) ([]byte, error) {
	var r io.Reader
	switch kind {
	case kindGzip:
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		r = zr
	case kindBzip2:
		r = bzip2.NewReader(bytes.NewReader(data))
	default:
		return nil, fmt.Errorf("not a compressed container")
	}
	return io.ReadAll(io.LimitReader(r, maxArchiveBytes+1))
}

// innerOf is the part of label inside the braces, so a nested label keeps
// building on it: a.zip{b.tar} becomes b.tar and its members a.zip{b.tar{c}}.
func innerOf(label, outer string) string {
	if label == outer {
		return ""
	}
	_, after, ok := strings.Cut(label, "{")
	if !ok {
		return ""
	}
	return strings.TrimSuffix(after, "}")
}

// container walks tar or zip bytes. level is the stage of the members: the
// arguments of a top-level archive are stage 1.
func (e *expander) container(data []byte, outer, inner string, level int) error {
	switch containerKindOf(data, outer) {
	case kindZip:
		return e.zipMembers(data, outer, inner, level)
	case kindTar:
		return e.tarMembers(data, outer, inner, level)
	}
	return nil
}

// label names one member: outer{path}, nested outer{a:b}.
func archiveLabel(outer, inner string) string {
	if inner == "" {
		return outer
	}
	return outer + "{" + inner + "}"
}

func (e *expander) zipMembers(data []byte, outer, inner string, level int) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		name := f.Name
		if strings.HasSuffix(name, "/") {
			continue
		}
		if !e.takeMember(fmt.Sprintf("%s: too many members, rest skipped", outer)) {
			return nil
		}
		label := archiveLabel(outer, joinInner(inner, name))
		if f.Flags&0x1 != 0 {
			e.warn(fmt.Sprintf("%s: encrypted member, not searched", label))
			continue
		}
		if f.Mode()&(os.ModeSymlink|os.ModeDevice|os.ModeNamedPipe|os.ModeSocket) != 0 {
			e.warn(fmt.Sprintf("%s: link or special file, not searched", label))
			continue
		}
		if f.UncompressedSize64 > maxMemberBytes {
			e.warn(fmt.Sprintf("%s: member larger than %d bytes, not searched", label, int64(maxMemberBytes)))
			continue
		}
		rc, err := f.Open()
		if err != nil {
			e.warn(fmt.Sprintf("%s: %v", label, err))
			continue
		}
		buf, err := io.ReadAll(io.LimitReader(rc, maxMemberBytes+1))
		rc.Close()
		if err != nil {
			e.warn(fmt.Sprintf("%s: %v", label, err))
			continue
		}
		if len(buf) > maxMemberBytes {
			e.warn(fmt.Sprintf("%s: member larger than %d bytes, not searched", label, int64(maxMemberBytes)))
			continue
		}
		e.dispatch(buf, name, label, outer, joinInner(inner, name), level)
	}
	return nil
}

func (e *expander) tarMembers(data []byte, outer, inner string, level int) error {
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeSymlink, tar.TypeLink:
			e.warn(fmt.Sprintf("%s: link, not searched", archiveLabel(outer, joinInner(inner, hdr.Name))))
			continue
		case tar.TypeReg, tar.TypeRegA:
		default:
			continue
		}
		if !e.takeMember(fmt.Sprintf("%s: too many members, rest skipped", outer)) {
			return nil
		}
		label := archiveLabel(outer, joinInner(inner, hdr.Name))
		if hdr.Size > maxMemberBytes {
			e.warn(fmt.Sprintf("%s: member larger than %d bytes, not searched", label, int64(maxMemberBytes)))
			continue
		}
		buf, err := io.ReadAll(io.LimitReader(tr, maxMemberBytes+1))
		if err != nil {
			e.warn(fmt.Sprintf("%s: %v", label, err))
			continue
		}
		if len(buf) > maxMemberBytes {
			e.warn(fmt.Sprintf("%s: member larger than %d bytes, not searched", label, int64(maxMemberBytes)))
			continue
		}
		e.dispatch(buf, hdr.Name, label, outer, joinInner(inner, hdr.Name), level)
	}
}

// joinInner appends one member name to the path inside the braces, always with
// a forward slash, so a label looks the same on Windows.
func joinInner(inner, name string) string {
	name = filepath.ToSlash(strings.TrimPrefix(name, "/"))
	if inner == "" {
		return name
	}
	return inner + ":" + name
}

// takeMember counts one member against the three limits.
func (e *expander) takeMember(tooMany string) bool {
	if e.seenMembers >= maxMembers {
		e.warn(tooMany)
		return false
	}
	e.seenMembers++
	return true
}

// dispatch decides whether a member is opened as another container or searched
// as it is. Opening costs a stage, so at --zmax=1 a packed member is searched
// as packed bytes - a random hit, but an honest one, and ugrep does the same.
func (e *expander) dispatch(buf []byte, name, label, outer, inner string, level int) {
	e.seenBytes += int64(len(buf))
	if e.seenBytes > maxArchiveBytes {
		e.warn(fmt.Sprintf("%s: archive larger than %d bytes after expansion, rest skipped", outer, int64(maxArchiveBytes)))
		return
	}
	kind := containerKindOf(buf, name)
	if kind == kindNone {
		e.visit(member{label: label, name: filepath.Base(name), data: buf})
		return
	}
	if level >= e.zmax {
		e.visit(member{label: label, name: filepath.Base(name), data: buf})
		return
	}
	if kind == kindTar || kind == kindZip {
		if err := e.container(buf, outer, inner, level+1); err != nil {
			e.warn(fmt.Sprintf("%s: %v", label, err))
		}
		return
	}
	// gzip or bzip2 member: the label keeps the member name, the content is
	// what gets searched (measured against ugrep: {c.txt.gz}:hello).
	if err := e.decompress(buf, outer, label, filepath.Base(strings.TrimSuffix(name, filepath.Ext(name))), level+1); err != nil {
		e.warn(fmt.Sprintf("%s: %v", label, err))
	}
}
