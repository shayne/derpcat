package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/shayne/derphole/pkg/derptun"
	"github.com/shayne/yargs"
)

type tokenFlags struct {
	Days    int    `flag:"days" help:"Token lifetime in days"`
	Expires string `flag:"expires" help:"Absolute expiry as RFC3339 or YYYY-MM-DD"`
}

var tokenHelpConfig = yargs.HelpConfig{
	Command: yargs.CommandInfo{
		Name:        "derptun",
		Description: "Generate durable derptun tokens.",
		Examples: []string{
			"derptun token --days 7",
			"derptun token --expires 2026-05-01T00:00:00Z",
			"derptun serve --token <token> --tcp 127.0.0.1:22",
		},
	},
	SubCommands: map[string]yargs.SubCommandInfo{
		"token": {
			Name:        "token",
			Description: "Generate a durable token for a derptun tunnel.",
			Usage:       "[--days N] [--expires RFC3339|YYYY-MM-DD]",
			Examples: []string{
				"derptun token --days 7",
				"derptun token --expires 2026-05-01T00:00:00Z",
			},
		},
	},
}

func runToken(args []string, stdout, stderr io.Writer) int {
	parsed, err := yargs.ParseWithCommandAndHelp[struct{}, tokenFlags, struct{}](append([]string{"token"}, args...), tokenHelpConfig)
	if err != nil {
		switch {
		case errors.Is(err, yargs.ErrHelp), errors.Is(err, yargs.ErrSubCommandHelp), errors.Is(err, yargs.ErrHelpLLM):
			if parsed != nil && parsed.HelpText != "" {
				fmt.Fprint(stderr, parsed.HelpText)
			} else {
				fmt.Fprint(stderr, tokenHelpText())
			}
			return 0
		default:
			fmt.Fprintln(stderr, err)
			fmt.Fprint(stderr, tokenHelpText())
			return 2
		}
	}
	if len(parsed.Parser.Args) != 0 || len(parsed.RemainingArgs) != 0 {
		fmt.Fprint(stderr, tokenHelpText())
		return 2
	}

	opts := derptun.TokenOptions{Days: parsed.SubCommandFlags.Days}
	if parsed.SubCommandFlags.Expires != "" {
		expires, err := parseTokenExpires(parsed.SubCommandFlags.Expires)
		if err != nil {
			fmt.Fprintln(stderr, err)
			fmt.Fprint(stderr, tokenHelpText())
			return 2
		}
		opts.Expires = expires
	}

	tokenValue, err := derptun.GenerateToken(opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, tokenValue)
	return 0
}

func parseTokenExpires(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	t, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --expires value %q; use RFC3339 or YYYY-MM-DD", value)
	}
	return t, nil
}

func tokenHelpText() string {
	return yargs.GenerateSubCommandHelp(tokenHelpConfig, "token", struct{}{}, tokenFlags{}, struct{}{})
}
