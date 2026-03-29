package main

import (
	"flag"
	"fmt"
	"io"
)

func runSend(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: derpcat send <token>")
		return 2
	}

	_ = stdout
	return 0
}
