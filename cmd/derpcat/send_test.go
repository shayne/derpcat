package main

import (
	"bytes"
	"testing"
)

func TestSendRejectsMissingTokenArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runSend([]string{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runSend() = %d, want 2", code)
	}
	if got := stderr.String(); got != "usage: derpcat send <token>\n" {
		t.Fatalf("stderr = %q, want usage text", got)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}
