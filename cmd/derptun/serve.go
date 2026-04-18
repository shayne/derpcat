package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/shayne/derphole/pkg/session"
	"github.com/shayne/derphole/pkg/telemetry"
	"github.com/shayne/yargs"
)

type serveFlags struct {
	Token      string `flag:"token" help:"Durable derptun token"`
	TCP        string `flag:"tcp" help:"Local TCP target to expose, for example 127.0.0.1:22"`
	ForceRelay bool   `flag:"force-relay" help:"Disable direct probing"`
}

var serveHelpConfig = yargs.HelpConfig{
	Command: yargs.CommandInfo{
		Name:        "derptun",
		Description: "Serve a local TCP service through a durable derptun token.",
		Examples: []string{
			"derptun token --days 7",
			"derptun serve --token <token> --tcp 127.0.0.1:22",
			"derptun open --token <token> --listen 127.0.0.1:2222",
		},
	},
	SubCommands: map[string]yargs.SubCommandInfo{
		"serve": {
			Name:        "serve",
			Description: "Expose a local TCP target until Ctrl-C.",
			Usage:       "--token TOKEN --tcp HOST:PORT [--force-relay]",
			Examples: []string{
				"derptun serve --token <token> --tcp 127.0.0.1:22",
			},
		},
	},
}

var derptunServe = session.DerptunServe

func runServe(args []string, level telemetry.Level, stderr io.Writer) int {
	parsed, err := yargs.ParseWithCommandAndHelp[struct{}, serveFlags, struct{}](append([]string{"serve"}, args...), serveHelpConfig)
	if err != nil {
		switch {
		case errors.Is(err, yargs.ErrHelp), errors.Is(err, yargs.ErrSubCommandHelp), errors.Is(err, yargs.ErrHelpLLM):
			if parsed != nil && parsed.HelpText != "" {
				fmt.Fprint(stderr, parsed.HelpText)
			} else {
				fmt.Fprint(stderr, serveHelpText())
			}
			return 0
		default:
			fmt.Fprintln(stderr, err)
			fmt.Fprint(stderr, serveHelpText())
			return 2
		}
	}
	if parsed.SubCommandFlags.Token == "" || parsed.SubCommandFlags.TCP == "" || len(parsed.Parser.Args) != 0 || len(parsed.RemainingArgs) != 0 {
		fmt.Fprint(stderr, serveHelpText())
		return 2
	}

	ctx, stop := commandContext()
	defer stop()
	if err := derptunServe(ctx, session.DerptunServeConfig{
		Token:         parsed.SubCommandFlags.Token,
		TargetAddr:    parsed.SubCommandFlags.TCP,
		Emitter:       telemetry.New(stderr, commandSessionTelemetryLevel(level)),
		ForceRelay:    parsed.SubCommandFlags.ForceRelay,
		UsePublicDERP: usePublicDERPTransport(),
	}); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func serveHelpText() string {
	return yargs.GenerateSubCommandHelp(serveHelpConfig, "serve", struct{}{}, serveFlags{}, struct{}{})
}
