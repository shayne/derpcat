package traversal

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestProbePromotesDirectPath(t *testing.T) {
	a, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	defer a.Close()

	b, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := ProbeDirect(ctx, a, b.LocalAddr().String(), b, a.LocalAddr().String())
	if err != nil {
		t.Fatalf("ProbeDirect() error = %v", err)
	}
	if !result.Direct {
		t.Fatalf("Direct = false, want true")
	}
}

func TestProbeFallsBackWhenNoPeerResponds(t *testing.T) {
	a, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	defer a.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	result, err := ProbeDirect(ctx, a, "127.0.0.1:9", nil, "")
	if err != nil {
		t.Fatalf("ProbeDirect() error = %v", err)
	}
	if result.Direct {
		t.Fatalf("Direct = true, want false")
	}
}

func TestGatherCandidatesRejectsNilDERPMap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := GatherCandidates(ctx, nil, nil); err == nil {
		t.Fatal("GatherCandidates() error = nil, want non-nil")
	}
}

func TestGatherCandidatesMergesMappedEndpoint(t *testing.T) {
	got := gatherCandidates(
		[]netip.AddrPort{netip.MustParseAddrPort("100.64.0.10:1000")},
		[]netip.AddrPort{netip.MustParseAddrPort("[fd7a:115c:a1e0::1]:2000")},
		func() (netip.AddrPort, bool) {
			return netip.MustParseAddrPort("100.64.0.11:4242"), true
		},
	)

	want := []string{"100.64.0.10:1000", "[fd7a:115c:a1e0::1]:2000", "100.64.0.11:4242"}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGatherCandidatesSkipsDuplicateMappedEndpoint(t *testing.T) {
	got := gatherCandidates(
		[]netip.AddrPort{netip.MustParseAddrPort("100.64.0.10:4242")},
		nil,
		func() (netip.AddrPort, bool) {
			return netip.MustParseAddrPort("100.64.0.10:4242"), true
		},
	)

	want := []string{"100.64.0.10:4242"}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAppendMappedCandidateRejectsInvalidEndpoint(t *testing.T) {
	candidates := []string{"100.64.0.10:4242"}
	got := appendMappedCandidate(candidates, netip.AddrPort{}, true)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
}
