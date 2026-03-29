package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/shayne/derpcat/pkg/session"
	"github.com/shayne/derpcat/pkg/telemetry"
	"github.com/shayne/yargs"
)

type listenFlags struct {
	PrintTokenOnly bool   `flag:"print-token-only" help:"Print only the session token"`
	ForceRelay     bool   `flag:"force-relay" help:"Disable direct probing"`
	TCPListen      string `flag:"tcp-listen" help:"Accept one local TCP connection and forward its bytes to the session sink"`
	TCPConnect     string `flag:"tcp-connect" help:"Connect to a local TCP service and forward session bytes to it"`
}

type listenGlobalFlags struct {
	Verbose bool `flag:"verbose" short:"v" help:"Show relay status updates"`
	Quiet   bool `flag:"quiet" short:"q" help:"Reduce relay status output"`
	Silent  bool `flag:"silent" short:"s" help:"Suppress relay status output"`
}

var listenHelpConfig = yargs.HelpConfig{
	Command: yargs.CommandInfo{
		Name:        "derpcat",
		Description: "Move one byte stream between hosts over public DERP with direct UDP promotion when available.",
		Examples: []string{
			"derpcat listen",
			"cat file | derpcat send <token>",
			"derpcat version",
		},
	},
	SubCommands: map[string]yargs.SubCommandInfo{
		"listen": {
			Name:        "listen",
			Description: "Listen for one incoming derpcat session and receive data.",
			Usage:       "[--print-token-only] [--tcp-listen addr | --tcp-connect addr] [--force-relay]",
			Examples: []string{
				"derpcat listen",
				"derpcat listen --tcp-connect 127.0.0.1:9000",
			},
		},
	},
}

func runListen(args []string, level telemetry.Level, stdout, stderr io.Writer) int {
	commandArgs := append([]string{"listen"}, args...)
	parsed, err := yargs.ParseWithCommandAndHelp[listenGlobalFlags, listenFlags, struct{}](commandArgs, listenHelpConfig)
	if err != nil {
		if errors.Is(err, yargs.ErrHelp) || errors.Is(err, yargs.ErrSubCommandHelp) || errors.Is(err, yargs.ErrHelpLLM) {
			if parsed != nil {
				fmt.Fprint(stderr, parsed.HelpText)
			} else {
				fmt.Fprint(stderr, yargs.GenerateSubCommandHelpLLM(
					listenHelpConfig,
					"listen",
					listenGlobalFlags{},
					listenFlags{},
					struct{}{},
				))
			}
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}

	known, err := yargs.ParseKnownFlags[listenFlags](args, yargs.KnownFlagsOptions{})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if len(known.RemainingArgs) != 0 {
		fmt.Fprint(stderr, yargs.GenerateSubCommandHelp(
			listenHelpConfig,
			"listen",
			listenGlobalFlags{},
			listenFlags{},
			struct{}{},
		))
		return 2
	}

	if known.Flags.TCPListen != "" && known.Flags.TCPConnect != "" {
		fmt.Fprintln(stderr, "listen: --tcp-listen and --tcp-connect are mutually exclusive")
		return 2
	}

	emitter := telemetry.New(stderr, level)
	tokenSink := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		_, err := session.Listen(context.Background(), session.ListenConfig{
			Emitter:       emitter,
			TokenSink:     tokenSink,
			StdioOut:      stdout,
			Attachment:    nil,
			TCPListen:     known.Flags.TCPListen,
			TCPConnect:    known.Flags.TCPConnect,
			ForceRelay:    known.Flags.ForceRelay,
			UsePublicDERP: usePublicDERPTransport(),
		})
		done <- err
	}()

	var tok string
	select {
	case tok = <-tokenSink:
	case err := <-done:
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if tok == "" {
		fmt.Fprintln(stderr, "failed to issue session token")
		return 1
	}

	tokenOut := stderr
	if known.Flags.PrintTokenOnly {
		tokenOut = stdout
	}
	fmt.Fprintln(tokenOut, tok)

	if err := <-done; err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
