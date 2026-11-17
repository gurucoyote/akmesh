package tools

import (
	"fmt"
	"os"

	"acmesh/internal/policy"
)

// runPolicy explains and tests the policy of the current process. It is the
// command a harness calls to see what is enforced, and it is how the test
// battery proves that reading outside the list really fails.
func runPolicy(args []string, stdin *os.File, stdout, stderr *os.File) int {
	check := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--check", "-c":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "policy: --check needs a path")
				return 2
			}
			i++
			check = args[i]
		case "--help", "-h":
			fmt.Fprintln(stdout, "usage: policy [--check PATH]")
			fmt.Fprintln(stdout, "prints the enforced policy; with --check PATH it tries to read PATH")
			return 0
		default:
			fmt.Fprintf(stderr, "policy: unknown argument %q\n", args[i])
			return 2
		}
	}

	active := policy.Active()
	if err := policy.InitError(); err != nil {
		fmt.Fprintf(stderr, "policy: %v\n", err)
		return 1
	}

	if check != "" {
		// The check is a real read in this process: if the policy is enforced,
		// the kernel refuses here and the exit code says so.
		if f, err := os.Open(check); err != nil {
			fmt.Fprintf(stdout, "denied: %s (%v)\n", check, err)
			return 1
		} else {
			f.Close()
			fmt.Fprintf(stdout, "allowed: %s\n", check)
			return 0
		}
	}

	if active == nil {
		fmt.Fprintln(stdout, "no policy: ACMESH_POLICY is not set, this process is unrestricted")
		return 1
	}
	ok, detail := policy.Supported()
	fmt.Fprintf(stdout, "policy: %s\n", active.Source())
	fmt.Fprintf(stdout, "enforcement: %s\n", detail)
	fmt.Fprintln(stdout, "read paths:")
	for _, p := range active.ReadPaths() {
		fmt.Fprintf(stdout, "  %s\n", p)
	}
	fmt.Fprintln(stdout, "write paths:")
	for _, p := range active.WritePaths() {
		fmt.Fprintf(stdout, "  %s\n", p)
	}
	fmt.Fprintln(stdout, "exec paths:")
	for _, p := range active.ExecPaths() {
		fmt.Fprintf(stdout, "  %s\n", p)
	}
	fmt.Fprintln(stdout, "network:")
	for _, h := range active.Hosts() {
		fmt.Fprintf(stdout, "  %s\n", h)
	}
	if !ok {
		return 1
	}
	return 0
}
