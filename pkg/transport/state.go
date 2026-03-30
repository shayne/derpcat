package transport

// Path describes the currently selected transport path.
type Path int

const (
	PathUnknown Path = iota
	PathRelay
	PathDirect
)

type pathState struct {
	current        Path
	relayAvailable bool
	directReady    bool
	directBroken   bool
}

func newPathState(hasRelay, hasDirect bool) pathState {
	current := PathUnknown
	switch {
	case hasRelay:
		current = PathRelay
	case hasDirect:
		current = PathDirect
	}

	return pathState{
		current:        current,
		relayAvailable: hasRelay,
		directReady:    hasDirect,
	}
}

func (s pathState) path() Path {
	return s.current
}

func (s *pathState) markDirectReady() bool {
	if !s.directReady || s.directBroken || s.current == PathDirect {
		return false
	}
	s.current = PathDirect
	return true
}

func (s *pathState) markDirectBroken() bool {
	next := PathUnknown
	if s.relayAvailable {
		next = PathRelay
	}

	changed := s.current != next || !s.directBroken
	s.directBroken = true
	s.current = next
	return changed
}
