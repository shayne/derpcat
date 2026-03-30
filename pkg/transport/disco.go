package transport

import (
	"context"
	"time"
)

var discoPingPayload = []byte("derpcat-disco-ping")

func (m *Manager) discoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(m.discoveryInterval())
	defer ticker.Stop()

	for {
		m.runDiscoveryCycle()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) runDiscoveryCycle() {
	plan := m.snapshotDiscoveryPlan()
	if !plan.shouldAttempt {
		return
	}

	if plan.needRefresh {
		_ = m.applyControl(controlMessage{kind: controlEndpointRefresh})
	}
	if plan.sendCallMe {
		_ = m.applyControl(controlMessage{kind: controlCallMeMaybe})
	}
	if m.cfg.DirectConn == nil {
		return
	}

	for _, target := range plan.probeTargets {
		if target == nil {
			continue
		}
		_, _ = m.cfg.DirectConn.WriteTo(discoPingPayload, target)
	}
}

func (m *Manager) snapshotDiscoveryPlan() discoveryPlan {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state.discoveryPlan()
}
