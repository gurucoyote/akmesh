//go:build !nonet

package net

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// runWget is the wget front on the same engine as curl. It covers the switches
// an agent or a script actually uses; the recursive mirroring of GNU wget is
// deliberately absent, and saying so with a clear message is better than
// pretending.
func runWget(args []string, stdin *os.File, stdout, stderr *os.File) int {
	opts := netOptions{Follow: true, MaxRedirs: 20, ShowError: true}
	spider := false
	serverResponse := false
	var urls []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		takeValue := func(what string) (string, bool) {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "wget: %s needs a value\n", what)
				return "", false
			}
			i++
			return args[i], true
		}
		switch {
		case a == "--help" || a == "-h":
			printWgetHelp(stdout)
			return 0
		case a == "-O" || a == "--output-document":
			v, ok := takeValue(a)
			if !ok {
				return 2
			}
			opts.Output = v
		case strings.HasPrefix(a, "--output-document="):
			opts.Output = strings.TrimPrefix(a, "--output-document=")
		case strings.HasPrefix(a, "-O") && len(a) > 2:
			opts.Output = a[2:]
		case a == "-q" || a == "--quiet" || a == "--no-verbose":
			opts.Silent = true
		case a == "-c" || a == "--continue":
			opts.Resume = true
		case a == "--header":
			v, ok := takeValue(a)
			if !ok {
				return 2
			}
			opts.Headers = append(opts.Headers, v)
		case strings.HasPrefix(a, "--header="):
			opts.Headers = append(opts.Headers, strings.TrimPrefix(a, "--header="))
		case a == "--post-data":
			v, ok := takeValue(a)
			if !ok {
				return 2
			}
			opts.DataRaw = v
		case strings.HasPrefix(a, "--post-data="):
			opts.DataRaw = strings.TrimPrefix(a, "--post-data=")
		case a == "--no-check-certificate":
			opts.Insecure = true
		case a == "-U" || a == "--user-agent":
			v, ok := takeValue(a)
			if !ok {
				return 2
			}
			opts.UserAgent = v
		case a == "-T" || a == "--timeout":
			v, ok := takeValue(a)
			if !ok {
				return 2
			}
			d, err := parseSeconds(v)
			if err != nil {
				fmt.Fprintf(stderr, "wget: %v\n", err)
				return 2
			}
			opts.MaxTime = d
		case a == "--spider":
			spider = true
			opts.Head = true
		case a == "-S" || a == "--server-response":
			serverResponse = true
			opts.Include = true
		case a == "-r" || a == "--recursive" || a == "-m" || a == "--mirror":
			fmt.Fprintln(stderr, "wget: recursive download is not supported (use curl with a script)")
			return 2
		case a == "-N" || a == "--timestamping":
			fmt.Fprintln(stderr, "wget: -N (timestamping) is not supported")
			return 2
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(stderr, "wget: unsupported option %q\n", a)
			return 2
		default:
			urls = append(urls, a)
		}
	}
	if len(urls) == 0 {
		fmt.Fprintln(stderr, "usage: wget [options] URL")
		return 2
	}
	opts.URL = urls[0]
	if spider {
		opts.Output = "-"
	}
	if serverResponse && opts.Silent {
		// -S wins over -q for the headers, which is what an agent asking for
		// both wants to see.
		opts.Include = true
	}
	if !opts.Silent && !spider {
		host := opts.URL
		if idx := strings.Index(host, "://"); idx >= 0 {
			host = host[idx+3:]
		}
		if idx := strings.IndexAny(host, "/?"); idx >= 0 {
			host = host[:idx]
		}
		fmt.Fprintf(stderr, "-- %s (%s) --\n", opts.URL, time.Now().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(stderr, "Resolving %s\n", host)
	}
	code := runNet(opts, stdin, stdout, stderr)
	if spider && code == 0 {
		fmt.Fprintf(stderr, "Remote file exists.\n")
	}
	return code
}

func printWgetHelp(stdout *os.File) {
	fmt.Fprint(stdout, `usage: wget [options] URL

  -O, --output-document FILE   write to FILE, - for stdout
  -q, --quiet                  no messages
      --no-verbose             same as -q
  -c, --continue               resume a partial download
      --header "Name: v"       add a request header
      --post-data DATA         POST with a body
      --no-check-certificate   skip TLS verification
  -U, --user-agent VALUE       user agent
  -T, --timeout SEC            timeout
      --spider                 only check that the file exists
  -S, --server-response        show response headers

Not supported: recursive mirroring (-r, -m) and timestamping (-N).
Only http and https are supported.
`)
}
