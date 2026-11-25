package webfonts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Google Fonts by family name, without a library.
//
// The CSS API is the smallest thing that works: GET
// fonts.googleapis.com/css2?family=<name>:ital,wght@0,400;0,700;1,400 answers
// with @font-face rules, and every rule names the file. Which format comes back
// depends only on the User-Agent: a modern browser gets woff2, which Word and
// most readers do not take, an old one gets plain TTF - the same bytes a
// designer downloads. So the request introduces itself as an old client and
// gets TTF (measured 2026-11-19: Open Sans v44, two files for 400 and 700).
//
// What arrives is not trusted: a font is only taken when it starts like a font,
// parses, and allows embedding. A 404 with an HTML page in it would otherwise be
// written into the document as if it were a font.

// googleUA is old on purpose: the CSS API answers a modern browser with woff2.
const googleUA = "Mozilla/5.0"

// Google is the client for the fetch. A nil client uses http.DefaultClient.
type Google struct {
	Client *http.Client
	// API overrides the CSS endpoint, for tests.
	API string
}

// fontFace is one @font-face rule from the CSS answer.
type fontFace struct {
	family string
	style  string // "normal", "italic"
	weight string // "400", "700"
	url    string
}

// Fetch asks Google Fonts for a family and returns the faces that can be
// embedded. An unknown family is an error, and so is an answer with no usable
// font: the caller falls back to the bundled set.
func (g Google) Fetch(ctx context.Context, family string) (Family, error) {
	if strings.TrimSpace(family) == "" {
		return Family{}, errors.New("webfonts: kein Familienname")
	}
	api := g.API
	if api == "" {
		api = "https://fonts.googleapis.com/css2"
	}
	u := api + "?family=" + url.PathEscape(family) +
		":ital,wght@0,400;0,700;1,400&display=swap"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Family{}, err
	}
	req.Header.Set("User-Agent", googleUA)
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Family{}, err
	}
	defer resp.Body.Close()
	// The status has to be checked: without it an error page would be parsed
	// as if it were a stylesheet.
	if resp.StatusCode != http.StatusOK {
		return Family{}, fmt.Errorf("webfonts: %s: %s", family, resp.Status)
	}
	css, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Family{}, err
	}
	faces := parseCSS(string(css))
	if len(faces) == 0 {
		return Family{}, fmt.Errorf("webfonts: %s: keine @font-face-Regel in der Antwort", family)
	}

	out := Family{Name: faces[0].family}
	for _, f := range faces {
		data, err := g.download(ctx, f.url)
		if err != nil {
			continue // one unusable face does not stop the others
		}
		if !valid(data) {
			continue
		}
		out.Faces = append(out.Faces, Face{
			Family: out.Name,
			Style:  styleOf(f.style, f.weight),
			TTF:    data,
		})
	}
	if len(out.Faces) == 0 {
		return Family{}, fmt.Errorf("webfonts: %s: keine brauchbare Schriftdatei", family)
	}
	return out, nil
}

// download reads one font file.
func (g Google) download(ctx context.Context, raw string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", googleUA)
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webfonts: %s: %s", raw, resp.Status)
	}
	// A font is not a library: 8 MB is already absurd.
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// styleOf maps the CSS declarations to one of the four faces a document knows.
func styleOf(style, weight string) Style {
	italic := strings.Contains(style, "italic")
	bold := weight == "700" || weight == "bold"
	switch {
	case italic && bold:
		return BoldItalic
	case italic:
		return Italic
	case bold:
		return Bold
	}
	return Regular
}

var fontFaceBlock = regexp.MustCompile(`(?s)@font-face\s*\{(.*?)\}`)

var (
	reFamily = regexp.MustCompile(`font-family:\s*['"]([^'"]+)['"]|font-family:\s*([^;}]+)`)
	reStyle  = regexp.MustCompile(`font-style:\s*([a-z]+)`)
	reWeight = regexp.MustCompile(`font-weight:\s*(\d+)`)
	reSrc    = regexp.MustCompile(`url\(\s*['"]?([^'")]+)['"]?\s*\)`)
)

