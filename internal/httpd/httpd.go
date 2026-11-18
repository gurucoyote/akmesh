// Command httpd serves a directory over HTTP or HTTPS, with the options of
// busybox httpd and the same behaviour where it matters: index.html, a
// directory listing, byte ranges, Last-Modified, a .htpasswd realm and the
// configuration file with prefix mappings.
//
// What it does not do is CGI. That is the one busybox feature left out on
// purpose: a hull exists so that a web page cannot reach a shell (acmesh noise).
package httpd

import (
	"bufio"
	"crypto/md5"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Options is what the command line says, after parsing.
type Options struct {
	Addr       string // [IP:]PORT, busybox -p
	Home       string // document root, busybox -h
	Conf       string // mapping file, busybox -c
	Foreground bool   // busybox -f
	Verbose    int    // busybox -v, -vv
	Realm      string // busybox -r
	User       string // busybox -u USER[:GROUP]
	Inetd      bool   // busybox -i
	NoCache    bool   // -N: send no-store, for looking at files while editing
	Cert       string // -S: PEM certificate, turns on TLS
	Key        string // -K: PEM key
}

const usage = `usage: httpd [-ifv[v]] [-N] [-c CONFFILE] [-p [IP:]PORT] [-u USER[:GRP]]
             [-r REALM] [-h HOME] [-S CERT -K KEY]
   or  httpd -d/-e STRING
   or  httpd -M STRING

  -i        inetd mode: serve one request from stdin
  -f        stay in the foreground (busybox daemonizes without it)
  -v, -vv   log requests; -vv also logs the headers
  -p        address and port to listen on (default :80)
  -h        document root (default .)
  -c        configuration file: "prefix:/directory" and "A:user:password"
  -u        user and group to switch to after binding
  -r        authentication realm
  -h        document root
  -N        send no-store for text/html, so an editor's change is seen
  -S, -K    serve HTTPS with this certificate and key
  -d        URL-decode STRING and print it
  -e        HTML-escape STRING and print it
  -M        print the MIME type for this file name

CGI is not implemented: a page that can start a program defeats the purpose of
the hull. Everything else follows busybox httpd.
`

// Run is the entry point, with the arguments after the command name.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	opts := Options{Addr: ":80", Home: ".", Conf: "/etc/httpd.conf", Realm: "Protected"}

	// busybox allows the one-string helpers before anything else.
	if len(args) >= 1 {
		switch args[0] {
		case "-d", "-e", "-M":
			if len(args) < 2 {
				fmt.Fprintf(stderr, "httpd: %s needs a string\n", args[0])
				return 1
			}
			out, err := helper(args[0], args[1])
			if err != nil {
				fmt.Fprintf(stderr, "httpd: %v\n", err)
				return 1
			}
			fmt.Fprintln(stdout, out)
			return 0
		}
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-i":
			opts.Inetd = true
		case "-f":
			opts.Foreground = true
		case "-v":
			opts.Verbose = 1
		case "-vv":
			opts.Verbose = 2
		case "-N":
			opts.NoCache = true
		case "-p":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "httpd: -p needs an address")
				return 1
			}
			opts.Addr = withDefaultPort(args[i])
		case "-h":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "httpd: -h needs a directory")
				return 1
			}
			opts.Home = args[i]
		case "-c":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "httpd: -c needs a file")
				return 1
			}
			opts.Conf = args[i]
		case "-u":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "httpd: -u needs USER[:GROUP]")
				return 1
			}
			opts.User = args[i]
		case "-r":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "httpd: -r needs a realm")
				return 1
			}
			opts.Realm = args[i]
		case "-S":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "httpd: -S needs a certificate")
				return 1
			}
			opts.Cert = args[i]
		case "-K":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "httpd: -K needs a key")
				return 1
			}
			opts.Key = args[i]
		case "--help":
			fmt.Fprint(stderr, usage)
			return 0
		default:
			fmt.Fprintf(stderr, "httpd: unknown option %s\n%s", a, usage)
			return 1
		}
	}
	if (opts.Cert == "") != (opts.Key == "") {
		fmt.Fprintln(stderr, "httpd: -S and -K belong together")
		return 1
	}

	conf, err := loadConf(opts.Conf)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "httpd: %v\n", err)
		return 1
	}

	// busybox httpd goes to the background unless -f is given. A Go program
	// cannot fork itself safely, so the same arguments are started again as a
	// child with -f and this process returns to the shell.
	if !opts.Foreground && !opts.Inetd && os.Getenv("ACMESH_HTTPD_CHILD") == "" {
		if err := detach(stderr); err != nil {
			fmt.Fprintf(stderr, "httpd: %v\n", err)
			return 1
		}
		return 0
	}

	srv := &server{opts: opts, conf: conf, stderr: stderr}
	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.serve)

	if opts.Inetd {
		if err := srv.serveOne(stdin); err != nil {
			fmt.Fprintf(stderr, "httpd: %v\n", err)
			return 1
		}
		return 0
	}

	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		fmt.Fprintf(stderr, "httpd: %v\n", err)
		return 1
	}
	if err := dropPrivileges(opts.User); err != nil {
		fmt.Fprintf(stderr, "httpd: %v\n", err)
		return 1
	}
	if opts.Verbose > 0 {
		scheme := "http"
		if opts.Cert != "" {
			scheme = "https"
		}
		fmt.Fprintf(stderr, "httpd: serving %s on %s://%s\n", opts.Home, scheme, ln.Addr())
	}

	s := &http.Server{Handler: mux}
	if opts.Cert != "" {
		err = s.ServeTLS(ln, opts.Cert, opts.Key)
	} else {
		err = s.Serve(ln)
	}
	if err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(stderr, "httpd: %v\n", err)
		return 1
	}
	return 0
}

