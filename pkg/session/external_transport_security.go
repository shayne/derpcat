package session

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"

	"github.com/shayne/derphole/pkg/token"
	"github.com/shayne/derphole/pkg/transport"
	"tailscale.com/types/key"
)

var externalTransportDiscoveryMACDomain = []byte("derphole-transport-direct-udp-disco-mac-v1")

func externalTransportDiscoveryKey(tok token.Token, localDERP, peerDERP key.NodePublic) transport.DiscoveryKey {
	localRaw := localDERP.AppendTo(nil)
	peerRaw := peerDERP.AppendTo(nil)
	first, second := localRaw, peerRaw
	if bytes.Compare(first, second) > 0 {
		first, second = second, first
	}
	mac := hmac.New(sha256.New, tok.BearerSecret[:])
	mac.Write(externalTransportDiscoveryMACDomain)
	mac.Write(tok.SessionID[:])
	mac.Write(first)
	mac.Write(second)
	var out transport.DiscoveryKey
	copy(out[:], mac.Sum(nil))
	return out
}
