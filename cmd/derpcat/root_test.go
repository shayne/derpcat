package main

import (
	"bytes"
	"testing"
)

func TestRunRejectsMissingSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, nil, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run() = 0, want non-zero")
	}
	if got := stderr.String(); got == "" {
		t.Fatalf("stderr = %q, want usage text", got)
	}
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bogus"}, nil, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run() = 0, want non-zero")
	}
}
