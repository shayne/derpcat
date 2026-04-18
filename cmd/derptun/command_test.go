package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/shayne/derphole/pkg/session"
)

func TestRunServePassesServerTokenAndTCP(t *testing.T) {
	oldServe := derptunServe
	defer func() { derptunServe = oldServe }()
	derptunServe = func(ctx context.Context, cfg session.DerptunServeConfig) error {
		if cfg.ServerToken != "dts1_test" {
			t.Fatalf("ServerToken = %q, want dts1_test", cfg.ServerToken)
		}
		if cfg.TargetAddr != "127.0.0.1:22" {
			t.Fatalf("TargetAddr = %q, want 127.0.0.1:22", cfg.TargetAddr)
		}
		if !cfg.UsePublicDERP {
			t.Fatal("UsePublicDERP = false, want true")
		}
		return nil
	}

	code := run([]string{"serve", "--token", "dts1_test", "--tcp", "127.0.0.1:22"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
}

func TestRunOpenPrintsBindAddress(t *testing.T) {
	oldOpen := derptunOpen
	defer func() { derptunOpen = oldOpen }()
	derptunOpen = func(ctx context.Context, cfg session.DerptunOpenConfig) error {
		if cfg.ClientToken != "dtc1_test" {
			t.Fatalf("ClientToken = %q, want dtc1_test", cfg.ClientToken)
		}
		if cfg.ListenAddr != "127.0.0.1:2222" {
			t.Fatalf("ListenAddr = %q, want 127.0.0.1:2222", cfg.ListenAddr)
		}
		cfg.BindAddrSink <- "127.0.0.1:2222"
		return nil
	}

	var stderr bytes.Buffer
	code := run([]string{"open", "--token", "dtc1_test", "--listen", "127.0.0.1:2222"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "listening on 127.0.0.1:2222") {
		t.Fatalf("stderr = %q, want bind address", stderr.String())
	}
}

func TestRunConnectRequiresStdio(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"connect", "--token", "dtc1_test"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--stdio") {
		t.Fatalf("stderr = %q, want stdio usage", stderr.String())
	}
}
