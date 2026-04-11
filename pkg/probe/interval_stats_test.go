package probe

import (
	"testing"
	"time"
)

func TestIntervalStatsTracksPeakThroughputFromMeasuredDeltas(t *testing.T) {
	var stats intervalStats
	started := time.Unix(100, 0)

	stats.Observe(started, 0)
	stats.Observe(started.Add(100*time.Millisecond), 1<<20)
	stats.Observe(started.Add(150*time.Millisecond), 2<<20)

	if got, want := stats.PeakMbps(), float64(1<<20)*8/0.05/1_000_000; !almostEqual(got, want) {
		t.Fatalf("PeakMbps() = %f, want %f", got, want)
	}
}

func TestIntervalStatsIgnoresNonMonotonicOrZeroDeltaSamples(t *testing.T) {
	var stats intervalStats
	started := time.Unix(200, 0)

	stats.Observe(started, 0)
	stats.Observe(started.Add(100*time.Millisecond), 1024)
	peak := stats.PeakMbps()

	stats.Observe(started.Add(100*time.Millisecond), 1024)
	stats.Observe(started.Add(90*time.Millisecond), 2048)
	stats.Observe(started.Add(150*time.Millisecond), 512)

	if got := stats.PeakMbps(); !almostEqual(got, peak) {
		t.Fatalf("PeakMbps() = %f, want %f", got, peak)
	}
}
