package telemetry

import (
	"bytes"
	"testing"
)

func TestEmitterQuietSuppressesStatus(t *testing.T) {
	var buf bytes.Buffer
	e := New(&buf, LevelQuiet)
	e.Status("waiting-for-claim")
	if got := buf.String(); got != "" {
		t.Fatalf("Status output = %q, want empty", got)
	}
}

func TestEmitterVerbosePrintsStatus(t *testing.T) {
	var buf bytes.Buffer
	e := New(&buf, LevelVerbose)
	e.Status("probing-direct")
	if got := buf.String(); got == "" {
		t.Fatal("Status output empty, want text")
	}
}
