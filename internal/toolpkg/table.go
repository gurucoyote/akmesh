// Package toolpkg carries the apk commands: install, build, report and guard.
//
// It is its own package because apk runs package scripts in akmesh's shell and
// therefore links mvdan.cc/sh - about 5 MiB a file-and-text binary must not
// carry. Measured in docs/design/split.md; acmesh-58me.
package toolpkg

import "akmesh/internal/cmdrun"

// Commands is the package block's table.
var Commands = map[string]cmdrun.Func{
	"apk":       runApk,
	"apkbuild":  runApkbuild,
	"apkguard":  runApkguard,
	"apkreport": runApkreport,
}

// Run is the entry point for cmd/<name>/main.go.
func Run(name string) int { return cmdrun.Run(Commands, name) }
