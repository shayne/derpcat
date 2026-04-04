package session

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

var errExternalHandoffWindowOverflow = errors.New("external handoff receive window overflow")

type externalHandoffChunk struct {
	TransferID uint64
	Offset     int64
	Payload    []byte
}

type externalHandoffReceiver struct {
	out       io.Writer
	maxWindow int64
	watermark int64
	pending   map[int64][]byte
	buffered  int64
}

func newExternalHandoffReceiver(out io.Writer, maxWindow int64) *externalHandoffReceiver {
	return &externalHandoffReceiver{
		out:       out,
		maxWindow: maxWindow,
		pending:   make(map[int64][]byte),
	}
}

func (r *externalHandoffReceiver) Watermark() int64 {
	return r.watermark
}

func (r *externalHandoffReceiver) AcceptChunk(chunk externalHandoffChunk) error {
	if chunk.Offset < 0 {
		return fmt.Errorf("external handoff chunk offset %d is negative", chunk.Offset)
	}
	if len(chunk.Payload) == 0 {
		return nil
	}

	offset := chunk.Offset
	payload := chunk.Payload
	end := offset + int64(len(payload))
	if end <= r.watermark {
		return nil
	}
	if offset < r.watermark {
		payload = payload[r.watermark-offset:]
		offset = r.watermark
	}

	if offset > r.watermark+r.maxWindow {
		return errExternalHandoffWindowOverflow
	}
	if pending, ok := r.pending[offset]; ok {
		if !bytes.Equal(pending, payload) {
			return fmt.Errorf("external handoff duplicate chunk at offset %d does not match buffered payload", offset)
		}
		return nil
	}
	if r.buffered+int64(len(payload)) > r.maxWindow {
		return errExternalHandoffWindowOverflow
	}

	copied := append([]byte(nil), payload...)
	r.pending[offset] = copied
	r.buffered += int64(len(copied))

	for {
		next, ok := r.pending[r.watermark]
		if !ok {
			return nil
		}
		if _, err := r.out.Write(next); err != nil {
			return err
		}
		delete(r.pending, r.watermark)
		r.buffered -= int64(len(next))
		r.watermark += int64(len(next))
	}
}
