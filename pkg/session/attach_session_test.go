package session

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

func TestListenAttachAndDialAttachLocalRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	listener, err := ListenAttach(ctx, AttachListenConfig{})
	if err != nil {
		t.Fatalf("ListenAttach() error = %v", err)
	}
	defer listener.Close()

	received := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := listener.Accept(ctx)
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		payload, err := io.ReadAll(conn)
		if err != nil {
			errCh <- err
			return
		}
		received <- payload
	}()

	conn, err := DialAttach(ctx, AttachDialConfig{Token: listener.Token})
	if err != nil {
		t.Fatalf("DialAttach() error = %v", err)
	}
	if _, err := conn.Write([]byte("hello attach")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("Accept/read error = %v", err)
	case got := <-received:
		if !bytes.Equal(got, []byte("hello attach")) {
			t.Fatalf("payload = %q, want %q", got, "hello attach")
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for payload")
	}
}
