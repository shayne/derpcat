package main

import (
	"context"
	"strings"
	"testing"
)

func TestParsePromotionSummaryReadsBenchmarkFooter(t *testing.T) {
	raw := strings.Join([]string{
		"target=ktzlxc",
		"size_mib=1024",
		"benchmark-host=ktzlxc",
		"benchmark-direction=forward",
		"benchmark-size-bytes=1073741824",
		"benchmark-total-duration-ms=4210",
		"benchmark-goodput-mbps=2039.1",
		"benchmark-peak-goodput-mbps=2210.4",
		"benchmark-first-byte-ms=18",
		"benchmark-success=true",
	}, "\n")

	got, err := parsePromotionSummary([]byte(raw))
	if err != nil {
		t.Fatalf("parsePromotionSummary() error = %v", err)
	}
	if got.Host != "ktzlxc" {
		t.Fatalf("Host = %q, want %q", got.Host, "ktzlxc")
	}
	if got.Direction != "forward" {
		t.Fatalf("Direction = %q, want %q", got.Direction, "forward")
	}
	if got.PeakGoodputMbps != 2210.4 {
		t.Fatalf("PeakGoodputMbps = %.1f, want 2210.4", got.PeakGoodputMbps)
	}
	if got.Success == nil || !*got.Success {
		t.Fatalf("Success = %#v, want true", got.Success)
	}
}

func TestRunMatrixIteratesAllHostsDirectionsAndIterations(t *testing.T) {
	prev := runMatrixCommand
	defer func() { runMatrixCommand = prev }()

	var calls []string
	runMatrixCommand = func(_ context.Context, script string, host string, sizeMiB int) ([]byte, error) {
		calls = append(calls, script+":"+host)
		return []byte(strings.Join([]string{
			"benchmark-host=" + host,
			"benchmark-direction=forward",
			"benchmark-size-bytes=1073741824",
			"benchmark-total-duration-ms=5000",
			"benchmark-goodput-mbps=1700.0",
			"benchmark-peak-goodput-mbps=2000.0",
			"benchmark-success=true",
		}, "\n")), nil
	}

	_, err := runMatrix(context.Background(), matrixConfig{
		Hosts:      []string{"ktzlxc", "canlxc"},
		Iterations: 2,
		SizeMiB:    1024,
	})
	if err != nil {
		t.Fatalf("runMatrix() error = %v", err)
	}
	if got, want := len(calls), 8; got != want {
		t.Fatalf("len(calls) = %d, want %d", got, want)
	}
}
