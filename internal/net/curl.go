//go:build !nonet

package net

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// curlShortWithValue lists the short switches that take a value. Everything else
// that starts with a dash is a boolean flag and may be clustered (-fsSL).
var curlShortWithValue = map[byte]bool{
	'X': true, 'H': true, 'd': true, 'u': true, 'A': true, 'e': true,
	'x': true, 'o': true, 'b': true, 'c': true, 'r': true, 'T': true,
	'F': true, 'w': true, 'm': true,
}

// runCurl implements curl's HTTP subset. Combined short switches are expanded
// on purpose: -fsSL is what agents and humans write, and rejecting it would be
// the difference between a working script and a mystery.
func runCurl(args []string, stdin *os.File, stdout, stderr *os.File) int {
	opts := netOptions{MaxRedirs: 50, Front: "curl"}
	var urls []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			printCurlHelp(stdout)
			return 0
		case strings.HasPrefix(a, "--"):
			name, value, hasValue := strings.Cut(a[2:], "=")
			takeValue := func() (string, bool) {
				if hasValue {
					return value, true
				}
				if i+1 >= len(args) {
					return "", false
				}
				i++
				return args[i], true
			}
			switch name {
			case "request":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--request needs a method")
				}
				opts.Method = strings.ToUpper(v)
			case "header":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--header needs an argument")
				}
				opts.Headers = append(opts.Headers, v)
			case "data", "data-ascii":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--data needs a value")
				}
				opts.Data = append(opts.Data, v)
			case "data-raw":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--data-raw needs a value")
				}
				opts.DataRaw = v
			case "data-binary":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--data-binary needs a value")
				}
				if strings.HasPrefix(v, "@") {
					opts.DataFile = strings.TrimPrefix(v, "@")
				} else {
					opts.DataRaw = v
				}
			case "form":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--form needs a value")
				}
				opts.Form = append(opts.Form, v)
			case "user":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--user needs user:password")
				}
				opts.User = v
			case "user-agent":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--user-agent needs a value")
				}
				opts.UserAgent = v
			case "referer":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--referer needs a value")
				}
				opts.Referer = v
			case "head":
				opts.Head = true
			case "include":
				opts.Include = true
			case "parallel":
				opts.Parallel = true
			case "parallel-max":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--parallel-max needs a number")
				}
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 || n > 10000 {
					return curlUsage(stderr, "--parallel-max needs a number between 1 and 10000")
				}
				opts.ParallelMax = n
				opts.Parallel = true
			case "repeat":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--repeat needs a number")
				}
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 || n > 1000000 {
					return curlUsage(stderr, "--repeat needs a number between 1 and 1000000")
				}
				opts.Repeat = n
				opts.Parallel = true
			case "no-keepalive":
				opts.NoKeepalive = true
			case "cacert":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--cacert needs a file")
				}
				opts.CAFile = v
			case "resolve":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--resolve needs HOST:PORT:ADDRESS")
				}
				host, port, target, err := splitResolve(v)
				if err != nil {
					return curlUsage(stderr, err.Error())
				}
				if opts.Resolve == nil {
					opts.Resolve = map[string]string{}
				}
				opts.Resolve[net.JoinHostPort(host, port)] = net.JoinHostPort(target, port)
			case "impersonate":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--impersonate needs a profile (chrome, edge, firefox, safari, go)")
				}
				if _, known := impersonationProfile(v); !known {
					return curlUsage(stderr, fmt.Sprintf("unknown impersonation profile %q (chrome, edge, firefox, safari, go)", v))
				}
				opts.Impersonate = v
			case "write-out":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--write-out needs a value")
				}
				opts.WriteOut = v
			case "silent":
				opts.Silent = true
			case "show-error":
				opts.ShowError = true
			case "fail", "fail-with-body":
				opts.Fail = true
			case "location":
				opts.Follow = true
			case "max-redirs":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--max-redirs needs a number")
				}
				n := 0
				fmt.Sscanf(v, "%d", &n)
				opts.MaxRedirs = n
			case "output":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--output needs a file")
				}
				opts.Output = v
			case "remote-name":
				opts.Remote = true
			case "insecure":
				opts.Insecure = true
			case "proxy":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--proxy needs a URL")
				}
				opts.Proxy = v
			case "max-time":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--max-time needs seconds")
				}
				d, err := parseSeconds(v)
				if err != nil {
					return curlUsage(stderr, err.Error())
				}
				opts.MaxTime = d
			case "connect-timeout":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--connect-timeout needs seconds")
				}
				d, err := parseSeconds(v)
				if err != nil {
					return curlUsage(stderr, err.Error())
				}
				opts.Connect = d
			case "cookie":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--cookie needs a value")
				}
				if strings.Contains(v, "=") {
					opts.Cookies = append(opts.Cookies, v)
				} else {
					opts.JarFile = v
				}
			case "cookie-jar":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--cookie-jar needs a file")
				}
				opts.JarFile = v
			case "range":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--range needs a value")
				}
				opts.Range = v
			case "upload-file":
				v, ok := takeValue()
				if !ok {
					return curlUsage(stderr, "--upload-file needs a file")
				}
				opts.Upload = v
			case "get":
				opts.AsGet = true
			case "verbose":
				opts.Verbose = true
			case "compressed":
				opts.Compress = true
			default:
				return curlUsage(stderr, fmt.Sprintf("unsupported option --%s", name))
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			cluster := a[1:]
			for pos := 0; pos < len(cluster); pos++ {
				ch := cluster[pos]
				if curlShortWithValue[ch] {
					value := cluster[pos+1:]
					if value == "" {
						if i+1 >= len(args) {
							return curlUsage(stderr, fmt.Sprintf("-%c needs a value", ch))
						}
						i++
						value = args[i]
					}
					if code := applyCurlShort(ch, value, &opts, stderr); code != 0 {
						return code
					}
					pos = len(cluster)
					continue
				}
				if code := applyCurlBool(ch, &opts, stderr); code != 0 {
					return code
				}
			}
		default:
			urls = append(urls, a)
		}
	}
	if opts.Parallel {
		// -Z with no URL is a mistake; -Z with --repeat is the flood of one page.
		if len(urls) == 0 {
			return curlUsage(stderr, "--parallel needs at least one URL")
		}
		targets := urls
		if opts.Repeat > 0 {
			targets = make([]string, 0, len(urls)*opts.Repeat)
			for i := 0; i < opts.Repeat; i++ {
				targets = append(targets, urls...)
			}
		}
		return runParallel(opts, targets, stderr)
	}
	if len(urls) == 0 {
		return curlUsage(stderr, "no URL given")
	}
	opts.URL = urls[0]
	return runNet(opts, stdin, stdout, stderr)
}

