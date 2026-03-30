package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunShareHelpShowsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"share", "--help"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	for _, want := range []string{
		"Share a local TCP service until Ctrl-C.",
		"derpcat share",
		"127.0.0.1:3000",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}
