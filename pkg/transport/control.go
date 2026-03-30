package transport

func (m *Manager) MarkDirectBroken() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.markDirectBroken()
	return nil
}
