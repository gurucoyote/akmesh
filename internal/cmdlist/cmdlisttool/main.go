// Command cmdlisttool answers with the command names of one group, so that
// scripts/build-bb.sh can exclude a whole block without owning a second copy of
// the table in internal/cmdlist.
//
//	go run ./internal/cmdlist/cmdlisttool services
//
// One name per line, sorted. An unknown id exits 2: a typo in a profile must
// fail the build, not quietly exclude nothing.
package main

import (
	"fmt"
	"os"

	"github.com/gurucoyote/akmesh/internal/cmdlist"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: cmdlisttool GROUPID [GROUPID...]")
		fmt.Fprintln(os.Stderr, "known ids: documents, files, system, services, network, shell, other")
		os.Exit(2)
	}
	for _, id := range args {
		names, ok := cmdlist.NamesIn(id)
		if !ok {
			fmt.Fprintf(os.Stderr, "cmdlisttool: unknown group %q\n", id)
			os.Exit(2)
		}
		for _, name := range names {
			fmt.Println(name)
		}
	}
}
