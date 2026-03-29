package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("DERPCAT_TEST_LOCAL_RELAY", "1")
	os.Exit(m.Run())
}
