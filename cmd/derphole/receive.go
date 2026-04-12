package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/shayne/derpcat/pkg/telemetry"
	"github.com/shayne/yargs"
)

type receiveFlags struct{}

type receiveArgs struct {
	Code string `pos:"0?" help:"Optional receive code"`
}

var receiveHelpConfig = yargs.HelpConfig{
	Command: yargs.CommandInfo{
		Name:        "derphole",
		Description: "Receive text, files, or directories with wormhole-shaped UX on top of derpcat transport.",
		Examples: []string{
			"derphole receive",
			"derphole receive 7-purple-sausages",
			"derphole rx",
		},
	},
	SubCommands: map[string]yargs.SubCommandInfo{
		"receive": {
			Name:        "receive",
			Description: "Receive text, a file, or a directory.",
			Usage:       "[code]",
			Examples: []string{
				"derphole receive",
				"derphole receive 7-purple-sausages",
			},
		},
	},
}

func runReceive(args []string, level telemetry.Level, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = level
	_ = stdin
	_ = stdout

	parsed, err := yargs.ParseWithCommandAndHelp[struct{}, receiveFlags, receiveArgs](append([]string{"receive"}, args...), receiveHelpConfig)
	if err != nil {
		switch {
		case errors.Is(err, yargs.ErrHelp), errors.Is(err, yargs.ErrSubCommandHelp), errors.Is(err, yargs.ErrHelpLLM):
			if parsed != nil && parsed.HelpText != "" {
				fmt.Fprint(stderr, parsed.HelpText)
			} else {
				fmt.Fprint(stderr, receiveHelpText())
			}
			return 0
		default:
			fmt.Fprintln(stderr, err)
			fmt.Fprint(stderr, receiveHelpText())
			return 2
		}
	}

	if len(parsed.Parser.Args) > 1 || len(parsed.RemainingArgs) != 0 {
		fmt.Fprint(stderr, receiveHelpText())
		return 2
	}

	fmt.Fprintln(stderr, "derphole receive is not implemented yet")
	return 1
}

func receiveHelpText() string {
	return yargs.GenerateSubCommandHelp(receiveHelpConfig, "receive", struct{}{}, receiveFlags{}, receiveArgs{})
}
