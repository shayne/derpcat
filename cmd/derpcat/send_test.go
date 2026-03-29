package main

import (
	"bytes"
	"testing"
)

func TestSendRejectsMissingTokenArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"send"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if got := stderr.String(); got != "send not implemented\n" {
		t.Fatalf("stderr = %q, want placeholder message", got)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}
