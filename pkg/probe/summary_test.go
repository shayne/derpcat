package probe

import "testing"

func TestSummarizeRunsComputesWallPeakAverageAndFailures(t *testing.T) {
	runs := []RunReport{
		{
			GoodputMbps:     100,
			PeakGoodputMbps: 150,
			DurationMS:      10,
			FirstByteMS:     5,
			successSet:      true,
			Success:         true,
		},
		{
			GoodputMbps:     150,
			PeakGoodputMbps: 175,
			DurationMS:      15,
			FirstByteMS:     7,
		},
		{
			GoodputMbps:     200,
			PeakGoodputMbps: 250,
			DurationMS:      20,
			FirstByteMS:     15,
			successSet:      true,
			Success:         true,
		},
		{
			GoodputMbps:     50,
			PeakGoodputMbps: 75,
			DurationMS:      40,
			FirstByteMS:     9,
			successSet:      true,
			Success:         false,
		},
	}

	summary := SummarizeRuns(runs)

	if summary.RunCount != 4 {
		t.Fatalf("RunCount = %d, want 4", summary.RunCount)
	}
	if summary.SuccessCount != 3 {
		t.Fatalf("SuccessCount = %d, want 3", summary.SuccessCount)
	}
	if summary.FailureCount != 1 {
		t.Fatalf("FailureCount = %d, want 1", summary.FailureCount)
	}
	if got, want := summary.FailureRate, 0.25; !almostEqual(got, want) {
		t.Fatalf("FailureRate = %f, want %f", got, want)
	}
	if got, want := summary.AverageGoodputMbps, (100.0+150.0+200.0+50.0)/4.0; !almostEqual(got, want) {
		t.Fatalf("AverageGoodputMbps = %f, want %f", got, want)
	}
	if got, want := summary.PeakGoodputMbps, 250.0; got != want {
		t.Fatalf("PeakGoodputMbps = %f, want %f", got, want)
	}
	if got, want := summary.AverageWallTimeMS, (10.0+15.0+20.0+40.0)/4.0; !almostEqual(got, want) {
		t.Fatalf("AverageWallTimeMS = %f, want %f", got, want)
	}
	if summary.FirstByteCount != 4 {
		t.Fatalf("FirstByteCount = %d, want 4", summary.FirstByteCount)
	}
	if got, want := summary.AverageFirstByteMS, (5.0+7.0+15.0+9.0)/4.0; !almostEqual(got, want) {
		t.Fatalf("AverageFirstByteMS = %f, want %f", got, want)
	}
	if summary.PeakFirstByteMS != 15 {
		t.Fatalf("PeakFirstByteMS = %d, want 15", summary.PeakFirstByteMS)
	}
	if !summary.HasFirstByteMetrics {
		t.Fatalf("HasFirstByteMetrics = false, want true")
	}
}

func TestSummarizeRunsTreatsLegacyReportsAsSuccessful(t *testing.T) {
	runs := []RunReport{
		{
			GoodputMbps: 10,
			DurationMS:  10,
			FirstByteMS: 5,
			Success:     false,
		},
	}

	summary := SummarizeRuns(runs)

	if summary.SuccessCount != 1 {
		t.Fatalf("SuccessCount = %d, want 1", summary.SuccessCount)
	}
	if summary.FailureCount != 0 {
		t.Fatalf("FailureCount = %d, want 0", summary.FailureCount)
	}
}

func TestCompareSummariesRejectsWallTimeAndFailureRegression(t *testing.T) {
	base := SeriesSummary{
		RunCount:           10,
		SuccessCount:       10,
		FailureCount:       0,
		FailureRate:        0,
		AverageWallTimeMS:  100,
		AverageGoodputMbps: 250,
		PeakGoodputMbps:    300,
	}
	head := SeriesSummary{
		RunCount:           10,
		SuccessCount:       8,
		FailureCount:       2,
		FailureRate:        0.2,
		AverageWallTimeMS:  120,
		AverageGoodputMbps: 240,
		PeakGoodputMbps:    290,
	}

	result := CompareSummaries(base, head)

	if !result.IsRegression {
		t.Fatalf("IsRegression = false, want true")
	}
	if !result.WallTimeRegression {
		t.Fatalf("WallTimeRegression = false, want true")
	}
	if !result.FailureRateRegression {
		t.Fatalf("FailureRateRegression = false, want true")
	}
	if got, want := result.WallTimeDeltaMS, 20.0; !almostEqual(got, want) {
		t.Fatalf("WallTimeDeltaMS = %f, want %f", got, want)
	}
	if got, want := result.FailureRateDelta, 0.2; !almostEqual(got, want) {
		t.Fatalf("FailureRateDelta = %f, want %f", got, want)
	}
	if len(result.Reasons) != 2 {
		t.Fatalf("Reasons = %#v, want 2 entries", result.Reasons)
	}
}

func almostEqual(got, want float64) bool {
	const epsilon = 1e-9
	if got > want {
		return got-want < epsilon
	}
	return want-got < epsilon
}
