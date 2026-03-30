package transport

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestManagerDiscoversDirectPathAfterLateCandidateUpdate(t *testing.T) {
	t.Helper()

	relay := newFakePacketConn(&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)})
	direct := newFakePacketConn(&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)})
	refresh := newFakeSignal()
	callMeMaybe := newFakeSignal()
	candidate := &net.UDPAddr{IP: net.IPv4(100, 64, 0, 2), Port: 12345}

	mgr := NewManager(ManagerConfig{
		RelayConn:              relay,
		DirectConn:             direct,
		DiscoveryInterval:      10 * time.Millisecond,
		RequestEndpointRefresh: refresh.fire,
		SendCallMeMaybe:        callMeMaybe.fire,
	})

	if got := mgr.PathState(); got != PathRelay {
		t.Fatalf("PathState() = %v, want %v", got, PathRelay)
	}

	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if got := mgr.PathState(); got != PathRelay {
		t.Fatalf("PathState() after Start = %v, want %v before validation", got, PathRelay)
	}

	if !refresh.waitForCount(1, 200*time.Millisecond) {
		t.Fatal("discovery loop did not request endpoint refresh")
	}
	if !callMeMaybe.waitForCount(1, 200*time.Millisecond) {
		t.Fatal("discovery loop did not send call-me-maybe")
	}

	if err := mgr.UpdateDirectCandidates([]net.Addr{candidate}); err != nil {
		t.Fatalf("UpdateDirectCandidates() error = %v", err)
	}

	if !direct.waitForWriteTo(candidate, 200*time.Millisecond) {
		t.Fatal("discovery loop did not probe the late direct candidate")
	}

	if got := mgr.PathState(); got != PathRelay {
		t.Fatalf("PathState() before validation = %v, want %v", got, PathRelay)
	}

	if err := mgr.MarkDirectValidated(candidate); err != nil {
		t.Fatalf("MarkDirectValidated() error = %v", err)
	}

	if !waitForPath(t, mgr, PathDirect) {
		t.Fatalf("PathState() = %v, want %v", mgr.PathState(), PathDirect)
	}
}

func TestManagerFallsBackAndRestartsDiscoveryAfterDirectBreak(t *testing.T) {
	t.Helper()

	relay := newFakePacketConn(&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)})
	direct := newFakePacketConn(&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)})
	refresh := newFakeSignal()
	callMeMaybe := newFakeSignal()
	candidate := &net.UDPAddr{IP: net.IPv4(100, 64, 0, 3), Port: 23456}

	mgr := NewManager(ManagerConfig{
		RelayConn:              relay,
		DirectConn:             direct,
		DiscoveryInterval:      10 * time.Millisecond,
		RequestEndpointRefresh: refresh.fire,
		SendCallMeMaybe:        callMeMaybe.fire,
	})

	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := mgr.UpdateDirectCandidates([]net.Addr{candidate}); err != nil {
		t.Fatalf("UpdateDirectCandidates() error = %v", err)
	}
	if !direct.waitForWriteTo(candidate, 200*time.Millisecond) {
		t.Fatal("discovery loop did not probe the initial direct candidate")
	}
	if err := mgr.MarkDirectValidated(candidate); err != nil {
		t.Fatalf("MarkDirectValidated() error = %v", err)
	}
	if !waitForPath(t, mgr, PathDirect) {
		t.Fatalf("PathState() = %v, want %v before fallback", mgr.PathState(), PathDirect)
	}

	if err := mgr.MarkDirectBroken(); err != nil {
		t.Fatalf("MarkDirectBroken() error = %v", err)
	}

	if got := mgr.PathState(); got != PathRelay {
		t.Fatalf("PathState() = %v, want %v", got, PathRelay)
	}

	refreshCount := refresh.countNow()
	callMeMaybeCount := callMeMaybe.countNow()
	probeCount := direct.writeCountTo(candidate)

	if !refresh.waitForCount(refreshCount+1, 200*time.Millisecond) {
		t.Fatal("fallback did not restart endpoint refresh")
	}
	if !callMeMaybe.waitForCount(callMeMaybeCount+1, 200*time.Millisecond) {
		t.Fatal("fallback did not restart call-me-maybe")
	}
	if err := mgr.UpdateDirectCandidates([]net.Addr{candidate}); err != nil {
		t.Fatalf("UpdateDirectCandidates() after fallback error = %v", err)
	}
	if !direct.waitForWriteCountTo(candidate, probeCount+1, 200*time.Millisecond) {
		t.Fatal("discovery loop did not retry the direct candidate after fallback")
	}
	if err := mgr.MarkDirectValidated(candidate); err != nil {
		t.Fatalf("MarkDirectValidated() after fallback error = %v", err)
	}
	if !waitForPath(t, mgr, PathDirect) {
		t.Fatalf("PathState() after fallback recovery = %v, want %v", mgr.PathState(), PathDirect)
	}
}

