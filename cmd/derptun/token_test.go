package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunTokenPrintsDerptunToken(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"token", "--days", "7"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "dt1_") {
		t.Fatalf("stdout = %q, want derptun token", stdout.String())
	}
}