// parseCSS reads the @font-face rules out of the answer. Only the latin block is
// taken: the CSS API answers one block per subset (cyrillic, greek, vietnamese,
// ...), and a document needs one face, not ten.
func parseCSS(css string) []fontFace {
	var out []fontFace
	seen := map[Style]bool{}
	for _, m := range fontFaceBlock.FindAllStringSubmatch(css, -1) {
		body := m[1]
		// A modern answer has one block per subset, each with a unicode-range;
		// the TTF answer has a single block without one, which is the whole
		// font and exactly what a document wants.
		if strings.Contains(body, "unicode-range") && !latinRange(body) {
			continue
		}
		f := fontFace{style: "normal", weight: "400"}
		if mm := reFamily.FindStringSubmatch(body); mm != nil {
			f.family = strings.TrimSpace(mm[1] + mm[2])
		}
		if mm := reStyle.FindStringSubmatch(body); mm != nil {
			f.style = mm[1]
		}
		if mm := reWeight.FindStringSubmatch(body); mm != nil {
			f.weight = mm[1]
		}
		if mm := reSrc.FindStringSubmatch(body); mm != nil {
			f.url = mm[1]
		}
		if f.family == "" || f.url == "" {
			continue
		}
		// One face per style: the last latin block wins, and a duplicate would
		// only add weight to the file.
		key := styleOf(f.style, f.weight)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}

// latinRange reports whether a @font-face block covers the basic latin range.
func latinRange(body string) bool {
	i := strings.Index(body, "unicode-range")
	if i < 0 {
		return true
	}
	return strings.Contains(body[i:], "U+0000")
}

// valid reports whether the bytes are a font a document may carry.
func valid(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	switch {
	case data[0] == 0 && data[1] == 1 && data[2] == 0 && data[3] == 0: // TrueType
	case string(data[:4]) == "true", string(data[:4]) == "OTTO": // sfnt, CFF
	case string(data[:4]) == "ttcf":
		return false // a collection holds several faces, a document wants one
	default:
		return false
	}
	// Embedding must be allowed; a font that forbids it is not taken, whoever
	// the licence text says.
	if fs, ok := FS(data); ok && fs&0x0002 != 0 {
		return false
	}
	return true
}

// Detect finds the family a page asks for: the first font-family declaration in
// its own stylesheet or style attribute, with generic families and the usual
// system names skipped. It is a heuristic, and the caller must live with a miss:
// without a family, the bundled set is used and the document still looks right.
func Detect(source []byte) string {
	s := string(source)
	generic := map[string]bool{
		"serif": true, "sans-serif": true, "monospace": true, "cursive": true,
		"fantasy": true, "system-ui": true, "inherit": true, "initial": true,
		"unset": true, "ui-monospace": true, "ui-serif": true, "ui-sans-serif": true,
		"arial": true, "helvetica": true, "times": true, "times new roman": true,
		"georgia": true, "verdana": true, "tahoma": true, "trebuchet ms": true,
		"courier": true, "courier new": true, "segoe ui": true, "roboto": false,
	}
	for _, m := range reFamily.FindAllStringSubmatch(s, -1) {
		name := strings.TrimSpace(m[1] + m[2])
		name = strings.TrimSuffix(name, "!important")
		name = strings.Trim(strings.TrimSpace(name), `'"`)
		if name == "" {
			continue
		}
		// A declaration can list several families; the first usable one wins.
		for part := range strings.SplitSeq(name, ",") {
			cand := strings.Trim(strings.TrimSpace(part), `'"`)
			if cand == "" || generic[strings.ToLower(cand)] {
				continue
			}
			// The name is taken whole: "Open Sans" stays two words, and a name
			// Google does not know simply fails to fetch and falls back.
			return cand
		}
	}
	return ""
}
