// Package imgload reads the bytes an image reference points at, for the
// document writers in this repository.
//
// Two rules, both learned the hard way:
//   - what is embedded is decided by decoding the bytes, never by the file
//     extension: a .png that holds an SVG is not a picture either format wants;
//   - nothing is fetched from the network. A converter that silently reaches
//     out is a surprise in a tool that otherwise works offline, and the
//     caller's replacement text stays readable without the picture.
package imgload

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Config is what a writer needs to place a picture: its pixel size and format.
type Config struct {
	Width  int
	Height int
	Format string // "png", "jpeg" or "gif"
	Data   []byte
}

// Load reads an image reference: a path relative to baseDir, a file: URL, or an
// inline data: URL. It reports false for anything it cannot read or that is not
// one of the three formats the writers can carry.
func Load(raw, baseDir string) (Config, bool) {
	data, ok := read(raw, baseDir)
	if !ok {
		return Config{}, false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return Config{}, false
	}
	switch format {
	case "png", "jpeg", "gif":
	default:
		return Config{}, false
	}
	return Config{Width: cfg.Width, Height: cfg.Height, Format: format, Data: data}, true
}

// read turns the reference into bytes.
func read(raw, baseDir string) ([]byte, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, false
	}
	if strings.HasPrefix(s, "data:") {
		comma := strings.Index(s, ",")
		if comma < 0 {
			return nil, false
		}
		meta, payload := s[:comma], s[comma+1:]
		if strings.Contains(meta, ";base64") {
			data, err := base64.StdEncoding.DecodeString(payload)
			if err != nil || len(data) == 0 {
				return nil, false
			}
			return data, true
		}
		plain, err := url.PathUnescape(payload)
		if err != nil || plain == "" {
			return nil, false
		}
		return []byte(plain), true
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "//") {
		return nil, false
	}
	if strings.HasPrefix(s, "file:") {
		u, err := url.Parse(s)
		if err != nil {
			return nil, false
		}
		s = u.Path
	}
	// A URL can carry a query or a fragment; a file name cannot.
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	if !filepath.IsAbs(s) {
		s = filepath.Join(baseDir, s)
	}
	data, err := os.ReadFile(s)
	if err != nil {
		return nil, false
	}
	return data, true
}
