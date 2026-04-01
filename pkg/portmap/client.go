package portmap

import (
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/shayne/derpcat/pkg/telemetry"
	"tailscale.com/net/portmapper/portmappertype"
)

type mapper interface {
	SetLocalPort(uint16)
	HaveMapping() bool
	GetCachedMappingOrStartCreatingOne() (netip.AddrPort, bool)
}

type Client struct {
	mu        sync.Mutex
	mapper    mapper
	emitter   *telemetry.Emitter
	localPort uint16
	mapped    netip.AddrPort
	have      bool
}

func NewForTest(m mapper, emitter *telemetry.Emitter) *Client {
	return &Client{mapper: m, emitter: emitter}
}

func (c *Client) SetLocalPort(port uint16) {
	if c == nil || c.mapper == nil {
		return
	}

	c.mu.Lock()
	if c.localPort == port {
		c.mapper.SetLocalPort(port)
		c.mu.Unlock()
		return
	}

	c.localPort = port
	c.mapped = netip.AddrPort{}
	c.have = false
	c.mapper.SetLocalPort(port)
	c.mu.Unlock()
}

func (c *Client) Refresh(now time.Time) bool {
	_ = now
	if c == nil || c.mapper == nil {
		return false
	}

	c.mu.Lock()
	next, ok := c.mapper.GetCachedMappingOrStartCreatingOne()
	changed := c.have != ok || c.mapped != next
	c.have = ok
	c.mapped = next
	c.mu.Unlock()

	if c.emitter != nil {
		if ok {
			c.emitter.Debug(fmt.Sprintf("portmap=external external=%s", next))
		} else {
			c.emitter.Debug("portmap=none")
		}
	}

	return changed
}

func (c *Client) Snapshot() (netip.AddrPort, bool) {
	if c == nil {
		return netip.AddrPort{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mapped, c.have
}

var _ mapper = (portmappertype.Client)(nil)
