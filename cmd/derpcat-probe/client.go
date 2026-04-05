package main

import (
	"fmt"
	"io"
)

func runClient(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && isRootHelpRequest(args) {
		fmt.Fprint(stderr, subcommandUsageLine("client"))
		return 0
	}
	if len(args) != 0 {
		fmt.Fprint(stderr, subcommandUsageLine("client"))
		return 2
	}

	return 0
}
