//go:build !nonet

package net

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"

	"akmesh/internal/policy"
	"strconv"
	"strings"
	"time"
)

// Exit codes are curl's, because scripts check them.
const (
	exitOK         = 0
	exitResolve    = 6
	exitConnect    = 7
	exitHTTPError  = 22
	exitTimeout    = 28
	exitTLS        = 35
	exitTooManyRed = 47
)

// netOptions is the shared request description. curl and wget both fill it, so
// redirects, cookies, timeouts and error texts cannot drift apart.
type netOptions struct {
	Method    string
	URL       string
	Headers   []string
	Data      []string
	DataRaw   string
	DataFile  string
	Form      []string
	User      string
	UserAgent string
	Referer   string
	Head      bool
	Include   bool
	Silent    bool
	ShowError bool
	Fail      bool
	Follow    bool
	MaxRedirs int
	Output    string
	Remote    bool
	Insecure  bool
	Proxy     string
	MaxTime   time.Duration
	Connect   time.Duration
	Cookies   []string
	JarFile   string
	Range     string
	Upload    string
	AsGet     bool
	Verbose   bool
	Compress  bool
	Resume    bool
	WriteOut    string
	Front       string
	Impersonate string
}

// The default User-Agent names this tool and says what it behaves like. It does
// not claim to be curl or wget: the version in a fake string is a lie a server
// can check, and the measured 503 on amazon.de was triggered by the literal
// "Go-http-client", not by the absence of a browser name (acmesh-ijz).
const (
	agentCurl = "akmesh/0.1 (curl compatible)"
	agentWget = "akmesh/0.1 (wget compatible)"
)

func defaultUserAgent(front string) string {
	if front == "wget" {
		return agentWget
	}
	return agentCurl
}

