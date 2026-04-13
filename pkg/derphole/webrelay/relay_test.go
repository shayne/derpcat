package webrelay

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shayne/derpcat/pkg/derphole/webproto"
)

type fakeDirect struct {
	readyCh chan struct{}
	failCh  chan error
	recvCh  chan []byte
	sentMu  sync.Mutex
	sent    [][]byte
}

func newFakeDirect() *fakeDirect {
	return &fakeDirect{
		readyCh: make(chan struct{}),
		failCh:  make(chan error, 1),
		recvCh:  make(chan []byte, 16),
	}
}

func (d *fakeDirect) Start(context.Context, DirectRole, DirectSignalPeer) error { return nil }

func (d *fakeDirect) Ready() <-chan struct{} { return d.readyCh }

func (d *fakeDirect) Failed() <-chan error { return d.failCh }

func (d *fakeDirect) SendFrame(ctx context.Context, frame []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.sentMu.Lock()
	defer d.sentMu.Unlock()
	d.sent = append(d.sent, append([]byte(nil), frame...))
	return nil
}

func (d *fakeDirect) ReceiveFrames() <-chan []byte { return d.recvCh }

func (d *fakeDirect) Close() error { return nil }

func (d *fakeDirect) markReady() {
	close(d.readyCh)
}

func TestChooseRelayBeforeDirectReady(t *testing.T) {
	direct := newFakeDirect()
	path := chooseSendPath(TransferOptions{Direct: direct}, false)
	if path != sendPathRelay {
		t.Fatalf("path = %v, want %v", path, sendPathRelay)
	}
}

func TestChooseDirectAfterReady(t *testing.T) {
	direct := newFakeDirect()
	direct.markReady()
	path := chooseSendPath(TransferOptions{Direct: direct}, true)
	if path != sendPathDirect {
		t.Fatalf("path = %v, want %v", path, sendPathDirect)
	}
}

func TestDirectFailureBeforeSwitchKeepsRelay(t *testing.T) {
	direct := newFakeDirect()
	direct.failCh <- errors.New("ice failed")
	state := directState{}
	state.noteFailureBeforeSwitch(<-direct.Failed())
	if state.active {
		t.Fatalf("direct state active after pre-switch failure")
	}
	if state.fallbackReason != "ice failed" {
		t.Fatalf("fallbackReason = %q, want %q", state.fallbackReason, "ice failed")
	}
}

func TestMarshalDirectReadyUsesCurrentOffset(t *testing.T) {
	frame, err := marshalDirectReadyFrame(33, 4096)
	if err != nil {
		t.Fatalf("marshalDirectReadyFrame() error = %v", err)
	}
	if frame.Kind != webproto.FrameDirectReady {
		t.Fatalf("Kind = %v, want %v", frame.Kind, webproto.FrameDirectReady)
	}
	var payload webproto.DirectReady
	if err := unmarshalFramePayload(frame, &payload); err != nil {
		t.Fatalf("unmarshalFramePayload() error = %v", err)
	}
	if payload.NextSeq != 33 || payload.BytesReceived != 4096 {
		t.Fatalf("payload = %+v", payload)
	}
}
