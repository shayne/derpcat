package main

import (
	"fmt"
	"io"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func versionString() string {
	_, _ = commit, buildDate
	return version
}

func runVersion(stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, versionString())
	_ = stderr
	return 0
}
