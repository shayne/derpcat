package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/shayne/derpcat/pkg/telemetry"
	"github.com/shayne/yargs"
)

type sendFlags struct{}

type sendArgs struct {
	What string `pos:"0?" help:"Optional text, file, or directory to send"`
}

var sendHelpConfig = yargs.HelpConfig{
	Command: yargs.CommandInfo{
		Name:        "derphole",
		Description: "Send text, files, or directories with wormhole-shaped UX on top of derpcat transport.",
		Examples: []string{
			"derphole send hello",
			"derphole send ./photo.jpg",
			"derphole tx ./project-dir",
		},
	},
	SubCommands: map[string]yargs.SubCommandInfo{
		"send": {
			Name:        "send",
			Description: "Send text, a file, or a directory.",
			Usage:       "[what]",
			Examples: []string{
				"derphole send hello",
				"derphole send ./photo.jpg",
			},
		},
	},
}

func runSend(args []string, level telemetry.Level, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = level
	_ = stdin
	_ = stdout

	parsed, err := yargs.ParseWithCommandAndHelp[struct{}, sendFlags, sendArgs](append([]string{"send"}, args...), sendHelpConfig)
	if err != nil {
		switch {
		case errors.Is(err, yargs.ErrHelp), errors.Is(err, yargs.ErrSubCommandHelp), errors.Is(err, yargs.ErrHelpLLM):
			if parsed != nil && parsed.HelpText != "" {
				fmt.Fprint(stderr, parsed.HelpText)
			} else {
				fmt.Fprint(stderr, sendHelpText())
			}
			return 0
		default:
			fmt.Fprintln(stderr, err)
			fmt.Fprint(stderr, sendHelpText())
			return 2
		}
	}

	if len(parsed.Parser.Args) > 1 || len(parsed.RemainingArgs) != 0 {
		fmt.Fprint(stderr, sendHelpText())
		return 2
	}

	fmt.Fprintln(stderr, "derphole send is not implemented yet")
	return 1
}

func sendHelpText() string {
	return yargs.GenerateSubCommandHelp(sendHelpConfig, "send", struct{}{}, sendFlags{}, sendArgs{})
}
