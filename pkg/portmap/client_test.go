package portmap

import (
	"bytes"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shayne/derpcat/pkg/telemetry"
)

type fakeMapper struct {
	localPort uint16
	external  netip.AddrPort
	have      bool
}

func (f *fakeMapper) SetLocalPort(p uint16) { f.localPort = p }

func (f *fakeMapper) HaveMapping() bool { return f.have }

func (f *fakeMapper) GetCachedMappingOrStartCreatingOne() (netip.AddrPort, bool) {
	return f.external, f.have
}

func newMappedClient(t *testing.T) (*Client, *fakeMapper, *bytes.Buffer) {
	t.Helper()

	mapper := &fakeMapper{
		have:     true,
		external: netip.MustParseAddrPort("198.51.100.10:54321"),
	}
	var buf bytes.Buffer
	c := NewForTest(mapper, telemetry.New(&buf, telemetry.LevelVerbose))

	if changed := c.Refresh(time.Now()); !changed {
		t.Fatal("initial Refresh() changed = false, want true")
	}

	if got, ok := c.Snapshot(); !ok || got != mapper.external {
		t.Fatalf("Snapshot() = %v, %v, want %v, true", got, ok, mapper.external)
	}

	return c, mapper, &buf
}

func TestClientSetLocalPortAndSnapshot(t *testing.T) {
	c, mapper, _ := newMappedClient(t)

	c.SetLocalPort(45678)
	if got, want := mapper.localPort, uint16(45678); got != want {
		t.Fatalf("local port = %d, want %d", got, want)
	}

	if got, ok := c.Snapshot(); ok {
		t.Fatalf("Snapshot() = %v, %v, want zero, false", got, ok)
	}

	if changed := c.Refresh(time.Now()); !changed {
		t.Fatal("Refresh() after rebinding changed = false, want true")
	}

	c.SetLocalPort(45678)
	if got, want := mapper.localPort, uint16(45678); got != want {
		t.Fatalf("local port = %d, want %d", got, want)
	}

	if got, ok := c.Snapshot(); !ok || got != mapper.external {
		t.Fatalf("Snapshot() = %v, %v, want %v, true", got, ok, mapper.external)
	}
}

func TestClientSetLocalPortChangedPortClearsSnapshot(t *testing.T) {
	c, mapper, _ := newMappedClient(t)

	c.SetLocalPort(45678)
	if got, want := mapper.localPort, uint16(45678); got != want {
		t.Fatalf("local port = %d, want %d", got, want)
	}

	if got, ok := c.Snapshot(); ok {
		t.Fatalf("Snapshot() = %v, %v, want zero, false", got, ok)
	}

	if changed := c.Refresh(time.Now()); !changed {
		t.Fatal("Refresh() after rebinding changed = false, want true")
	}

	c.SetLocalPort(45679)
	if got, want := mapper.localPort, uint16(45679); got != want {
		t.Fatalf("local port = %d, want %d", got, want)
	}

	if got, ok := c.Snapshot(); ok {
		t.Fatalf("Snapshot() = %v, %v, want zero, false", got, ok)
	}
}

type blockingMapper struct {
	mu             sync.Mutex
	localPort      uint16
	externalByPort map[uint16]netip.AddrPort
	have           bool
	setStarted     chan uint16
	allowSet       chan struct{}
}

func (m *blockingMapper) SetLocalPort(p uint16) {
	m.setStarted <- p
	<-m.allowSet
	m.mu.Lock()
	m.localPort = p
	m.mu.Unlock()
}

func (m *blockingMapper) HaveMapping() bool { return m.have }

func (m *blockingMapper) GetCachedMappingOrStartCreatingOne() (netip.AddrPort, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	external, ok := m.externalByPort[m.localPort]
	if !ok {
		return netip.AddrPort{}, false
	}
	return external, true
}

func TestClientSetLocalPortBlocksRefreshUntilComplete(t *testing.T) {
	mapper := &blockingMapper{
		localPort: 45678,
		externalByPort: map[uint16]netip.AddrPort{
			45678: netip.MustParseAddrPort("198.51.100.10:54321"),
			45679: netip.MustParseAddrPort("198.51.100.10:54322"),
		},
		have:       true,
		setStarted: make(chan uint16, 1),
		allowSet:   make(chan struct{}),
	}
	var buf bytes.Buffer
	c := NewForTest(mapper, telemetry.New(&buf, telemetry.LevelVerbose))

	if changed := c.Refresh(time.Now()); !changed {
		t.Fatal("initial Refresh() changed = false, want true")
	}

	setDone := make(chan struct{})
	go func() {
		c.SetLocalPort(45679)
		close(setDone)
	}()

	select {
	case got := <-mapper.setStarted:
		if got != 45679 {
			t.Fatalf("SetLocalPort arg = %d, want 45679", got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for SetLocalPort to enter mapper")
	}

	refreshDone := make(chan struct {
		changed bool
		snap    netip.AddrPort
		ok      bool
	})
	go func() {
		changed := c.Refresh(time.Now())
		snap, ok := c.Snapshot()
		refreshDone <- struct {
			changed bool
			snap    netip.AddrPort
			ok      bool
		}{changed: changed, snap: snap, ok: ok}
	}()

	select {
	case <-refreshDone:
		t.Fatal("Refresh completed before SetLocalPort finished")
	case <-time.After(100 * time.Millisecond):
	}

	close(mapper.allowSet)

	select {
	case <-setDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for SetLocalPort to finish")
	}

	select {
	case got := <-refreshDone:
		if !got.changed {
			t.Fatal("Refresh() changed = false, want true")
		}
		if !got.ok || got.snap != mapper.externalByPort[45679] {
			t.Fatalf("Snapshot() = %v, %v, want %v, true", got.snap, got.ok, mapper.externalByPort[45679])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for Refresh to finish")
	}
}

func TestClientRefreshPublishesExternalMapping(t *testing.T) {
	mapper := &fakeMapper{
		have:     true,
		external: netip.MustParseAddrPort("198.51.100.10:54321"),
	}
	var buf bytes.Buffer
	c := NewForTest(mapper, telemetry.New(&buf, telemetry.LevelVerbose))

	changed := c.Refresh(time.Now())
	if !changed {
		t.Fatal("Refresh() changed = false, want true")
	}

	got, ok := c.Snapshot()
	if !ok || got != mapper.external {
		t.Fatalf("Snapshot() = %v, %v, want %v, true", got, ok, mapper.external)
	}

	if got := buf.String(); !strings.Contains(got, "portmap=external external=198.51.100.10:54321") {
		t.Fatalf("debug output = %q, want external mapping line", got)
	}
}

func TestClientRefreshTransitionsAndStability(t *testing.T) {
	mapper := &fakeMapper{
		have:     true,
		external: netip.MustParseAddrPort("198.51.100.10:54321"),
	}
	var buf bytes.Buffer
	c := NewForTest(mapper, telemetry.New(&buf, telemetry.LevelVerbose))

	if changed := c.Refresh(time.Now()); !changed {
		t.Fatal("initial Refresh() changed = false, want true")
	}

	mapper.have = false
	mapper.external = netip.AddrPort{}

	if changed := c.Refresh(time.Now()); !changed {
		t.Fatal("Refresh() mapped->unmapped changed = false, want true")
	}

	if got, ok := c.Snapshot(); ok {
		t.Fatalf("Snapshot() = %v, %v, want zero, false", got, ok)
	}

	if changed := c.Refresh(time.Now()); changed {
		t.Fatal("second identical Refresh() changed = true, want false")
	}
}
