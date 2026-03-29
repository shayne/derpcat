package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRejectsMissingSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if got := stderr.String(); !strings.Contains(got, "usage: derpcat <listen|send> [flags]") {
		t.Fatalf("stderr = %q, want usage text", got)
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
	if got := stderr.String(); !strings.Contains(got, `unknown subcommand "bogus"`) {
		t.Fatalf("stderr = %q, want unknown subcommand message", got)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}
