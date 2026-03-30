package transport

import (
	"net"
	"sync"
	"time"
)

type packet struct {
	payload []byte
	addr    net.Addr
}

type fakePacketConn struct {
	mu       sync.Mutex
	local    net.Addr
	reads    []packet
	writes   []packet
	notify   chan struct{}
	closed   bool
	deadline time.Time
}

func newFakePacketConn(local net.Addr) *fakePacketConn {
	return &fakePacketConn{
		local:  local,
		notify: make(chan struct{}),
	}
}

func (c *fakePacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	for {
		c.mu.Lock()
		if len(c.reads) > 0 {
			pkt := c.reads[0]
			c.reads = c.reads[1:]
			c.mu.Unlock()
			n := copy(b, pkt.payload)
			return n, pkt.addr, nil
		}
		if c.closed {
			c.mu.Unlock()
			return 0, nil, net.ErrClosed
		}
		deadline := c.deadline
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			c.mu.Unlock()
			return 0, nil, timeoutErr{}
		}
		waitCh := c.notify
		c.mu.Unlock()

		if deadline.IsZero() {
			<-waitCh
			continue
		}

		delay := time.Until(deadline)
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-waitCh:
		case <-timer.C:
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
}

func (c *fakePacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, net.ErrClosed
	}
	c.writes = append(c.writes, packet{
		payload: append([]byte(nil), b...),
		addr:    addr,
	})
	c.signalLocked()
	return len(b), nil
}

func (c *fakePacketConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.signalLocked()
	return nil
}

func (c *fakePacketConn) LocalAddr() net.Addr { return c.local }

func (c *fakePacketConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deadline = t
	c.signalLocked()
	return nil
}

func (c *fakePacketConn) SetReadDeadline(t time.Time) error { return c.SetDeadline(t) }
func (c *fakePacketConn) SetWriteDeadline(time.Time) error  { return nil }

func (c *fakePacketConn) signalLocked() {
	next := make(chan struct{})
	close(c.notify)
	c.notify = next
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

var _ net.PacketConn = (*fakePacketConn)(nil)
var _ error = timeoutErr{}

func (c *fakePacketConn) waitForWriteTo(addr net.Addr, timeout time.Duration) bool {
	return c.waitForWriteCountTo(addr, 1, timeout)
}

func (c *fakePacketConn) writeCountTo(addr net.Addr) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for _, pkt := range c.writes {
		if pkt.addr.String() == addr.String() {
			count++
		}
	}
	return count
}

func (c *fakePacketConn) waitForWriteCountTo(addr net.Addr, n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		c.mu.Lock()
		count := 0
		for _, pkt := range c.writes {
			if pkt.addr.String() == addr.String() {
				count++
			}
		}
		if count >= n {
			c.mu.Unlock()
			return true
		}
		if c.closed {
			c.mu.Unlock()
			return false
		}
		waitCh := c.notify
		c.mu.Unlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}

		timer := time.NewTimer(remaining)
		select {
		case <-waitCh:
		case <-timer.C:
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
}

type fakeSignal struct {
	mu     sync.Mutex
	count  int
	notify chan struct{}
}

func newFakeSignal() *fakeSignal {
	return &fakeSignal{notify: make(chan struct{})}
}

func (s *fakeSignal) fire() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count++
	next := make(chan struct{})
	close(s.notify)
	s.notify = next
}

func (s *fakeSignal) waitForCount(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		if s.count >= n {
			s.mu.Unlock()
			return true
		}
		waitCh := s.notify
		s.mu.Unlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}

		timer := time.NewTimer(remaining)
		select {
		case <-waitCh:
		case <-timer.C:
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
}

func (s *fakeSignal) countNow() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}
