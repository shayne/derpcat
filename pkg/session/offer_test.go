package session

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shayne/derpcat/pkg/telemetry"
)

func TestPublicRelayOnlyOfferedStdioRoundTrip(t *testing.T) {
	srv := newSessionTestDERPServer(t)
	t.Setenv("DERPCAT_TEST_DERP_MAP_URL", srv.MapURL)
	t.Setenv("DERPCAT_TEST_DERP_SERVER_URL", srv.DERPURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tokenSink := make(chan string, 1)
	offerErr := make(chan error, 1)
	go func() {
		_, err := Offer(ctx, OfferConfig{
			Emitter:       telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
			TokenSink:     tokenSink,
			StdioIn:       strings.NewReader("public offered payload"),
			ForceRelay:    true,
			UsePublicDERP: true,
		})
		offerErr <- err
	}()

	token := <-tokenSink
	var receiverOut bytes.Buffer
	if err := Receive(ctx, ReceiveConfig{
		Token:         token,
		Emitter:       telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
		StdioOut:      &receiverOut,
		ForceRelay:    true,
		UsePublicDERP: true,
	}); err != nil {
		t.Fatalf("Receive() error = %v", err)
	}

	if err := <-offerErr; err != nil {
		t.Fatalf("Offer() error = %v", err)
	}
	if got := receiverOut.String(); got != "public offered payload" {
		t.Fatalf("receiver output = %q, want %q", got, "public offered payload")
	}
}

func TestOfferedStdioStartsRelayPayloadBeforeDelayedDirectPromotion(t *testing.T) {
	t.Setenv("DERPCAT_FAKE_TRANSPORT", "1")
	t.Setenv("DERPCAT_FAKE_TRANSPORT_ENABLE_DIRECT_AT", strconv.FormatInt(time.Now().Add(24*time.Hour).UnixNano(), 10))

	srv := newSessionTestDERPServer(t)
	t.Setenv("DERPCAT_TEST_DERP_MAP_URL", srv.MapURL)
	t.Setenv("DERPCAT_TEST_DERP_SERVER_URL", srv.DERPURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tokenSink := make(chan string, 1)
	offerErr := make(chan error, 1)
	go func() {
		_, err := Offer(ctx, OfferConfig{
			Emitter:       telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
			TokenSink:     tokenSink,
			StdioIn:       strings.NewReader("relay-first offered payload"),
			UsePublicDERP: true,
		})
		offerErr <- err
	}()

	token := <-tokenSink
	var receiverOut bytes.Buffer
	start := time.Now()
	if err := Receive(ctx, ReceiveConfig{
		Token:         token,
		Emitter:       telemetry.New(&bytes.Buffer{}, telemetry.LevelSilent),
		StdioOut:      &receiverOut,
		UsePublicDERP: true,
	}); err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 3*time.Second {
		t.Fatalf("Receive() elapsed = %v, want relay payload before delayed direct promotion", elapsed)
	}

	if err := <-offerErr; err != nil {
		t.Fatalf("Offer() error = %v", err)
	}
	if got := receiverOut.String(); got != "relay-first offered payload" {
		t.Fatalf("receiver output = %q, want %q", got, "relay-first offered payload")
	}
}
