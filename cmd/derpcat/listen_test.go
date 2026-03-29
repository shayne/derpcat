package main

import (
	"bytes"
	"testing"
)

func TestListenPrintTokenOnlyTargetsStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"listen", "--print-token-only"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if stdout.String() == "" {
		t.Fatal("stdout empty, want token")
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
