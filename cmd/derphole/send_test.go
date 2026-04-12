package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelpSendShowsSendHelp(t *testing.T) {
	for _, args := range [][]string{{"help", "send"}, {"send", "--help"}, {"tx", "--help"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run() = %d, want 0", code)
			}
			if got, want := stderr.String(), sendHelpText(); got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
		})
	}
}
