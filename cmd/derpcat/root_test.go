package main

import (
	"bytes"
	"testing"
)

func TestRunRejectsMissingSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if got := stderr.String(); got != "usage: derpcat <listen|send> [flags]\n" {
		t.Fatalf("stderr = %q, want exact usage text", got)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bogus"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if got := stderr.String(); got != "unknown subcommand \"bogus\"\n" {
		t.Fatalf("stderr = %q, want exact unknown subcommand message", got)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestRunRootHelpSucceeds(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}} {
		t.Run(args[0], func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0", code)
			}
			if got := stderr.String(); got != "usage: derpcat <listen|send> [flags]\n" {
				t.Fatalf("stderr = %q, want exact root usage", got)
			}
			if got := stdout.String(); got != "" {
				t.Fatalf("stdout = %q, want empty", got)
			}
		})
	}
}

func TestRunListenDispatchesToListenBehavior(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"listen"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if stdout.String() == "" {
		t.Fatal("stdout empty, want token")
	}
	if got := stderr.String(); got != "waiting-for-claim\n" {
		t.Fatalf("stderr = %q, want status text", got)
	}
}

func TestRunVerbosityFlagsBeforeListen(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
	}{
		{name: "quiet", args: []string{"-q", "listen"}, wantCode: 0, wantStderr: ""},
		{name: "silent", args: []string{"-s", "listen"}, wantCode: 0, wantStderr: ""},
		{name: "verbose", args: []string{"-v", "listen"}, wantCode: 0, wantStderr: "waiting-for-claim\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, nil, &stdout, &stderr)
			if code != tc.wantCode {
				t.Fatalf("run() = %d, want %d", code, tc.wantCode)
			}
			if got := stderr.String(); got != tc.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, tc.wantStderr)
			}
			if stdout.String() == "" {
				t.Fatal("stdout empty, want token")
			}
		})
	}
}

func TestRunListenHelpSucceeds(t *testing.T) {
	for _, args := range [][]string{{"listen", "-h"}, {"listen", "--help"}} {
		t.Run(args[1], func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0", code)
			}
			if got := stderr.String(); got != "usage: derpcat listen [--print-token-only]\n" {
				t.Fatalf("stderr = %q, want exact listen usage", got)
			}
			if got := stdout.String(); got != "" {
				t.Fatalf("stdout = %q, want empty", got)
			}
		})
	}
}

func TestRunVerbosityFlagsBeforeSend(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
	}{
		{name: "quiet usage", args: []string{"-q", "send"}, wantCode: 2, wantStderr: "usage: derpcat send <token> [flags...]\n"},
		{name: "silent help", args: []string{"-s", "send", "token-value", "-h"}, wantCode: 0, wantStderr: "usage: derpcat send <token> [flags...]\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, nil, &stdout, &stderr)
			if code != tc.wantCode {
				t.Fatalf("run() = %d, want %d", code, tc.wantCode)
			}
			if got := stderr.String(); got != tc.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, tc.wantStderr)
			}
			if tc.wantCode == 0 && stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestRunSendDispatchesToSendUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"send"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if got := stderr.String(); got != "usage: derpcat send <token> [flags...]\n" {
		t.Fatalf("stderr = %q, want usage text", got)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestRunSendGrammarAllowsTokenBeforeFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"send", "token-value", "-h"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if got := stderr.String(); got != "usage: derpcat send <token> [flags...]\n" {
		t.Fatalf("stderr = %q, want exact send usage", got)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}
