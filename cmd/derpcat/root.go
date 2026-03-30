package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/shayne/derpcat/pkg/telemetry"
	"github.com/shayne/yargs"
)

type rootGlobalFlags struct {
	Verbose bool `flag:"verbose" short:"v" help:"Show relay status updates"`
	Quiet   bool `flag:"quiet" short:"q" help:"Reduce relay status output"`
	Silent  bool `flag:"silent" short:"s" help:"Suppress relay status output"`
}

var rootRegistry = yargs.Registry{
	Command: yargs.CommandInfo{
		Name:        "derpcat",
		Description: "Move one-shot streams or shared TCP services over public DERP with direct UDP promotion when available.",
		Examples: []string{
			"derpcat listen",
			"derpcat send <token>",
			"derpcat share 127.0.0.1:3000",
			"derpcat open <token>",
			"derpcat version",
		},
	},
	SubCommands: map[string]yargs.CommandSpec{
		"listen": {
			Info: yargs.SubCommandInfo{
				Name:        "listen",
				Description: "Listen for a claim token and stream payloads to stdout.",
			},
		},
		"send": {
			Info: yargs.SubCommandInfo{
				Name:        "send",
				Description: "Claim a token and stream stdin to a relay listener.",
			},
		},
		"share": {
			Info: yargs.SubCommandInfo{
				Name:        "share",
				Description: "Share a local TCP service until Ctrl-C.",
			},
		},
		"open": {
			Info: yargs.SubCommandInfo{
				Name:        "open",
				Description: "Open a shared service locally until Ctrl-C.",
			},
		},
		"version": {
			Info: yargs.SubCommandInfo{
				Name:        "version",
				Description: "Print the derpcat version.",
			},
		},
	},
}

var rootHelpConfig = rootRegistry.HelpConfig()

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	parsed, err := yargs.ParseKnownFlags[rootGlobalFlags](args, yargs.KnownFlagsOptions{})
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprint(stderr, yargs.GenerateGlobalHelp(rootHelpConfig, rootGlobalFlags{}))
		return 2
	}

	level, err := rootTelemetryLevel(parsed.Flags)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	remaining := yargs.ApplyAliases(parsed.RemainingArgs, rootHelpConfig)
	if len(remaining) == 0 || isRootHelpRequest(remaining) {
		fmt.Fprint(stderr, yargs.GenerateGlobalHelp(rootHelpConfig, rootGlobalFlags{}))
		return 0
	}
	if isRootHelpLLMRequest(remaining) {
		fmt.Fprint(stderr, yargs.GenerateGlobalHelpLLM(rootHelpConfig, rootGlobalFlags{}))
		return 0
	}
	if strings.HasPrefix(remaining[0], "-") {
		fmt.Fprintf(stderr, "unknown flag: %s\n", remaining[0])
		fmt.Fprint(stderr, yargs.GenerateGlobalHelp(rootHelpConfig, rootGlobalFlags{}))
		return 2
	}
	if remaining[0] == "help" {
		return runHelpCommand(remaining[1:], stderr)
	}

	switch remaining[0] {
	case "listen":
		return runListen(remaining[1:], level, stdout, stderr)
	case "send":
		return runSend(remaining[1:], level, stdin, stdout, stderr)
	case "share":
		return runShare(remaining[1:], level, stdout, stderr)
	case "open":
		return runOpen(remaining[1:], level, stdout, stderr)
	case "version":
		return runVersion(remaining[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\nRun 'derpcat --help' for usage\n", remaining[0])
		return 2
	}
}

func isRootHelpRequest(args []string) bool {
	return len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help")
}

func isRootHelpLLMRequest(args []string) bool {
	return len(args) == 1 && args[0] == "--help-llm"
}

func rootTelemetryLevel(flags rootGlobalFlags) (telemetry.Level, error) {
	count := 0
	level := telemetry.LevelDefault

	if flags.Verbose {
		count++
		level = telemetry.LevelVerbose
	}
	if flags.Quiet {
		count++
		level = telemetry.LevelQuiet
	}
	if flags.Silent {
		count++
		level = telemetry.LevelSilent
	}
	if count > 1 {
		return telemetry.LevelDefault, fmt.Errorf("only one of --verbose, --quiet, or --silent may be set")
	}
	return level, nil
}

func runHelpCommand(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, yargs.GenerateGlobalHelp(rootHelpConfig, rootGlobalFlags{}))
		return 0
	}
	if len(args) > 2 || (len(args) == 2 && args[1] != "--help" && args[1] != "--help-llm") {
		fmt.Fprint(stderr, yargs.GenerateGlobalHelp(rootHelpConfig, rootGlobalFlags{}))
		return 2
	}

	llm := len(args) == 2 && args[1] == "--help-llm"
	switch args[0] {
	case "listen":
		if llm {
			fmt.Fprint(stderr, listenHelpLLMText())
		} else {
			fmt.Fprint(stderr, listenHelpText())
		}
		return 0
	case "send":
		if llm {
			fmt.Fprint(stderr, sendHelpLLMText())
		} else {
			fmt.Fprint(stderr, sendHelpText())
		}
		return 0
	case "share":
		if llm {
			fmt.Fprint(stderr, shareHelpLLMText())
		} else {
			fmt.Fprint(stderr, shareHelpText())
		}
		return 0
	case "open":
		if llm {
			fmt.Fprint(stderr, openHelpLLMText())
		} else {
			fmt.Fprint(stderr, openHelpText())
		}
		return 0
	case "version":
		if llm {
			fmt.Fprint(stderr, versionHelpLLMText())
		} else {
			fmt.Fprint(stderr, versionHelpText())
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\nRun 'derpcat --help' for usage\n", args[0])
		return 2
	}
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	parsed, err := yargs.ParseWithCommandAndHelp[rootGlobalFlags, struct{}, struct{}](append([]string{"version"}, args...), rootHelpConfig)
	if err != nil {
		switch {
		case parsed != nil && (err == yargs.ErrHelp || err == yargs.ErrSubCommandHelp || err == yargs.ErrHelpLLM):
			fmt.Fprint(stderr, parsed.HelpText)
			return 0
		default:
			fmt.Fprintln(stderr, err)
			fmt.Fprint(stderr, versionHelpText())
			return 2
		}
	}

	if len(parsed.Parser.Args) != 0 || len(parsed.RemainingArgs) != 0 {
		fmt.Fprint(stderr, versionHelpText())
		return 2
	}

	fmt.Fprintln(stdout, versionString())
	return 0
}

func versionHelpText() string {
	return yargs.GenerateSubCommandHelp(rootHelpConfig, "version", rootGlobalFlags{}, struct{}{}, struct{}{})
}

func versionHelpLLMText() string {
	return yargs.GenerateSubCommandHelpLLMFromConfig(rootHelpConfig, "version", rootGlobalFlags{})
}
