package session

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

var errExternalHandoffWindowOverflow = errors.New("external handoff receive window overflow")
var errExternalHandoffUnackedWindowFull = errors.New("external handoff unacked window full")

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

type externalHandoffSpool struct {
	src            io.Reader
	file           *os.File
	filePath       string
	chunkSize      int
	maxUnacked     int64
	readOffset     int64
	sourceOffset   int64
	ackedWatermark int64
	eof            bool
}

func newExternalHandoffSpool(src io.Reader, chunkSize int, maxUnackedBytes int64) (*externalHandoffSpool, error) {
	if src == nil {
		return nil, errors.New("external handoff spool source is nil")
	}
	if chunkSize <= 0 {
		return nil, fmt.Errorf("external handoff chunk size %d must be positive", chunkSize)
	}
	if maxUnackedBytes <= 0 {
		return nil, fmt.Errorf("external handoff unacked window %d must be positive", maxUnackedBytes)
	}
	file, err := os.CreateTemp("", "derpcat-external-handoff-*.spool")
	if err != nil {
		return nil, err
	}
	return &externalHandoffSpool{
		src:        src,
		file:       file,
		filePath:   file.Name(),
		chunkSize:  chunkSize,
		maxUnacked: maxUnackedBytes,
	}, nil
}

func (s *externalHandoffSpool) NextChunk() (externalHandoffChunk, error) {
	if s.readOffset-s.ackedWatermark >= s.maxUnacked {
		return externalHandoffChunk{}, errExternalHandoffUnackedWindowFull
	}
	chunkLen := int64(s.chunkSize)
	if remaining := s.maxUnacked - (s.readOffset - s.ackedWatermark); remaining < chunkLen {
		chunkLen = remaining
	}
	if chunkLen <= 0 {
		return externalHandoffChunk{}, errExternalHandoffUnackedWindowFull
	}

	if s.readOffset < s.sourceOffset {
		available := s.sourceOffset - s.readOffset
		if available < chunkLen {
			chunkLen = available
		}
		payload := make([]byte, chunkLen)
		n, err := s.file.ReadAt(payload, s.readOffset)
		if err != nil && !errors.Is(err, io.EOF) {
			return externalHandoffChunk{}, err
		}
		payload = payload[:n]
		chunk := externalHandoffChunk{Offset: s.readOffset, Payload: payload}
		s.readOffset += int64(n)
		return chunk, nil
	}

	if s.eof {
		return externalHandoffChunk{}, io.EOF
	}

	payload := make([]byte, chunkLen)
	n, err := io.ReadFull(s.src, payload)
	switch {
	case err == nil:
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		s.eof = true
		if n == 0 {
			return externalHandoffChunk{}, io.EOF
		}
	default:
		return externalHandoffChunk{}, err
	}
	payload = payload[:n]
	if _, err := s.file.WriteAt(payload, s.sourceOffset); err != nil {
		return externalHandoffChunk{}, err
	}
	chunk := externalHandoffChunk{Offset: s.sourceOffset, Payload: payload}
	s.sourceOffset += int64(len(payload))
	s.readOffset = s.sourceOffset
	return chunk, nil
}

func (s *externalHandoffSpool) AckTo(watermark int64) error {
	if watermark < s.ackedWatermark {
		return fmt.Errorf("external handoff ack watermark %d moved backward from %d", watermark, s.ackedWatermark)
	}
	if watermark > s.sourceOffset {
		return fmt.Errorf("external handoff ack watermark %d exceeds source offset %d", watermark, s.sourceOffset)
	}
	s.ackedWatermark = watermark
	return nil
}

func (s *externalHandoffSpool) RewindTo(offset int64) error {
	if offset < s.ackedWatermark {
		return fmt.Errorf("external handoff rewind offset %d precedes ack watermark %d", offset, s.ackedWatermark)
	}
	if offset > s.sourceOffset {
		return fmt.Errorf("external handoff rewind offset %d exceeds source offset %d", offset, s.sourceOffset)
	}
	s.readOffset = offset
	return nil
}

func (s *externalHandoffSpool) Close() error {
	if s == nil || s.file == nil {
		return nil
	}
	err := s.file.Close()
	removeErr := os.Remove(s.filePath)
	s.file = nil
	if err != nil {
		return err
	}
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return removeErr
	}
	return nil
}
