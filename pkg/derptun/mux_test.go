package derptun

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestMuxCarriesOneTCPStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientMux, serverMux := newMuxPair(t, time.Second)
	defer clientMux.Close()
	defer serverMux.Close()

	serverConnCh := make(chan net.Conn, 1)
	serverErrCh := make(chan error, 1)
	go func() {
		conn, err := serverMux.Accept(ctx)
		if err != nil {
			serverErrCh <- err
			return
		}
		serverConnCh <- conn
	}()

	clientConn, err := clientMux.OpenStream(ctx)
	if err != nil {
		t.Fatalf("OpenStream() error = %v", err)
	}
	defer clientConn.Close()

	var serverConn net.Conn
	select {
	case err := <-serverErrCh:
		t.Fatalf("Accept() error = %v", err)
	case serverConn = <-serverConnCh:
	case <-ctx.Done():
		t.Fatal("Accept() did not return")
	}
	defer serverConn.Close()

	if _, err := clientConn.Write([]byte("hello over mux")); err != nil {
		t.Fatalf("client Write() error = %v", err)
	}

	got := make([]byte, len("hello over mux"))
	if _, err := io.ReadFull(serverConn, got); err != nil {
		t.Fatalf("server Read() error = %v", err)
	}
	if !bytes.Equal(got, []byte("hello over mux")) {
		t.Fatalf("server got %q, want %q", got, "hello over mux")
	}

	if _, err := serverConn.Write([]byte("reply")); err != nil {
		t.Fatalf("server Write() error = %v", err)
	}

	reply := make([]byte, len("reply"))
	if _, err := io.ReadFull(clientConn, reply); err != nil {
		t.Fatalf("client Read() error = %v", err)
	}
	if !bytes.Equal(reply, []byte("reply")) {
		t.Fatalf("client got %q, want %q", reply, "reply")
	}
}

func TestMuxResendsUnackedDataAfterCarrierReplacement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientMux, serverMux := newMuxPair(t, 2*time.Second)
	defer clientMux.Close()
	defer serverMux.Close()

	serverConnCh := make(chan net.Conn, 1)
	serverErrCh := make(chan error, 1)
	go func() {
		conn, err := serverMux.Accept(ctx)
		if err != nil {
			serverErrCh <- err
			return
		}
		serverConnCh <- conn
	}()

	clientConn, err := clientMux.OpenStream(ctx)
	if err != nil {
		t.Fatalf("OpenStream() error = %v", err)
	}
	defer clientConn.Close()

	var serverConn net.Conn
	select {
	case err := <-serverErrCh:
		t.Fatalf("Accept() error = %v", err)
	case serverConn = <-serverConnCh:
	case <-ctx.Done():
		t.Fatal("Accept() did not return")
	}
	defer serverConn.Close()

	if _, err := clientConn.Write([]byte("before")); err != nil {
		t.Fatalf("initial client Write() error = %v", err)
	}
	before := make([]byte, len("before"))
	if _, err := io.ReadFull(serverConn, before); err != nil {
		t.Fatalf("initial server Read() error = %v", err)
	}
	if !bytes.Equal(before, []byte("before")) {
		t.Fatalf("initial payload = %q, want %q", before, "before")
	}

	writeDone := make(chan error, 1)
	go func() {
		_, err := clientConn.Write([]byte("after reconnect"))
		writeDone <- err
	}()

	time.Sleep(20 * time.Millisecond)
	closeBoth(t, clientMux.ReplaceCarrier, serverMux.ReplaceCarrier)

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("client Write() error = %v", err)
		}
	case <-ctx.Done():
		t.Fatal("client Write() did not complete after carrier replacement")
	}

	got := make([]byte, len("after reconnect"))
	if _, err := io.ReadFull(serverConn, got); err != nil {
		t.Fatalf("server Read() error = %v", err)
	}
	if !bytes.Equal(got, []byte("after reconnect")) {
		t.Fatalf("server got %q, want %q", got, "after reconnect")
	}
}

func newMuxPair(t *testing.T, reconnectTimeout time.Duration) (*Mux, *Mux) {
	t.Helper()

	clientCarrier, serverCarrier := net.Pipe()

	clientMux := NewMux(MuxConfig{
		Role:             MuxRoleClient,
		ReconnectTimeout: reconnectTimeout,
	})
	serverMux := NewMux(MuxConfig{
		Role:             MuxRoleServer,
		ReconnectTimeout: reconnectTimeout,
	})

	clientMux.ReplaceCarrier(clientCarrier)
	serverMux.ReplaceCarrier(serverCarrier)

	return clientMux, serverMux
}

func closeBoth(t *testing.T, replaceClient func(io.ReadWriteCloser), replaceServer func(io.ReadWriteCloser)) {
	t.Helper()

	nextClient, nextServer := net.Pipe()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		replaceClient(nextClient)
	}()
	go func() {
		defer wg.Done()
		replaceServer(nextServer)
	}()
	wg.Wait()
}
