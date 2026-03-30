package transport

import (
	"errors"
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
	reads    chan packet
	writes   []packet
	closed   bool
	deadline time.Time
}

func newFakePacketConn(local net.Addr) *fakePacketConn {
	return &fakePacketConn{
		local: local,
		reads: make(chan packet, 8),
	}
}

func (c *fakePacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	select {
	case pkt := <-c.reads:
		n := copy(b, pkt.payload)
		return n, pkt.addr, nil
	default:
	}

	c.mu.Lock()
	closed := c.closed
	deadline := c.deadline
	c.mu.Unlock()
	if closed {
		return 0, nil, net.ErrClosed
	}
	if !deadline.IsZero() && time.Now().After(deadline) {
		return 0, nil, timeoutErr{}
	}
	return 0, nil, timeoutErr{}
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
	return len(b), nil
}

func (c *fakePacketConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakePacketConn) LocalAddr() net.Addr { return c.local }

func (c *fakePacketConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deadline = t
	return nil
}

func (c *fakePacketConn) SetReadDeadline(t time.Time) error { return c.SetDeadline(t) }
func (c *fakePacketConn) SetWriteDeadline(time.Time) error  { return nil }

func (c *fakePacketConn) inject(payload []byte, addr net.Addr) {
	c.reads <- packet{payload: append([]byte(nil), payload...), addr: addr}
}

func (c *fakePacketConn) writesSnapshot() []packet {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]packet, len(c.writes))
	copy(out, c.writes)
	return out
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

var _ net.PacketConn = (*fakePacketConn)(nil)
var _ error = timeoutErr{}

var errFakePacketTimeout = errors.New("fake packet timeout")
