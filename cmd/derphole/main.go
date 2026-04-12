package main

import (
	"io"
	"os"
)

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run(args, stdin, stdout, stderr)
}