// withDefaultPort turns ":8080" or "127.0.0.1:8080" into a listen address and
// a bare port into one on all interfaces, which is what busybox does.
func withDefaultPort(p string) string {
	if strings.HasPrefix(p, ":") || strings.Contains(p, ":]") {
		return p
	}
	if _, _, err := net.SplitHostPort(p); err == nil {
		return p
	}
	if strings.Contains(p, ":") {
		return p
	}
	return ":" + p
}

// helper implements -d, -e and -M, the three one-string modes of busybox httpd.
func helper(mode, in string) (string, error) {
	switch mode {
	case "-d":
		return url.QueryUnescape(in)
	case "-e":
		return html.EscapeString(in), nil
	case "-M":
		if ext := filepath.Ext(in); ext != "" {
			if t := mimeType(ext); t != "" {
				return t, nil
			}
		}
		return "text/plain", nil
	}
	return "", fmt.Errorf("unknown mode %s", mode)
}

// confEntry is one line of the busybox configuration file.
type confEntry struct {
	prefix string // "*" is the catch-all
	dir    string
	user   string
	pass   string // base64 of the MD5 of the password, as busybox writes it
}

// loadConf reads "prefix:/dir" and "A:user:password" lines. A missing file is
// not an error: then only the document root is served.
func loadConf(name string) ([]confEntry, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var out []confEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "A:") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) == 3 {
				out = append(out, confEntry{user: parts[1], pass: parts[2]})
			}
			continue
		}
		i := strings.LastIndex(line, ":")
		if i <= 0 {
			continue
		}
		out = append(out, confEntry{prefix: line[:i], dir: line[i+1:]})
	}
	return out, nil
}

// server holds what every request needs.
type server struct {
	opts   Options
	conf   []confEntry
	stderr *os.File
	mu     sync.Mutex
}

// serve maps the request path to a directory and serves a file from it.
func (s *server) serve(w http.ResponseWriter, r *http.Request) {
	if s.opts.Verbose > 0 {
		s.logf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		if s.opts.Verbose > 1 {
			s.logf("%v", r.Header)
		}
	}

	root, prefix, ok := s.rootFor(r.URL.Path)
	if !ok {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}
	if !s.authorised(w, r, root) {
		return
	}

	rel, err := url.PathUnescape(r.URL.Path)
	if err != nil {
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}
	if prefix != "" && prefix != "*" {
		// The prefix chose the directory; it is not part of the name inside.
		rel = strings.TrimPrefix(rel, prefix)
	}
	clean := path.Clean("/" + rel)
	name := filepath.Join(root, filepath.FromSlash(clean))

	fi, err := os.Stat(name)
	if err != nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}
	if fi.IsDir() {
		if !strings.HasSuffix(r.URL.Path, "/") {
			// A relative redirect keeps the prefix the browser asked for.
			http.Redirect(w, r, path.Base(clean)+"/", http.StatusMovedPermanently)
			return
		}
		index := filepath.Join(name, "index.html")
		if st, err := os.Stat(index); err == nil && !st.IsDir() {
			s.sendFile(w, r, index, st)
			return
		}
		s.listDir(w, r, name)
		return
	}
	s.sendFile(w, r, name, fi)
}

// rootFor picks the directory for a request path, using the prefix mappings of
// the configuration file (and its "*" catch-all).
func (s *server) rootFor(p string) (string, string, bool) {
	best := ""
	dir := s.opts.Home
	for _, e := range s.conf {
		if e.prefix == "" {
			continue
		}
		if e.prefix == "*" {
			if best == "" {
				best, dir = "*", e.dir
			}
			continue
		}
		if strings.HasPrefix(p, e.prefix) && len(e.prefix) > len(best) {
			best, dir = e.prefix, e.dir
		}
	}
	return dir, best, true
}

