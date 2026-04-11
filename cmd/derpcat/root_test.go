package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/shayne/yargs"
)

func TestRunWithoutArgsShowsRootHelpAndSucceeds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	assertRootHelp(t, stderr.String())
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestRunRootHelpSucceeds(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"help"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0", code)
			}
			assertRootHelp(t, stderr.String())
			if got := stdout.String(); got != "" {
				t.Fatalf("stdout = %q, want empty", got)
			}
		})
	}
}

func TestRunRootHelpLLMSucceeds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help-llm"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if got, want := stderr.String(), yargs.GenerateGlobalHelpLLM(rootHelpConfig, rootGlobalFlags{}); got != want {
		t.Fatalf("stderr = %q, want exact LLM help %q", got, want)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestRunHelpListenShowsListenHelp(t *testing.T) {
	for _, args := range [][]string{{"help", "listen"}, {"listen", "--help"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0", code)
			}
			if got, want := stderr.String(), listenHelpText(); got != want {
				t.Fatalf("stderr = %q, want exact listen help %q", got, want)
			}
			if got := stdout.String(); got != "" {
				t.Fatalf("stdout = %q, want empty", got)
			}
		})
	}
}

func TestRunHelpSendShowsSendHelp(t *testing.T) {
	for _, args := range [][]string{{"help", "send"}, {"send", "--help"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0", code)
			}
			assertSendHelpText(t, stderr.String())
			if got := stdout.String(); got != "" {
				t.Fatalf("stdout = %q, want empty", got)
			}
		})
	}
}

func TestRunHelpShareShowsShareHelp(t *testing.T) {
	for _, args := range [][]string{{"help", "share"}, {"share", "--help"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0", code)
			}
			if got, want := stderr.String(), shareHelpText(); got != want {
				t.Fatalf("stderr = %q, want exact share help %q", got, want)
			}
			if got := stdout.String(); got != "" {
				t.Fatalf("stdout = %q, want empty", got)
			}
		})
	}
}

func TestRunHelpOpenShowsOpenHelp(t *testing.T) {
	for _, args := range [][]string{{"help", "open"}, {"open", "--help"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0", code)
			}
			if got, want := stderr.String(), openHelpText(); got != want {
				t.Fatalf("stderr = %q, want exact open help %q", got, want)
			}
			if got := stdout.String(); got != "" {
				t.Fatalf("stdout = %q, want empty", got)
			}
		})
	}
}

