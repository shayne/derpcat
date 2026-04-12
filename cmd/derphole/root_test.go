package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunWithoutArgsShowsRootHelpAndSucceeds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if got, want := stderr.String(), rootHelpText(); got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestRunHelpReceiveShowsReceiveHelp(t *testing.T) {
	for _, args := range [][]string{{"help", "receive"}, {"receive", "--help"}, {"rx", "--help"}, {"recv", "--help"}, {"recieve", "--help"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0", code)
			}
			if got, want := stderr.String(), receiveHelpText(); got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
		})
	}
}
