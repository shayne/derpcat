package main

import (
	"flag"
	"fmt"
	"io"
)

func runSend(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "send not implemented")
		return 2
	}

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
