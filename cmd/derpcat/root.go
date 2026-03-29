package main

import (
	"fmt"
	"io"
)

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: derpcat <listen|send> [flags]")
		return 2
	}

	switch args[0] {
	case "listen":
		return runListen(args[1:], stdout, stderr)
	case "send":
		_ = stdin
		return runSend(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n", args[0])
		return 2
	}
}
