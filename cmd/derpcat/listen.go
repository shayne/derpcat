package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/shayne/derpcat/pkg/telemetry"
	"github.com/shayne/derpcat/pkg/token"
)

func runListen(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "listen not implemented")
		return 2
	}

	fs := flag.NewFlagSet("listen", flag.ContinueOnError)
	fs.SetOutput(stderr)

	printTokenOnly := fs.Bool("print-token-only", false, "print only the session token")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	tok, err := token.Encode(token.Token{
		Version:      token.SupportedVersion,
		ExpiresUnix:  time.Now().Add(10 * time.Minute).Unix(),
		Capabilities: token.CapabilityStdio | token.CapabilityTCP,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if *printTokenOnly {
		fmt.Fprintln(stdout, tok)
		return 0
	}

	telemetry.New(stderr, telemetry.LevelDefault).Status("waiting-for-claim")
	fmt.Fprintln(stdout, tok)
	return 0
}
