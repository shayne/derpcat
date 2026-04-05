package main

import (
	"fmt"
	"io"
)

func runOrchestrate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && isRootHelpRequest(args) {
		fmt.Fprint(stderr, subcommandUsageLine("orchestrate"))
		return 0
	}
	if len(args) != 0 {
		fmt.Fprint(stderr, subcommandUsageLine("orchestrate"))
		return 2
	}

	return 0
}
