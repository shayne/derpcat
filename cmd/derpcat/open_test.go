package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shayne/derpcat/pkg/telemetry"
)

func TestRunOpenHelpShowsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"open", "--help"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	for _, want := range []string{
		"Open a shared service locally until Ctrl-C.",
		"derpcat open",
		"127.0.0.1:8080",
		"-P",
		"--parallel",
		"auto",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestOpenHelpMentionsParallelFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runOpen([]string{"--help"}, telemetry.LevelDefault, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runOpen() = %d, want 0", code)
	}
	for _, want := range []string{"-P", "--parallel", "auto"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want help mentioning %q", stderr.String(), want)
		}
	}
}

func TestOpenReportsRelayThenDirectWhenTransportUpgrades(t *testing.T) {
	_, openStderr := runUpgradingExternalShareAndOpen(t)

	assertStatusLinesPrefix(t, openStderr, "open stderr", "probing-direct", "connected-relay", "connected-direct")
}