func applyCurlShort(ch byte, value string, opts *netOptions, stderr *os.File) int {
	switch ch {
	case 'X':
		opts.Method = strings.ToUpper(value)
	case 'H':
		opts.Headers = append(opts.Headers, value)
	case 'd':
		opts.Data = append(opts.Data, value)
	case 'u':
		opts.User = value
	case 'A':
		opts.UserAgent = value
	case 'e':
		opts.Referer = value
	case 'x':
		opts.Proxy = value
	case 'o':
		opts.Output = value
	case 'b':
		if strings.Contains(value, "=") {
			opts.Cookies = append(opts.Cookies, value)
		} else {
			opts.JarFile = value
		}
	case 'c':
		opts.JarFile = value
	case 'r':
		opts.Range = value
	case 'T':
		opts.Upload = value
	case 'F':
		opts.Form = append(opts.Form, value)
	case 'm':
		d, err := parseSeconds(value)
		if err != nil {
			return curlUsage(stderr, err.Error())
		}
		opts.MaxTime = d
	case 'w':
		opts.WriteOut = value
	}
	return 0
}

func applyCurlBool(ch byte, opts *netOptions, stderr *os.File) int {
	switch ch {
	case 'Z':
		opts.Parallel = true
	case 's':
		opts.Silent = true
	case 'S':
		opts.ShowError = true
	case 'f':
		opts.Fail = true
	case 'L':
		opts.Follow = true
	case 'i':
		opts.Include = true
	case 'I':
		opts.Head = true
	case 'k':
		opts.Insecure = true
	case 'v':
		opts.Verbose = true
	case 'G':
		opts.AsGet = true
	case 'O':
		opts.Remote = true
	default:
		return curlUsage(stderr, fmt.Sprintf("unsupported option -%c", ch))
	}
	return 0
}

func curlUsage(stderr *os.File, msg string) int {
	fmt.Fprintf(stderr, "curl: %s\n", msg)
	fmt.Fprintln(stderr, "usage: curl [options] URL  (curl -h for the list)")
	return 2
}

func printCurlHelp(stdout *os.File) {
	fmt.Fprint(stdout, `usage: curl [options] URL

  -X, --request METHOD      method (GET, POST, PUT, ...)
  -H, --header "Name: v"    add a request header
      --cacert FILE         verify the server with these roots, not the system's
      --resolve H:P:ADDR    dial ADDR for host H on port P, the name stays
  -d, --data DATA           form encoded body, repeatable
      --data-raw DATA       body as typed
      --data-binary @FILE   body from a file
  -F, --form name=value     multipart form, @file for upload
  -u, --user USER:PASS      basic auth
  -A, --user-agent VALUE    user agent
  -e, --referer VALUE       referer
  -I, --head                HEAD request
  -i, --include             print response headers
  -s, --silent              no messages (body still printed)
  -S, --show-error          show errors even with -s
  -f, --fail                HTTP >= 400 is an error (exit 22)
  -L, --location            follow redirects
      --max-redirs N        redirect limit (default 50)
  -o, --output FILE         write body to FILE
  -O, --remote-name         take the file name from the URL
  -k, --insecure            skip TLS verification
  -x, --proxy URL           proxy
      --max-time SEC        whole request timeout
      --connect-timeout SEC connect timeout
  -b, --cookie DATA         cookie or jar file
  -c, --cookie-jar FILE     write the jar
  -r, --range A-B           byte range
  -T, --upload-file FILE    PUT a file
  -G, --get                 put data in the query string
  -v, --verbose             show request and response
      --compressed          ask for gzip
  -w, --write-out FORMAT    print http_code, size_download, url_effective, ...
      --impersonate PROFIL  TLS-Handshake wie chrome, edge, firefox, safari, go

Short switches may be combined: -fsSL. Only http and https are supported.
`)
}

// splitResolve parses curl's HOST:PORT:ADDRESS. The address may carry its own
// port after a comma, but only the plain form is accepted here: a wrong guess
// about a foreign syntax would connect somewhere unexpected.
func splitResolve(v string) (host, port, addr string, err error) {
	parts := strings.Split(v, ":")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("--resolve wants HOST:PORT:ADDRESS, got %q", v)
	}
	host, port, addr = parts[0], parts[1], parts[2]
	if host == "" || port == "" || addr == "" {
		return "", "", "", fmt.Errorf("--resolve wants HOST:PORT:ADDRESS, got %q", v)
	}
	if strings.ContainsAny(addr, "+*,") {
		// A comma would be curl's HOST:PORT:ADDRESS:PORT form.
		// curl's "+" keeps the address for other requests; refusing is clearer
		// than half-doing it.
		return "", "", "", fmt.Errorf("--resolve does not support the '+', '*' or ',port' address forms")
	}
	return host, port, addr, nil
}
