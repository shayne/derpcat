package traversal

import (
	"context"
	"net/netip"

	"tailscale.com/net/netcheck"
	"tailscale.com/net/netmon"
	"tailscale.com/tailcfg"
	"tailscale.com/types/logger"
)

func GatherCandidates(ctx context.Context, dm *tailcfg.DERPMap, mapped func() (netip.AddrPort, bool)) ([]string, error) {
	client := &netcheck.Client{
		Logf:   logger.Discard,
		NetMon: netmon.NewStatic(),
	}
	if err := client.Standalone(ctx, ":0"); err != nil {
		return nil, err
	}

	report, err := client.GetReport(ctx, dm, nil)
	if err != nil {
		return nil, err
	}

	v4, v6 := report.GetGlobalAddrs()
	return gatherCandidates(v4, v6, mapped), nil
}

func gatherCandidates(v4, v6 []netip.AddrPort, mapped func() (netip.AddrPort, bool)) []string {
	candidates := make([]string, 0, len(v4)+len(v6)+1)
	for _, addr := range v4 {
		candidates = append(candidates, addr.String())
	}
	for _, addr := range v6 {
		candidates = append(candidates, addr.String())
	}

	if mapped == nil {
		return candidates
	}

	mappedAddr, ok := mapped()
	return appendMappedCandidate(candidates, mappedAddr, ok)
}

func appendMappedCandidate(candidates []string, mapped netip.AddrPort, ok bool) []string {
	if !ok || !mapped.IsValid() {
		return candidates
	}

	mappedStr := mapped.String()
	for _, candidate := range candidates {
		if candidate == mappedStr {
			return candidates
		}
	}

	return append(candidates, mappedStr)
}
