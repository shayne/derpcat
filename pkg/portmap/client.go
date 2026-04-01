package portmap

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/shayne/derpcat/pkg/telemetry"
	"tailscale.com/net/netmon"
	"tailscale.com/net/portmapper"
	"tailscale.com/net/portmapper/portmappertype"
	"tailscale.com/types/logger"
	"tailscale.com/util/eventbus"
)

type mapper interface {
	SetLocalPort(uint16)
	SetGatewayLookupFunc(func() (gw, myIP netip.Addr, ok bool))
	HaveMapping() bool
	GetCachedMappingOrStartCreatingOne() (netip.AddrPort, bool)
	Close() error
}

var newNetMon = netmon.New
var newPortmapperClient = func(c portmapper.Config) mapper {
	return portmapper.NewClient(c)
}

type Client struct {
	mu        sync.Mutex
	closeOnce sync.Once
	closeErr  error
	mapper    mapper
	monitor   *netmon.Monitor
	bus       *eventbus.Bus
	emitter   *telemetry.Emitter
	localPort uint16
	mapped    netip.AddrPort
	have      bool
}

func NewForTest(m mapper, emitter *telemetry.Emitter) *Client {
	return &Client{mapper: m, emitter: emitter}
}

func New(emitter *telemetry.Emitter) *Client {
	bus := eventbus.New()
	nm, err := newNetMon(bus, logger.Discard)
	useGatewayLookup := err == nil
	if err != nil {
		nm = netmon.NewStatic()
	} else {
		nm.Start()
	}
	pm := newPortmapperClient(portmapper.Config{
		EventBus: bus,
		NetMon:   nm,
		Logf:     logger.Discard,
	})
	if useGatewayLookup {
		pm.SetGatewayLookupFunc(nm.GatewayAndSelfIP)
	}

	return &Client{
		mapper:  pm,
		monitor: nm,
		bus:     bus,
		emitter: emitter,
	}
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

func (c *Client) SnapshotAddrs() []net.Addr {
	mapped, ok := c.Snapshot()
	if !ok || !mapped.Addr().IsValid() || mapped.Port() == 0 {
		return nil
	}

	return []net.Addr{
		&net.UDPAddr{
			IP:   append(net.IP(nil), mapped.Addr().AsSlice()...),
			Port: int(mapped.Port()),
			Zone: mapped.Addr().Zone(),
		},
	}
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	c.closeOnce.Do(func() {
		var errs []error
		if c.mapper != nil {
			if err := c.mapper.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		if c.monitor != nil {
			if err := c.monitor.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		if c.bus != nil {
			c.bus.Close()
		}
		c.closeErr = errors.Join(errs...)
	})
	return c.closeErr
}

var _ mapper = (portmappertype.Client)(nil)
