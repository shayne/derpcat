package probe

import (
	"slices"
	"testing"
)

func TestClassifyTopologyDetectsSSHFrontDoorMismatch(t *testing.T) {
	report := TopologyReport{
		DNSAddresses: []string{"161.210.92.1"},
		Remote: TopologyHost{
			EgressIP: "44.240.253.236",
			Interfaces: []TopologyInterface{
				{Name: "eth0", Addrs: []string{"10.42.0.64/16"}},
			},
		},
	}

	got := ClassifyTopology(report)

	if !slices.Contains(got, TopologyClassSSHFrontDoorMismatch) {
		t.Fatalf("ClassifyTopology() = %v, want %s", got, TopologyClassSSHFrontDoorMismatch)
	}
}

func TestClassifyTopologyDetectsRemoteUDPUnreachable(t *testing.T) {
	report := TopologyReport{
		UDPReachability: []UDPReachabilityResult{
			{Target: "dns-a", Address: "161.210.92.1:47000", Received: false},
			{Target: "egress", Address: "44.240.253.236:47000", Received: false},
		},
	}

	got := ClassifyTopology(report)

	if !slices.Contains(got, TopologyClassRemoteUDPUnreachable) {
		t.Fatalf("ClassifyTopology() = %v, want %s", got, TopologyClassRemoteUDPUnreachable)
	}
}

func TestClassifyTopologyDetectsDirectUDPPossible(t *testing.T) {
	report := TopologyReport{
		UDPReachability: []UDPReachabilityResult{
			{Target: "egress", Address: "203.0.113.10:47000", Received: true},
		},
		PunchTests: []UDPPunchResult{
			{Name: "simultaneous", LocalReceived: true, RemoteReceived: true},
		},
	}

	got := ClassifyTopology(report)

	if !slices.Contains(got, TopologyClassDirectUDPPossible) {
		t.Fatalf("ClassifyTopology() = %v, want %s", got, TopologyClassDirectUDPPossible)
	}
}
