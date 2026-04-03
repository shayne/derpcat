package quicpath

import "testing"

func TestDefaultQUICConfigUsesConservativeInitialPacketSize(t *testing.T) {
	cfg := DefaultQUICConfig()
	if got, want := cfg.InitialPacketSize, uint16(1200); got != want {
		t.Fatalf("InitialPacketSize = %d, want %d", got, want)
	}
}

func TestDefaultQUICConfigEnablesQlogTracerFromEnv(t *testing.T) {
	t.Setenv("DERPCAT_QLOG_DIR", t.TempDir())

	cfg := DefaultQUICConfig()
	if cfg.Tracer == nil {
		t.Fatal("Tracer = nil, want qlog tracer from DERPCAT_QLOG_DIR")
	}
}

func TestDefaultQUICConfigEnablesMetricsTracerFromEnv(t *testing.T) {
	t.Setenv("DERPCAT_QUIC_METRICS_DIR", t.TempDir())

	cfg := DefaultQUICConfig()
	if cfg.Tracer == nil {
		t.Fatal("Tracer = nil, want metrics tracer from DERPCAT_QUIC_METRICS_DIR")
	}
}