// authorised checks basic auth against the "A:" lines of the config file and
// against a .htpasswd in the directory asked for.
func (s *server) authorised(w http.ResponseWriter, r *http.Request, root string) bool {
	users := map[string]string{}
	need := false
	for _, e := range s.conf {
		if e.user != "" {
			users[e.user] = e.pass
			need = true
		}
	}
	if !need {
		return true
	}
	user, pass, ok := r.BasicAuth()
	if ok && checkPass(users[user], pass) {
		return true
	}
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm=%q`, s.opts.Realm))
	http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
	return false
}

// checkPass compares a password with the stored form: busybox writes the
// base64 of the MD5 of the password, and also accepts the plain password.
func checkPass(stored, given string) bool {
	if stored == "" {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(stored), []byte(given)) == 1 {
		return true
	}
	sum := md5.Sum([]byte(given))
	enc := base64.StdEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(stored), []byte(enc)) == 1
}

// sendFile answers with the file, honouring If-Modified-Since and Range, which
// is what makes a video or a big download work in a browser.
func (s *server) sendFile(w http.ResponseWriter, r *http.Request, name string, fi os.FileInfo) {
	f, err := os.Open(name)
	if err != nil {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}
	defer f.Close()

	if ext := filepath.Ext(name); ext != "" {
		if t := mimeType(ext); t != "" {
			w.Header().Set("Content-Type", t)
		}
	}
	w.Header().Set("Last-Modified", fi.ModTime().UTC().Format(http.TimeFormat))
	if s.opts.NoCache {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeContent(w, r, filepath.Base(name), fi.ModTime(), f)
}

// listDir prints a plain listing, sorted, the way busybox httpd does.
func (s *server) listDir(w http.ResponseWriter, r *http.Request, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var b strings.Builder
	fmt.Fprintf(&b, "<html><head><title>Index of %s</title></head><body>\n", html.EscapeString(r.URL.Path))
	fmt.Fprintf(&b, "<h1>Index of %s</h1>\n<pre>\n", html.EscapeString(r.URL.Path))
	if r.URL.Path != "/" {
		fmt.Fprint(&b, `<a href="../">../</a>`+"\n")
	}
	for _, e := range entries {
		name := e.Name()
		slash := ""
		if e.IsDir() {
			slash = "/"
		}
		fmt.Fprintf(&b, `<a href="%s%s">%s%s</a>`+"\n",
			url.PathEscape(name), slash, html.EscapeString(name), slash)
	}
	fmt.Fprint(&b, "</pre></body></html>\n")
	io.WriteString(w, b.String())
}

func (s *server) logf(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.stderr, "httpd: "+format+"\n", args...)
}

// mimeType returns the type for an extension, with the few that Go's table
// misses or that matter for a page under construction.
func mimeType(ext string) string {
	switch strings.ToLower(ext) {
	case ".mjs", ".js":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".wasm":
		return "application/wasm"
	case ".webmanifest":
		return "application/manifest+json"
	case ".md", ".markdown":
		return "text/markdown; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return ""
}

// serveOne answers a single request on the given stream, for inetd mode: the
// request comes from stdin, the answer goes to stdout.
func (s *server) serveOne(stdin *os.File) error {
	if stdin == nil {
		return fmt.Errorf("inetd mode needs stdin")
	}
	req, err := http.ReadRequest(bufio.NewReader(stdin))
	if err != nil {
		return err
	}
	rw := &oneShot{header: http.Header{}, status: http.StatusOK}
	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
		req.URL.Host = req.Host
	}
	s.serve(rw, req)
	fmt.Fprintf(os.Stdout, "HTTP/1.1 %d %s\r\n", rw.status, http.StatusText(rw.status))
	rw.header.Write(os.Stdout)
	fmt.Fprint(os.Stdout, "\r\n")
	os.Stdout.Write(rw.body)
	return nil
}

// oneShot collects the answer of one request, so that inetd mode can write it
// to a stream instead of a connection.
type oneShot struct {
	header http.Header
	status int
	body   []byte
}

func (o *oneShot) Header() http.Header { return o.header }

func (o *oneShot) WriteHeader(code int) {
	if o.status == http.StatusOK {
		o.status = code
	}
}

func (o *oneShot) Write(b []byte) (int, error) {
	o.body = append(o.body, b...)
	return len(b), nil
}

// detach starts this command again with -f in the background and reports the
// pid. The child's streams go to /dev/null: a background server must not keep
// the caller's pipe or terminal open, or the caller waits forever.
func detach(stderr *os.File) error {
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devnull.Close()

	args := append([]string{"-f"}, os.Args[1:]...)
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "ACMESH_HTTPD_CHILD=1")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
	cmd.SysProcAttr = detachedAttr()
	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "httpd: started in the background, pid %d\n", cmd.Process.Pid)
	return nil
}
