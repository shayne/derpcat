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

func TestRunSendDispatchesToSendUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"send"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if got := stderr.String(); got != "usage: derpcat send <token>\n" {
		t.Fatalf("stderr = %q, want usage text", got)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}