func TestManagerDirectOnlyBrokenDoesNotInventRelay(t *testing.T) {
	t.Helper()

	direct := newFakePacketConn(&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)})
	candidate := &net.UDPAddr{IP: net.IPv4(100, 64, 0, 4), Port: 34567}

	mgr := NewManager(ManagerConfig{
		DirectConn:        direct,
		DiscoveryInterval: 10 * time.Millisecond,
	})

	if got := mgr.PathState(); got != PathUnknown {
		t.Fatalf("PathState() before Start = %v, want %v", got, PathUnknown)
	}

	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := mgr.UpdateDirectCandidates([]net.Addr{candidate}); err != nil {
		t.Fatalf("UpdateDirectCandidates() error = %v", err)
	}
	if !direct.waitForWriteTo(candidate, 200*time.Millisecond) {
		t.Fatal("discovery loop did not probe the direct-only candidate")
	}
	if err := mgr.MarkDirectValidated(candidate); err != nil {
		t.Fatalf("MarkDirectValidated() error = %v", err)
	}
	if !waitForPath(t, mgr, PathDirect) {
		t.Fatalf("PathState() = %v, want %v before fallback", mgr.PathState(), PathDirect)
	}

	if err := mgr.MarkDirectBroken(); err != nil {
		t.Fatalf("MarkDirectBroken() error = %v", err)
	}

	if got := mgr.PathState(); got != PathUnknown {
		t.Fatalf("PathState() = %v, want %v", got, PathUnknown)
	}
}

func TestManagerRelayOnlyStartKeepsRelay(t *testing.T) {
	t.Helper()

	relay := newFakePacketConn(&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)})

	mgr := NewManager(ManagerConfig{
		RelayConn: relay,
	})

	if got := mgr.PathState(); got != PathRelay {
		t.Fatalf("PathState() before Start = %v, want %v", got, PathRelay)
	}

	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if got := mgr.PathState(); got != PathRelay {
		t.Fatalf("PathState() after Start = %v, want %v", got, PathRelay)
	}
}

func TestManagerCanceledStartCanBeRetried(t *testing.T) {
	t.Helper()

	relay := newFakePacketConn(&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)})
	direct := newFakePacketConn(&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)})

	mgr := NewManager(ManagerConfig{
		RelayConn:  relay,
		DirectConn: direct,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := mgr.Start(ctx); err == nil {
		t.Fatal("Start() error = nil, want context cancellation")
	}

	if got := mgr.PathState(); got != PathRelay {
		t.Fatalf("PathState() after canceled start = %v, want %v", got, PathRelay)
	}

	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start() retry error = %v", err)
	}

	if got := mgr.PathState(); got != PathRelay {
		t.Fatalf("PathState() after retry = %v, want %v before discovery", got, PathRelay)
	}
}

func waitForPath(t *testing.T, mgr *Manager, want Path) bool {
	t.Helper()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if mgr.PathState() == want {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return mgr.PathState() == want
}