func TestRunVersionCommandSucceeds(t *testing.T) {
	origVersion, origCommit, origBuildDate := version, commit, buildDate
	t.Cleanup(func() {
		version, commit, buildDate = origVersion, origCommit, origBuildDate
	})

	version = "v0.0.1"
	commit = "abc1234"
	buildDate = "2026-03-29T12:00:00Z"

	var stdout, stderr bytes.Buffer
	code := run([]string{"version"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if got := stdout.String(); got != "v0.0.1\n" {
		t.Fatalf("stdout = %q, want version output", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestRunVersionHelpShowsSubcommandHelp(t *testing.T) {
	for _, args := range [][]string{{"version", "--help"}, {"help", "version"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0", code)
			}
			if got, want := stderr.String(), yargs.GenerateSubCommandHelp(rootHelpConfig, "version", rootGlobalFlags{}, struct{}{}, struct{}{}); got != want {
				t.Fatalf("stderr = %q, want exact version help %q", got, want)
			}
			if got := stdout.String(); got != "" {
				t.Fatalf("stdout = %q, want empty", got)
			}
		})
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bogus"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if got, want := stderr.String(), "unknown command: bogus\nRun 'derpcat --help' for usage\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestRunRejectsUnknownRootFlag(t *testing.T) {
	for _, args := range [][]string{{"--bogus"}, {"--version"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run() = %d, want 2", code)
			}
			if got := stderr.String(); !strings.HasPrefix(got, "unknown flag: "+args[0]+"\n") {
				t.Fatalf("stderr = %q, want unknown-flag prefix for %q", got, args[0])
			}
			assertRootHelp(t, stderr.String())
			if got := stdout.String(); got != "" {
				t.Fatalf("stdout = %q, want empty", got)
			}
		})
	}
}

func TestRunListenRejectsRemovedTCPFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"listen", "--tcp-connect", "127.0.0.1:3000"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if !strings.HasPrefix(stderr.String(), "unknown flag: --tcp-connect\n") {
		t.Fatalf("stderr = %q, want removed-flag parse error", stderr.String())
	}
}

func TestRunSendRejectsRemovedTCPFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"send", "token-value", "--tcp-listen", "127.0.0.1:8080"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if !strings.HasPrefix(stderr.String(), "unknown flag: --tcp-listen\n") {
		t.Fatalf("stderr = %q, want removed-flag parse error", stderr.String())
	}
}

func TestRunRejectsConflictingVerbosityFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-v", "--quiet", "listen"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if got, want := stderr.String(), "only one of --verbose, --quiet, or --silent may be set\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestRunListenDispatchesToListenBehavior(t *testing.T) {
	listenerStdoutBuf := &lockedBuffer{}
	listenerStderrBuf := &lockedBuffer{}
	listenerDone := make(chan int, 1)
	go func() {
		listenerDone <- run([]string{"listen"}, nil, listenerStdoutBuf, listenerStderrBuf)
	}()

	issuedToken := waitForIssuedToken(t, listenerStderrBuf)

	var senderStdout, senderStderr bytes.Buffer
	code := run([]string{"send", issuedToken, "--force-relay"}, strings.NewReader("hello through root"), &senderStdout, &senderStderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0, stderr=%q", code, senderStderr.String())
	}
	if got := senderStdout.String(); got != "" {
		t.Fatalf("sender stdout = %q, want empty", got)
	}

	select {
	case code := <-listenerDone:
		if code != 0 {
			t.Fatalf("run() = %d, want 0, stderr=%q", code, listenerStderrBuf.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listen subcommand did not complete after sender finished")
	}

	if got := listenerStdoutBuf.String(); got != "hello through root" {
		t.Fatalf("listener stdout = %q, want payload", got)
	}
	if !strings.Contains(listenerStderrBuf.String(), issuedToken+"\n") {
		t.Fatalf("listener stderr = %q, want issued token", listenerStderrBuf.String())
	}
}

func TestRunVerbosityFlagsBeforeListen(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPrefix string
	}{
		{name: "default", args: []string{"listen"}, wantPrefix: ""},
		{name: "quiet", args: []string{"-q", "listen"}, wantPrefix: ""},
		{name: "silent", args: []string{"-s", "listen"}, wantPrefix: ""},
		{name: "verbose", args: []string{"-v", "listen"}, wantPrefix: "waiting-for-claim\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			listenerStdoutBuf := &lockedBuffer{}
			listenerStderrBuf := &lockedBuffer{}
			listenerDone := make(chan int, 1)
			go func() {
				listenerDone <- run(tc.args, nil, listenerStdoutBuf, listenerStderrBuf)
			}()

			issuedToken := waitForIssuedToken(t, listenerStderrBuf)

			var senderStdout, senderStderr bytes.Buffer
			code := run([]string{"send", issuedToken, "--force-relay"}, strings.NewReader("payload"), &senderStdout, &senderStderr)
			if code != 0 {
				t.Fatalf("send run() = %d, want 0, stderr=%q", code, senderStderr.String())
			}

			select {
			case code := <-listenerDone:
				if code != 0 {
					t.Fatalf("listen run() = %d, want 0, stderr=%q", code, listenerStderrBuf.String())
				}
			case <-time.After(2 * time.Second):
				t.Fatal("listen subcommand did not complete after sender finished")
			}

			if got := listenerStderrBuf.String(); !strings.HasPrefix(got, tc.wantPrefix) {
				t.Fatalf("stderr = %q, want prefix %q", got, tc.wantPrefix)
			}
			if !strings.Contains(listenerStderrBuf.String(), issuedToken+"\n") {
				t.Fatalf("stderr = %q, want token on stderr", listenerStderrBuf.String())
			}
		})
	}
}

func TestRunVerbosityFlagsBeforeSend(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPrefix string
	}{
		{name: "default", args: []string{"send"}, wantPrefix: ""},
		{name: "quiet", args: []string{"-q", "send"}, wantPrefix: ""},
		{name: "silent", args: []string{"-s", "send"}, wantPrefix: ""},
		{name: "verbose", args: []string{"-v", "send"}, wantPrefix: "probing-direct\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			listenerStdoutBuf := &lockedBuffer{}
			listenerStderrBuf := &lockedBuffer{}
			listenerDone := make(chan int, 1)
			go func() {
				listenerDone <- run([]string{"listen"}, nil, listenerStdoutBuf, listenerStderrBuf)
			}()

			issuedToken := waitForIssuedToken(t, listenerStderrBuf)

			senderArgs := append(append([]string(nil), tc.args...), issuedToken, "--force-relay")
			var senderStdout, senderStderr bytes.Buffer
			code := run(senderArgs, strings.NewReader("payload"), &senderStdout, &senderStderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0, stderr=%q", code, senderStderr.String())
			}

			select {
			case code := <-listenerDone:
				if code != 0 {
					t.Fatalf("listen run() = %d, want 0, stderr=%q", code, listenerStderrBuf.String())
				}
			case <-time.After(2 * time.Second):
				t.Fatal("listen subcommand did not complete after sender finished")
			}

			if tc.wantPrefix == "" {
				if got := senderStderr.String(); got != "" {
					t.Fatalf("stderr = %q, want empty", got)
				}
				return
			}
			if got := senderStderr.String(); !strings.HasPrefix(got, tc.wantPrefix) {
				t.Fatalf("stderr = %q, want prefix %q", got, tc.wantPrefix)
			}
		})
	}
}

func TestRunSendDispatchesUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"send"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	assertSendHelpText(t, stderr.String())
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func assertRootHelp(t *testing.T, got string) {
	t.Helper()
	for _, want := range []string{
		"GLOBAL OPTIONS:",
		"-v, --verbose",
		"-q, --quiet",
		"-s, --silent",
		"EXAMPLES:",
		"derpcat listen",
		"derpcat send <token>",
		"derpcat share 127.0.0.1:3000",
		"derpcat open <token>",
		"derpcat version",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr = %q, want root help mentioning %q", got, want)
		}
	}
}
