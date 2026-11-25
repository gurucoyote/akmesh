// Package tooljq carries jq, the one command that links gojq.
//
// Its own package so that gojq and its time-formatting helper stay out of every
// binary that does not offer jq; the cost is about a megabyte (measured in
// docs/design/split.md, acmesh-58me).
package tooljq

import (
	"os"

	"akmesh/internal/cmdrun"
	"akmesh/internal/jq"
)

// Commands is the table of this block.
var Commands = map[string]cmdrun.Func{
	"jq": runJq,
}

// Run is the entry point for cmd/jq/main.go.
func Run(name string) int { return cmdrun.Run(Commands, name) }

func runJq(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return jq.Run(args, stdin, stdout, stderr)
}
