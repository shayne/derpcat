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

func TestListenWithoutFlagsPrintsStatusAndToken(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runListen([]string{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runListen() = %d, want 0", code)
	}
	if stdout.String() == "" {
		t.Fatal("stdout empty, want token")
	}
	if got := stderr.String(); got != "waiting-for-claim\n" {
		t.Fatalf("stderr = %q, want status text", got)
	}
}
