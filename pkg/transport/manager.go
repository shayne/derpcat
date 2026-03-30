package transport

import (
	"context"
	"net"
	"sync"
	"time"
)

type ManagerConfig struct {
	RelayConn              net.PacketConn
	DirectConn             net.PacketConn
	DiscoveryInterval      time.Duration
	RequestEndpointRefresh func()
	SendCallMeMaybe        func()
}

type Manager struct {
	mu      sync.Mutex
	cfg     ManagerConfig
	state   pathState
	started bool
}

func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		cfg:   cfg,
		state: newPathState(cfg.RelayConn != nil, cfg.DirectConn != nil),
	}
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	if err := ctx.Err(); err != nil {
		m.mu.Unlock()
		return err
	}

	m.started = true
	m.mu.Unlock()

	go m.discoveryLoop(ctx)
	return nil
}

func (m *Manager) PathState() Path {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state.path()
}

func (m *Manager) discoveryInterval() time.Duration {
	if m.cfg.DiscoveryInterval > 0 {
		return m.cfg.DiscoveryInterval
	}
	return 250 * time.Millisecond
}
