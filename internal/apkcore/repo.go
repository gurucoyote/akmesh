package apkcore

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client fetches indexes and packages. Local directories and http(s) URLs are
// both repositories; anything else is refused instead of guessed at.
type Client struct {
	Arch   string
	Keys   []*Key
	HTTP   *http.Client
	Cache  string   // directory for downloaded indexes, empty means no cache
	Repos  []string // repositories from -X or the repositories file
	Parser *Version
}

// Version matches apk's package name pattern, kept as a value so a caller can
// adjust it; the default accepts what Alpine ships.
type Version struct{}

// NewClient returns a client with a 30 second timeout.
func NewClient(arch string, keys []*Key, cache string) *Client {
	return &Client{
		Arch:  arch,
		Keys:  keys,
		Cache: cache,
		HTTP:  &http.Client{Timeout: 30 * time.Second},
	}
}

// NormalizeRepo trims a trailing slash and checks the scheme.
func NormalizeRepo(repo string) (string, error) {
	repo = strings.TrimSuffix(strings.TrimSpace(repo), "/")
	if repo == "" {
		return "", fmt.Errorf("empty repository")
	}
	if strings.Contains(repo, "://") && !strings.HasPrefix(repo, "http://") && !strings.HasPrefix(repo, "https://") {
		return "", fmt.Errorf("unsupported repository %q", repo)
	}
	return repo, nil
}

// Index fetches and parses one repository's APKINDEX for the client's arch.
func (c *Client) Index(repo string) (*Index, error) {
	repo, err := NormalizeRepo(repo)
	if err != nil {
		return nil, err
	}
	data, err := c.get(repo+"/"+c.Arch+"/APKINDEX.tar.gz", c.cacheName(repo, "index"))
	if err != nil {
		return nil, err
	}
	return ParseIndex(data, repo, c.Keys)
}

// PackageData fetches the .apk of one package and checks its Q1 hash.
func (c *Client) PackageData(p *Package) ([]byte, error) {
	url := p.RepoName + "/" + c.Arch + "/" + p.FullName() + ".apk"
	data, err := c.get(url, "")
	if err != nil {
		return nil, err
	}
	if err := VerifyPackage(data, p, c.Keys...); err != nil {
		return nil, err
	}
	return data, nil
}

// get reads a URL or a file. cachePath, when set, keeps a copy and is read
// first, the way /var/cache/apk works for indexes.
func (c *Client) get(url, cachePath string) ([]byte, error) {
	if cachePath != "" {
		if data, err := os.ReadFile(cachePath); err == nil {
			return data, nil
		}
	}
	var data []byte
	var err error
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		data, err = c.fetchHTTP(url)
	} else {
		data, err = os.ReadFile(url)
	}
	if err != nil {
		return nil, err
	}
	if cachePath != "" {
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err == nil {
			_ = os.WriteFile(cachePath, data, 0o644)
		}
	}
	return data, nil
}

func (c *Client) fetchHTTP(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "akmesh/apk")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// cacheName turns a repository URL into a stable cache file name.
func (c *Client) cacheName(repo, kind string) string {
	if c.Cache == "" {
		return ""
	}
	sum := sha1.Sum([]byte(repo))
	name := fmt.Sprintf("%s-%x.%s", kind, sum[:6], "tar.gz")
	return filepath.Join(c.Cache, name)
}
