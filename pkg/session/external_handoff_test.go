package session

import (
	"bytes"
	"testing"
)

func TestExternalHandoffReceiverWritesContiguousChunksInOrderAndDedupes(t *testing.T) {
	var out bytes.Buffer
	rx := newExternalHandoffReceiver(&out, 2<<20)

	if err := rx.AcceptChunk(externalHandoffChunk{TransferID: 7, Offset: 5, Payload: []byte("world")}); err != nil {
		t.Fatal(err)
	}
	if err := rx.AcceptChunk(externalHandoffChunk{TransferID: 7, Offset: 0, Payload: []byte("hello")}); err != nil {
		t.Fatal(err)
	}
	if err := rx.AcceptChunk(externalHandoffChunk{TransferID: 7, Offset: 0, Payload: []byte("hello")}); err != nil {
		t.Fatal(err)
	}

	if got := out.String(); got != "helloworld" {
		t.Fatalf("output = %q, want %q", got, "helloworld")
	}
	if got := rx.Watermark(); got != 10 {
		t.Fatalf("watermark = %d, want 10", got)
	}
}

func TestExternalHandoffReceiverRejectsWindowOverflow(t *testing.T) {
	var out bytes.Buffer
	rx := newExternalHandoffReceiver(&out, 8)

	err := rx.AcceptChunk(externalHandoffChunk{TransferID: 7, Offset: 1024, Payload: []byte("overflow")})
	if err == nil {
		t.Fatal("AcceptChunk() error = nil, want overflow rejection")
	}
}
