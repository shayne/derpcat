package main

import (
	"io"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("DERPCAT_TEST_LOCAL_RELAY", "1")
	os.Exit(m.Run())
}

func TestRunMainWritesCPUProfileFromEnv(t *testing.T) {
	path := t.TempDir() + "/cpu.pprof"
	t.Setenv("DERPCAT_CPU_PROFILE", path)

	if code := runMain([]string{"version"}, nil, io.Discard, io.Discard); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat(%q) error = %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("stat(%q).Size() = 0, want non-empty profile", path)
	}
}

func TestRunMainWritesBlockProfileFromEnv(t *testing.T) {
	path := t.TempDir() + "/block.pprof"
	t.Setenv("DERPCAT_BLOCK_PROFILE", path)

	if code := runMain([]string{"version"}, nil, io.Discard, io.Discard); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat(%q) error = %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("stat(%q).Size() = 0, want non-empty profile", path)
	}
}
