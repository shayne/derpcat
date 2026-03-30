package session

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shayne/derpcat/pkg/telemetry"
	"go4.org/mem"
	"tailscale.com/types/key"
)

func TestDERPPublicKeyRaw32RoundTrip(t *testing.T) {
	want := [32]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
	}

	pub := key.NodePublicFromRaw32(mem.B(want[:]))
	if got := derpPublicKeyRaw32(pub); got != want {
		t.Fatalf("derpPublicKeyRaw32() = %x, want %x", got, want)
	}
}

func TestRelayOnlyStdioRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var listenerOut bytes.Buffer
	var senderIn bytes.Buffer
	senderIn.WriteString("hello over derp")

	listenerReady := make(chan string, 1)
	go func() {
		token, err := Listen(ctx, ListenConfig{
			Emitter:   telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
			TokenSink: listenerReady,
			StdioOut:  &listenerOut,
		})
		if err != nil || token == "" {
			t.Errorf("Listen() err=%v token=%q", err, token)
		}
	}()

	token := <-listenerReady
	if err := Send(ctx, SendConfig{
		Token:      token,
		StdioIn:    &senderIn,
		Emitter:    telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
		ForceRelay: true,
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if got := listenerOut.String(); got != "hello over derp" {
		t.Fatalf("listener output = %q, want %q", got, "hello over derp")
	}
}

func TestSessionPromotesDirectStateWhenProbeSucceeds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var listenerOut bytes.Buffer
	var senderIn bytes.Buffer
	var listenerStatus bytes.Buffer
	var senderStatus bytes.Buffer
	senderIn.WriteString("hello direct")

	listenerReady := make(chan string, 1)
	go func() {
		token, err := Listen(ctx, ListenConfig{
			Emitter:   telemetry.New(&listenerStatus, telemetry.LevelDefault),
			TokenSink: listenerReady,
			StdioOut:  &listenerOut,
		})
		if err != nil || token == "" {
			t.Errorf("Listen() err=%v token=%q", err, token)
		}
	}()

	token := <-listenerReady
	if err := Send(ctx, SendConfig{
		Token:   token,
		StdioIn: &senderIn,
		Emitter: telemetry.New(&senderStatus, telemetry.LevelDefault),
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if !strings.Contains(listenerStatus.String(), string(StateDirect)) {
		t.Fatalf("listener statuses = %q, want %q", listenerStatus.String(), StateDirect)
	}
	if !strings.Contains(senderStatus.String(), string(StateDirect)) {
		t.Fatalf("sender statuses = %q, want %q", senderStatus.String(), StateDirect)
	}
	if got := listenerOut.String(); got != "hello direct" {
		t.Fatalf("listener output = %q, want %q", got, "hello direct")
	}
}

func TestShareOpenUsesEphemeralLocalBind(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	backendAddr, backendDone := startEchoServer(t, ctx)

	tokenSink := make(chan string, 1)
	shareErr := make(chan error, 1)
	go func() {
		_, err := Share(ctx, ShareConfig{
			Emitter:    telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
			TokenSink:  tokenSink,
			TargetAddr: backendAddr,
		})
		shareErr <- err
	}()

	tok := <-tokenSink
	bindSink := make(chan string, 1)
	openErr := make(chan error, 1)
	go func() {
		openErr <- Open(ctx, OpenConfig{
			Token:        tok,
			BindAddrSink: bindSink,
			Emitter:      telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
		})
	}()

	bindAddr := <-bindSink
	if !strings.HasPrefix(bindAddr, "127.0.0.1:") {
		t.Fatalf("bindAddr = %q, want ephemeral localhost listener", bindAddr)
	}
	if strings.HasSuffix(bindAddr, ":0") {
		t.Fatalf("bindAddr = %q, want assigned port", bindAddr)
	}

	cancel()
	waitNoErr(t, <-openErr)
	waitNoErr(t, <-shareErr)
	backendDone()
}

func TestShareOpenForwardsSequentialConnections(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	backendAddr, backendDone := startEchoServer(t, ctx)
	openAddr, stop, shareErr, openErr := startSharedSession(t, ctx, backendAddr, "")

	for _, payload := range []string{"alpha", "beta", "gamma"} {
		reply := roundTripTCP(t, ctx, openAddr, payload)
		if reply != payload {
			t.Fatalf("reply = %q, want %q", reply, payload)
		}
	}

	stop()
	waitNoErr(t, <-openErr)
	waitNoErr(t, <-shareErr)
	backendDone()
}

func TestShareOpenForwardsConcurrentConnections(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	backendAddr, backendDone := startEchoServer(t, ctx)
	openAddr, stop, shareErr, openErr := startSharedSession(t, ctx, backendAddr, "")

	payloads := []string{"one", "two", "three", "four", "five"}
	var wg sync.WaitGroup
	errCh := make(chan error, len(payloads))
	for _, payload := range payloads {
		wg.Add(1)
		go func(payload string) {
			defer wg.Done()
			reply := roundTripTCP(t, ctx, openAddr, payload)
			if reply != payload {
				errCh <- errors.New("reply mismatch")
			}
		}(payload)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	stop()
	waitNoErr(t, <-openErr)
	waitNoErr(t, <-shareErr)
	backendDone()
}

func TestShareTokenAllowsOneClaimer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	backendAddr, backendDone := startEchoServer(t, ctx)

	tokenSink := make(chan string, 1)
	shareErr := make(chan error, 1)
	go func() {
		_, err := Share(ctx, ShareConfig{
			Emitter:    telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
			TokenSink:  tokenSink,
			TargetAddr: backendAddr,
		})
		shareErr <- err
	}()

	tok := <-tokenSink
	bindSink := make(chan string, 1)
	openErr := make(chan error, 1)
	go func() {
		openErr <- Open(ctx, OpenConfig{
			Token:        tok,
			BindAddrSink: bindSink,
			Emitter:      telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
		})
	}()
	<-bindSink

	err := Open(ctx, OpenConfig{
		Token:   tok,
		Emitter: telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
	})
	if !errors.Is(err, ErrSessionClaimed) {
		t.Fatalf("Open() error = %v, want %v", err, ErrSessionClaimed)
	}

	cancel()
	waitNoErr(t, <-openErr)
	waitNoErr(t, <-shareErr)
	backendDone()
}

func TestExternalListenSendCanUpgradeAfterRelayStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var listenerStatus bytes.Buffer
	var senderStatus bytes.Buffer
	var listenerOut bytes.Buffer
	var senderIn bytes.Buffer
	senderIn.WriteString("relay-first-then-direct")

	listenerReady := make(chan string, 1)
	go func() {
		token, err := Listen(ctx, ListenConfig{
			Emitter:    telemetry.New(&listenerStatus, telemetry.LevelDefault),
			TokenSink:  listenerReady,
			StdioOut:   &listenerOut,
			ForceRelay: true,
		})
		if err != nil || token == "" {
			t.Errorf("Listen() err=%v token=%q", err, token)
		}
	}()

	token := <-listenerReady
	if err := Send(ctx, SendConfig{
		Token:      token,
		StdioIn:    &senderIn,
		Emitter:    telemetry.New(&senderStatus, telemetry.LevelDefault),
		ForceRelay: true,
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if !strings.Contains(listenerStatus.String(), string(StateDirect)) {
		t.Fatalf("listener statuses = %q, want %q after relay-first start", listenerStatus.String(), StateDirect)
	}
	if !strings.Contains(senderStatus.String(), string(StateDirect)) {
		t.Fatalf("sender statuses = %q, want %q after relay-first start", senderStatus.String(), StateDirect)
	}
	if got := listenerOut.String(); got != "relay-first-then-direct" {
		t.Fatalf("listener output = %q, want %q", got, "relay-first-then-direct")
	}
}

func connectWithRetry(ctx context.Context, addr string) (net.Conn, error) {
	var d net.Dialer
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func startEchoServer(t *testing.T, ctx context.Context) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := acceptNetListener(ctx, listener)
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}(conn)
		}
	}()

	return listener.Addr().String(), func() {
		_ = listener.Close()
		<-done
	}
}

func startSharedSession(t *testing.T, ctx context.Context, backendAddr, bindAddr string) (string, func(), <-chan error, <-chan error) {
	t.Helper()

	sessionCtx, cancel := context.WithCancel(ctx)
	tokenSink := make(chan string, 1)
	shareErr := make(chan error, 1)
	go func() {
		_, err := Share(sessionCtx, ShareConfig{
			Emitter:    telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
			TokenSink:  tokenSink,
			TargetAddr: backendAddr,
		})
		shareErr <- err
	}()

	tok := <-tokenSink
	bindSink := make(chan string, 1)
	openErr := make(chan error, 1)
	go func() {
		openErr <- Open(sessionCtx, OpenConfig{
			Token:        tok,
			BindAddr:     bindAddr,
			BindAddrSink: bindSink,
			Emitter:      telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
		})
	}()

	return <-bindSink, cancel, shareErr, openErr
}

func roundTripTCP(t *testing.T, ctx context.Context, addr, payload string) string {
	t.Helper()

	conn, err := connectWithRetry(ctx, addr)
	if err != nil {
		t.Fatalf("connectWithRetry() error = %v", err)
	}
	defer conn.Close()

	if _, err := io.WriteString(conn, payload); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}
	return string(buf)
}

func waitNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}
