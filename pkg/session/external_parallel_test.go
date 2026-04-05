package session

import "testing"

func TestExternalParallelAutoBootstrapReady(t *testing.T) {
	if externalParallelAutoBootstrapReady(externalHandoffSpoolSnapshot{AckedWatermark: externalParallelAutoBootstrapBytes - 1}) {
		t.Fatal("bootstrap gate should stay closed below threshold")
	}
	if !externalParallelAutoBootstrapReady(externalHandoffSpoolSnapshot{AckedWatermark: externalParallelAutoBootstrapBytes}) {
		t.Fatal("bootstrap gate should open at threshold")
	}
}
