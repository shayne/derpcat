package probe

import (
	"crypto/cipher"
	"errors"
)

var errStreamReplayWindowFull = errors.New("stream replay window full")

const defaultStreamReplayWindowBytes = 256 << 20

type streamReplayWindow struct {
	runID       [16]byte
	maxBytes    uint64
	packetAEAD  cipher.AEAD
	packets     map[uint64][]byte
	packetBytes map[uint64]uint64
	retained    uint64
	maxRetained uint64
	ackFloor    uint64
}

func newStreamReplayWindow(runID [16]byte, chunkSize int, maxBytes uint64, packetAEAD cipher.AEAD) *streamReplayWindow {
	_ = chunkSize
	return &streamReplayWindow{
		runID:       runID,
		maxBytes:    maxBytes,
		packetAEAD:  packetAEAD,
		packets:     make(map[uint64][]byte),
		packetBytes: make(map[uint64]uint64),
	}
}

func (w *streamReplayWindow) AddDataPacket(stripeID uint16, seq uint64, offset uint64, payload []byte) ([]byte, error) {
	if w == nil {
		return nil, errors.New("nil stream replay window")
	}
	wire, err := marshalBlastPayloadPacket(PacketTypeData, w.runID, stripeID, seq, offset, 0, 0, payload, w.packetAEAD)
	if err != nil {
		return nil, err
	}
	packetBytes := uint64(len(wire))
	if w.maxBytes > 0 && w.retained+packetBytes > w.maxBytes {
		return nil, errStreamReplayWindowFull
	}
	w.packets[seq] = wire
	w.packetBytes[seq] = packetBytes
	w.retained += packetBytes
	w.maxRetained = max(w.maxRetained, w.retained)
	return wire, nil
}

func (w *streamReplayWindow) AckFloor(seq uint64) {
	if w == nil || seq <= w.ackFloor {
		return
	}
	for acked := w.ackFloor; acked < seq; acked++ {
		w.delete(acked)
	}
	w.ackFloor = seq
}

func (w *streamReplayWindow) Packet(seq uint64) []byte {
	if w == nil {
		return nil
	}
	return w.packets[seq]
}

func (w *streamReplayWindow) RetainedBytes() uint64 {
	if w == nil {
		return 0
	}
	return w.retained
}

func (w *streamReplayWindow) MaxRetainedBytes() uint64 {
	if w == nil {
		return 0
	}
	return w.maxRetained
}

func (w *streamReplayWindow) MaxBytes() uint64 {
	if w == nil {
		return 0
	}
	return w.maxBytes
}

func (w *streamReplayWindow) delete(seq uint64) {
	packetBytes, ok := w.packetBytes[seq]
	if !ok {
		return
	}
	delete(w.packetBytes, seq)
	delete(w.packets, seq)
	if packetBytes >= w.retained {
		w.retained = 0
		return
	}
	w.retained -= packetBytes
}
