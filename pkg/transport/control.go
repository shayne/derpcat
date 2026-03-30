package transport

import "net"

type controlKind int

const (
	controlEndpointRefresh controlKind = iota
	controlCallMeMaybe
	controlCandidateUpdate
	controlDirectValidated
	controlDirectBroken
)

type controlMessage struct {
	kind       controlKind
	candidates []net.Addr
	addr       net.Addr
}

func (m *Manager) MarkDirectBroken() error {
	return m.applyControl(controlMessage{kind: controlDirectBroken})
}

func (m *Manager) UpdateDirectCandidates(candidates []net.Addr) error {
	return m.applyControl(controlMessage{kind: controlCandidateUpdate, candidates: candidates})
}

func (m *Manager) MarkDirectValidated(addr net.Addr) error {
	return m.applyControl(controlMessage{kind: controlDirectValidated, addr: addr})
}

func (m *Manager) applyControl(msg controlMessage) error {
	m.mu.Lock()
	refresh := false
	callMeMaybe := false

	switch msg.kind {
	case controlEndpointRefresh:
		refresh = m.cfg.RequestEndpointRefresh != nil
	case controlCallMeMaybe:
		callMeMaybe = m.cfg.SendCallMeMaybe != nil
	case controlCandidateUpdate:
		m.state.updateCandidates(msg.candidates)
	case controlDirectValidated:
		m.state.markDirectValidated(msg.addr)
	case controlDirectBroken:
		m.state.markDirectBroken()
	}

	refreshHook := m.cfg.RequestEndpointRefresh
	callMeMaybeHook := m.cfg.SendCallMeMaybe
	m.mu.Unlock()

	if refresh {
		refreshHook()
	}
	if callMeMaybe {
		callMeMaybeHook()
	}
	return nil
}
