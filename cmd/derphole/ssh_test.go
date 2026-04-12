package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelpSSHInviteShowsSSHHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"ssh", "invite", "--help"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Add a public key to authorized_keys") {
		t.Fatalf("stderr = %q, want ssh invite help", stderr.String())
	}
}

func TestRunHelpSSHAcceptShowsSSHAcceptHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"ssh", "accept", "--help"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Accept an SSH key invite") {
		t.Fatalf("stderr = %q, want ssh accept help", stderr.String())
	}
}