// runNet performs the request and writes the result. The returned value is the
// process exit code.
func runNet(opts netOptions, stdin *os.File, stdout, stderr *os.File) int {
	if opts.URL == "" {
		fmt.Fprintln(stderr, "no URL given")
		return 2
	}
	if !strings.HasPrefix(opts.URL, "http://") && !strings.HasPrefix(opts.URL, "https://") {
		// Only http and https. file:// would be a way around the file policies
		// akmesh exists to enforce.
		fmt.Fprintf(stderr, "unsupported URL scheme in %q (only http and https)\n", opts.URL)
		return 3
	}
	// The policy decides which hosts may be reached, before a socket is opened.
	if u, err := url.Parse(opts.URL); err == nil {
		if err := policy.CheckHost(u.Host); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
	}
	method := opts.Method
	if method == "" {
		method = http.MethodGet
		if len(opts.Data) > 0 || opts.DataRaw != "" || opts.DataFile != "" || len(opts.Form) > 0 {
			method = http.MethodPost
		}
	}
	if opts.Head {
		method = http.MethodHead
	}
	if opts.Upload != "" {
		if method == http.MethodGet {
			method = http.MethodPut
		}
	}

	body, contentType, err := buildBody(opts)
	if err != nil {
		fmt.Fprintf(stderr, "body: %v\n", err)
		return 2
	}

	requestURL := opts.URL
	if opts.AsGet && len(body) > 0 {
		sep := "?"
		if strings.Contains(requestURL, "?") {
			sep = "&"
		}
		requestURL += sep + string(body)
		body = nil
		contentType = ""
	}

	client, err := buildClient(opts, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitConnect
	}

	req, err := http.NewRequest(method, requestURL, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if contentType != "" && !opts.AsGet {
		req.Header.Set("Content-Type", contentType)
	}
	for _, h := range opts.Headers {
		name, value, ok := strings.Cut(h, ":")
		if !ok {
			fmt.Fprintf(stderr, "bad header %q, want \"Name: value\"\n", h)
			return 2
		}
		req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	} else if agent := defaultUserAgent(opts.Front); agent != "" {
		// Without this Go announces "Go-http-client/2.0", and a measured
		// request to amazon.de was answered with 503 for exactly that string
		// while every other agent got the page (acmesh-ijz).
		req.Header.Set("User-Agent", agent)
	}
	if req.Header.Get("Accept") == "" {
		// curl and wget both send it; a missing Accept is its own signal.
		req.Header.Set("Accept", "*/*")
	}
	if opts.Referer != "" {
		req.Header.Set("Referer", opts.Referer)
	}
	if opts.User != "" {
		user, pass, _ := strings.Cut(opts.User, ":")
		req.SetBasicAuth(user, pass)
	}
	if opts.Range != "" {
		r := opts.Range
		if !strings.Contains(r, "-") {
			r = r + "-"
		}
		req.Header.Set("Range", "bytes="+r)
	}
	if opts.Resume && opts.Output != "" {
		if info, err := os.Stat(opts.Output); err == nil && info.Size() > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", info.Size()))
		}
	}
	if opts.Compress {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	if opts.Verbose {
		fmt.Fprintf(stderr, "> %s %s\n", method, requestURL)
		for name, values := range req.Header {
			for _, v := range values {
				fmt.Fprintf(stderr, "> %s: %s\n", name, v)
			}
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return reportNetError(err, opts, stderr, start)
	}
	defer resp.Body.Close()

	if opts.Verbose {
		fmt.Fprintf(stderr, "< %s\n", resp.Status)
		for name, values := range resp.Header {
			for _, v := range values {
				fmt.Fprintf(stderr, "< %s: %s\n", name, v)
			}
		}
	}
	if opts.Include {
		// -i must look like curl's own output: a status line the way curl
		// writes it, then the header lines without any prefix (acmesh-ly7).
		writeResponseHead(stdout, resp)
	}

	if opts.Fail && resp.StatusCode >= 400 {
		if !opts.Silent || opts.ShowError {
			fmt.Fprintf(stderr, "HTTP error %d for %s\n", resp.StatusCode, requestURL)
		}
		return exitHTTPError
	}

	target, closer, err := openOutput(opts, requestURL, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitConnect
	}
	if closer != nil {
		defer closer()
	}
	written, err := io.Copy(target, resp.Body)
	if err != nil {
		fmt.Fprintf(stderr, "transfer incomplete: %v\n", err)
		return exitConnect
	}
	if opts.Silent && !opts.Verbose {
		// -s only mutes progress, never the body; nothing to do here, kept for
		// clarity of intent.
	}
	if opts.Verbose {
		fmt.Fprintf(stderr, "< %d bytes in %s\n", written, time.Since(start).Round(time.Millisecond))
	}
	if opts.WriteOut != "" {
		// -w writes to stdout even when -o sent the body to a file, and -s does
		// not mute it (acmesh-ly7).
		vars := writeOutVars{
			code: resp.StatusCode,
			size: written,
			took: time.Since(start),
			url:  requestURL,
			kind: resp.Header.Get("Content-Type"),
		}
		if resp.Request != nil && resp.Request.URL != nil {
			vars.url = resp.Request.URL.String()
		}
		fmt.Fprint(stdout, expandWriteOut(opts.WriteOut, vars, stderr))
	}
	if opts.JarFile != "" {
		if err := writeCookieFile(opts.JarFile, client, opts.URL); err != nil {
			fmt.Fprintf(stderr, "cookie jar: %v\n", err)
		}
	}
	return exitOK
}

func buildBody(opts netOptions) ([]byte, string, error) {
	if len(opts.Form) > 0 {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		for _, item := range opts.Form {
			name, value, ok := strings.Cut(item, "=")
			if !ok {
				return nil, "", fmt.Errorf("bad form field %q, want name=value", item)
			}
			if strings.HasPrefix(value, "@") {
				path := strings.TrimPrefix(value, "@")
				f, err := os.Open(path)
				if err != nil {
					return nil, "", err
				}
				part, err := writer.CreateFormFile(name, path)
				if err != nil {
					f.Close()
					return nil, "", err
				}
				if _, err := io.Copy(part, f); err != nil {
					f.Close()
					return nil, "", err
				}
				f.Close()
				continue
			}
			writer.WriteField(name, value)
		}
		writer.Close()
		return buf.Bytes(), writer.FormDataContentType(), nil
	}
	if opts.DataFile != "" {
		data, err := os.ReadFile(opts.DataFile)
		if err != nil {
			return nil, "", err
		}
		return data, "application/octet-stream", nil
	}
	if opts.Upload != "" {
		data, err := os.ReadFile(opts.Upload)
		if err != nil {
			return nil, "", err
		}
		return data, "application/octet-stream", nil
	}
	if opts.DataRaw != "" {
		return []byte(opts.DataRaw), "application/x-www-form-urlencoded", nil
	}
	if len(opts.Data) > 0 {
		return []byte(strings.Join(opts.Data, "&")), "application/x-www-form-urlencoded", nil
	}
	return nil, "", nil
}

// buildClient assembles transport, TLS settings, proxy and cookie jar.
func buildClient(opts netOptions, stderr *os.File) (*http.Client, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	if opts.Proxy != "" {
		proxyURL, err := url.Parse(opts.Proxy)
		if err != nil {
			return nil, fmt.Errorf("bad proxy %q: %v", opts.Proxy, err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	if opts.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if opts.Impersonate != "" {
		// A proxy would have to carry the ClientHello, and that means speaking
		// CONNECT with a foreign TLS stack: refuse instead of silently ignoring
		// the flag (acmesh-agy).
		if opts.Proxy != "" {
			return nil, fmt.Errorf("--impersonate cannot be combined with --proxy")
		}
		dial, err := impersonateDialer(opts.Impersonate, opts.Insecure, opts.Connect)
		if err != nil {
			return nil, err
		}
		transport.DialTLSContext = dial
	}
	// The jar comes from a helper: TinyGo's standard library has no
	// net/http/cookiejar, so a build tag swaps in nil and cookie options then
	// report themselves as unsupported instead of breaking the build.
	jar := newCookieJar()
	client := &http.Client{Transport: transport, Jar: jar}
	if opts.MaxTime > 0 {
		client.Timeout = opts.MaxTime
	}
	if opts.Connect > 0 {
		transport.DialContext = (&net.Dialer{Timeout: opts.Connect}).DialContext
	}
	if opts.Follow {
		limit := opts.MaxRedirs
		if limit <= 0 {
			limit = 50
		}
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= limit {
				return errTooManyRedirects
			}
			return nil
		}
	} else {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	if len(opts.Cookies) > 0 {
		if jar == nil {
			fmt.Fprintln(stderr, "curl: cookies are not supported in this build")
		}
	}
	if len(opts.Cookies) > 0 && jar != nil {
		u, err := url.Parse(opts.URL)
		if err != nil {
			return nil, err
		}
		for _, c := range opts.Cookies {
			name, value, ok := strings.Cut(c, "=")
			if !ok {
				continue
			}
			jar.SetCookies(u, []*http.Cookie{{Name: strings.TrimSpace(name), Value: strings.TrimSpace(value)}})
		}
	}
	if opts.JarFile != "" && jar != nil {
		if cookies, err := readCookieFile(opts.JarFile); err == nil {
			u, err := url.Parse(opts.URL)
			if err == nil {
				jar.SetCookies(u, cookies)
			}
		}
	}
	return client, nil
}

var errTooManyRedirects = errors.New("too many redirects")

// reportNetError turns a transport error into curl's exit code and a message,
// which is what a script branches on.
func reportNetError(err error, opts netOptions, stderr *os.File, start time.Time) int {
	var dns *net.DNSError
	var netErr net.Error
	switch {
	case errors.As(err, &dns):
		if !opts.Silent || opts.ShowError {
			fmt.Fprintf(stderr, "could not resolve host: %s\n", dns.Name)
		}
		return exitResolve
	case errors.Is(err, errTooManyRedirects):
		if !opts.Silent || opts.ShowError {
			fmt.Fprintln(stderr, "too many redirects")
		}
		return exitTooManyRed
	case errors.As(err, &netErr) && netErr.Timeout():
		if !opts.Silent || opts.ShowError {
			fmt.Fprintf(stderr, "timeout after %s\n", time.Since(start).Round(time.Millisecond))
		}
		return exitTimeout
	}
	msg := err.Error()
	if strings.Contains(msg, "certificate") || strings.Contains(msg, "x509") || strings.Contains(msg, "tls") {
		if !opts.Silent || opts.ShowError {
			fmt.Fprintf(stderr, "TLS error: %v\n", err)
		}
		return exitTLS
	}
	if !opts.Silent || opts.ShowError {
		fmt.Fprintf(stderr, "connection failed: %v\n", err)
	}
	return exitConnect
}

// openOutput decides where the body goes: stdout, a named file, or the file name
// taken from the URL.
func openOutput(opts netOptions, requestURL string, stdout *os.File) (io.Writer, func(), error) {
	name := opts.Output
	if opts.Remote && name == "" {
		u, err := url.Parse(requestURL)
		if err == nil {
			base := u.Path
			if idx := strings.LastIndex(base, "/"); idx >= 0 {
				base = base[idx+1:]
			}
			if base != "" {
				name = base
			}
		}
	}
	if name == "" || name == "-" {
		return stdout, nil, nil
	}
	flags := os.O_CREATE | os.O_WRONLY
	if opts.Resume {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(name, flags, 0o644)
	if err != nil {
		return nil, nil, err
	}
	if opts.JarFile != "" {
		// written after the transfer in the caller's defer is too late for the
		// jar, so the caller re-reads it; nothing to do here.
	}
	return f, func() { f.Close() }, nil
}

// readCookieFile reads a Netscape style cookies.txt. Only the fields we write
// ourselves are used; anything else is ignored.
func readCookieFile(path string) ([]*http.Cookie, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cookies []*http.Cookie
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		cookies = append(cookies, &http.Cookie{
			Domain: fields[0],
			Path:   fields[2],
			Name:   fields[5],
			Value:  fields[6],
		})
	}
	return cookies, scanner.Err()
}

// writeCookieFile stores the jar in the same simple format.
func writeCookieFile(path string, client *http.Client, rawURL string) error {
	if client.Jar == nil {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	cookies := client.Jar.Cookies(u)
	if len(cookies) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	for _, c := range cookies {
		domain := c.Domain
		if domain == "" {
			domain = u.Hostname()
		}
		expiry := c.Expires.Unix()
		if expiry < 0 {
			expiry = 0
		}
		fmt.Fprintf(&b, "%s\tFALSE\t/\tFALSE\t%d\t%s\t%s\n", domain, expiry, c.Name, c.Value)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// parseSeconds accepts "10", "10s" and Go durations.
func parseSeconds(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("bad duration %q", s)
	}
	return time.Duration(f * float64(time.Second)), nil
}

// writeResponseHead prints what -i shows: a status line and the headers, with no
// prefix. curl writes "< HTTP/2 200" for -v and "HTTP/2 200" for -i; agents
// parse that line, so the shape matters (acmesh-ly7). Header names are printed
// lowercase as HTTP/2 sends them, sorted, because net/http hands them over in a
// map and curl's received order is not recoverable from it.
func writeResponseHead(w io.Writer, resp *http.Response) {
	if resp.ProtoMajor == 2 {
		// curl writes "HTTP/2 200 " with a trailing space, because HTTP/2 has no
		// reason phrase; matching it keeps a byte-by-byte diff clean.
		fmt.Fprintf(w, "HTTP/2 %d \n", resp.StatusCode)
	} else {
		fmt.Fprintf(w, "%s %s\n", resp.Proto, resp.Status)
	}
	names := make([]string, 0, len(resp.Header))
	for name := range resp.Header {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, v := range resp.Header[name] {
			fmt.Fprintf(w, "%s: %s\n", strings.ToLower(name), v)
		}
	}
	fmt.Fprint(w, "\n")
}

// writeOutVars are the values -w can print.
type writeOutVars struct {
	code int
	size int64
	took time.Duration
	url  string
	kind string
}

// expandWriteOut turns a -w format into text: \n, 	, \r and \\ escapes, and
// the %{name} variables curl knows. An unknown variable is reported the way curl
// reports it and left in the text, so a typo never silently prints nothing.
func expandWriteOut(format string, v writeOutVars, stderr *os.File) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		switch format[i] {
		case '\\':
			if i+1 >= len(format) {
				b.WriteByte('\\')
				continue
			}
			i++
			switch format[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('	')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte('\\')
				b.WriteByte(format[i])
			}
		case '%':
			if i+1 < len(format) && format[i+1] == '{' {
				if end := strings.IndexByte(format[i+2:], '}'); end >= 0 {
					name := format[i+2 : i+2+end]
					if value, ok := writeOutValue(name, v); ok {
						b.WriteString(value)
						i += 2 + end
						continue
					}
					fmt.Fprintf(stderr, "curl: unknown --write-out variable: '%s'\n", name)
					b.WriteString(format[i : i+3+end])
					i += 2 + end
					continue
				}
			}
			b.WriteByte('%')
		default:
			b.WriteByte(format[i])
		}
	}
	return b.String()
}

func writeOutValue(name string, v writeOutVars) (string, bool) {
	switch name {
	case "http_code", "response_code":
		return strconv.Itoa(v.code), true
	case "size_download":
		return strconv.FormatInt(v.size, 10), true
	case "time_total":
		return strconv.FormatFloat(v.took.Seconds(), 'f', 6, 64), true
	case "url_effective":
		return v.url, true
	case "content_type":
		return v.kind, true
	}
	return "", false
}
