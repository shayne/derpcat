package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/shayne/derpcat/pkg/telemetry"
	"github.com/shayne/yargs"
)

type sshInviteFlags struct{}
type sshInviteArgs struct {
	PublicKey string `pos:"0?" help:"Path to a public key file"`
}

type sshAcceptFlags struct{}
type sshAcceptArgs struct {
	Code string `pos:"0?" help:"Optional receive code"`
}

var sshHelpConfig = yargs.HelpConfig{
	Command: yargs.CommandInfo{
		Name:        "derphole",
		Description: "Exchange SSH access invites with derphole.",
		Examples: []string{
			"derphole ssh invite ~/.ssh/id_ed25519.pub",
			"derphole ssh accept",
		},
	},
	SubCommands: map[string]yargs.SubCommandInfo{
		"ssh": {
			Name:        "ssh",
			Description: "SSH invite and accept workflows.",
			Usage:       "<invite|accept>",
		},
		"invite": {
			Name:        "invite",
			Description: "Add a public key to authorized_keys on the receiving host.",
			Usage:       "[public-key]",
		},
		"accept": {
			Name:        "accept",
			Description: "Accept an SSH key invite and update authorized_keys.",
			Usage:       "[code]",
		},
	},
}

func runSSH(args []string, level telemetry.Level, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = level
	_ = stdin
	_ = stdout

	if len(args) == 0 || isRootHelpRequest(args) {
		fmt.Fprint(stderr, sshHelpText())
		return 0
	}

	switch canonicalSSHCommand(args[0]) {
	case "invite":
		return runSSHInvite(args[1:], stderr)
	case "accept":
		return runSSHAccept(args[1:], stderr)
	default:
		fmt.Fprintf(stderr, "unknown ssh command: %s\n", args[0])
		fmt.Fprint(stderr, sshHelpText())
		return 2
	}
}

func runSSHInvite(args []string, stderr io.Writer) int {
	parsed, err := yargs.ParseWithCommandAndHelp[struct{}, sshInviteFlags, sshInviteArgs](append([]string{"invite"}, args...), sshHelpConfig)
	if err != nil {
		switch {
		case errors.Is(err, yargs.ErrHelp), errors.Is(err, yargs.ErrSubCommandHelp), errors.Is(err, yargs.ErrHelpLLM):
			if parsed != nil && parsed.HelpText != "" {
				fmt.Fprint(stderr, parsed.HelpText)
			} else {
				fmt.Fprint(stderr, sshInviteHelpText())
			}
			return 0
		default:
			fmt.Fprintln(stderr, err)
			fmt.Fprint(stderr, sshInviteHelpText())
			return 2
		}
	}

	if len(parsed.Parser.Args) > 1 || len(parsed.RemainingArgs) != 0 {
		fmt.Fprint(stderr, sshInviteHelpText())
		return 2
	}

	fmt.Fprintln(stderr, "derphole ssh invite is not implemented yet")
	return 1
}

func runSSHAccept(args []string, stderr io.Writer) int {
	parsed, err := yargs.ParseWithCommandAndHelp[struct{}, sshAcceptFlags, sshAcceptArgs](append([]string{"accept"}, args...), sshHelpConfig)
	if err != nil {
		switch {
		case errors.Is(err, yargs.ErrHelp), errors.Is(err, yargs.ErrSubCommandHelp), errors.Is(err, yargs.ErrHelpLLM):
			if parsed != nil && parsed.HelpText != "" {
				fmt.Fprint(stderr, parsed.HelpText)
			} else {
				fmt.Fprint(stderr, sshAcceptHelpText())
			}
			return 0
		default:
			fmt.Fprintln(stderr, err)
			fmt.Fprint(stderr, sshAcceptHelpText())
			return 2
		}
	}

	if len(parsed.Parser.Args) > 1 || len(parsed.RemainingArgs) != 0 {
		fmt.Fprint(stderr, sshAcceptHelpText())
		return 2
	}

	fmt.Fprintln(stderr, "derphole ssh accept is not implemented yet")
	return 1
}

func canonicalSSHCommand(name string) string {
	return name
}

func sshHelpText() string {
	return yargs.GenerateSubCommandHelp(sshHelpConfig, "ssh", struct{}{}, struct{}{}, struct{}{})
}

func sshInviteHelpText() string {
	return yargs.GenerateSubCommandHelp(sshHelpConfig, "invite", struct{}{}, sshInviteFlags{}, sshInviteArgs{})
}

func sshAcceptHelpText() string {
	return yargs.GenerateSubCommandHelp(sshHelpConfig, "accept", struct{}{}, sshAcceptFlags{}, sshAcceptArgs{})
}
