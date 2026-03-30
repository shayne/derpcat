package transport

import "net"

// Path describes the currently selected transport path.
type Path int

const (
	PathUnknown Path = iota
	PathRelay
	PathDirect
)

type pathState struct {
	current          Path
	relayConfigured  bool
	directConfigured bool
	directStale      bool
	endpoints        map[string]net.Addr
}

func newPathState(hasRelay, hasDirect bool) pathState {
	current := PathUnknown
	if hasRelay {
		current = PathRelay
	}

	return pathState{
		current:          current,
		relayConfigured:  hasRelay,
		directConfigured: hasDirect,
		directStale:      hasDirect,
		endpoints:        make(map[string]net.Addr),
	}
}

func (s pathState) path() Path {
	return s.current
}

type discoveryPlan struct {
	needRefresh   bool
	sendCallMe    bool
	probeTargets  []net.Addr
	shouldAttempt bool
}

func (s pathState) discoveryPlan() discoveryPlan {
	shouldAttempt := s.directConfigured && (s.current != PathDirect || s.directStale)
	if !shouldAttempt {
		return discoveryPlan{}
	}

	targets := make([]net.Addr, 0, len(s.endpoints))
	for _, endpoint := range s.endpoints {
		targets = append(targets, cloneAddr(endpoint))
	}

	return discoveryPlan{
		needRefresh:   true,
		sendCallMe:    s.relayConfigured,
		probeTargets:  targets,
		shouldAttempt: true,
	}
}

func (s *pathState) updateCandidates(candidates []net.Addr) bool {
	next := make(map[string]net.Addr, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		next[candidate.String()] = cloneAddr(candidate)
	}

	changed := len(next) != len(s.endpoints)
	if !changed {
		for key := range next {
			if _, ok := s.endpoints[key]; !ok {
				changed = true
				break
			}
		}
	}

	s.endpoints = next
	if len(s.endpoints) > 0 && s.directConfigured && s.current != PathDirect {
		s.directStale = true
	}
	return changed
}

func (s *pathState) markDirectValidated(addr net.Addr) bool {
	if !s.directConfigured || addr == nil {
		return false
	}
	if _, ok := s.endpoints[addr.String()]; !ok {
		return false
	}

	changed := s.current != PathDirect || s.directStale
	s.current = PathDirect
	s.directStale = false
	return changed
}

func (s *pathState) markDirectBroken() bool {
	next := PathUnknown
	if s.relayConfigured {
		next = PathRelay
	}

	changed := s.current != next || !s.directStale
	s.current = next
	if s.directConfigured {
		s.directStale = true
	}
	return changed
}

func cloneAddr(addr net.Addr) net.Addr {
	switch v := addr.(type) {
	case *net.UDPAddr:
		cp := *v
		if v.IP != nil {
			cp.IP = append(net.IP(nil), v.IP...)
		}
		return &cp
	case *net.IPAddr:
		cp := *v
		if v.IP != nil {
			cp.IP = append(net.IP(nil), v.IP...)
		}
		return &cp
	default:
		return addr
	}
}
