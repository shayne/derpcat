package probe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"time"
)

const (
	defaultChunkSize     = 1200
	defaultWindowSize    = 8
	defaultRetryInterval = 20 * time.Millisecond
	maxAckMaskBits       = 64
)

type SendConfig struct {
	Raw        bool
	ChunkSize  int
	WindowSize int
}

type ReceiveConfig struct {
	Raw bool
}

type TransferStats struct {
	BytesSent     int64
	BytesReceived int64
	PacketsSent   int64
	PacketsAcked  int64
	StartedAt     time.Time
	CompletedAt   time.Time
}

func Send(ctx context.Context, conn net.PacketConn, remoteAddr string, src io.Reader, cfg SendConfig) (TransferStats, error) {
	if conn == nil {
		return TransferStats{}, errors.New("nil packet conn")
	}
	if src == nil {
		return TransferStats{}, errors.New("nil source reader")
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = defaultChunkSize
	}
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = defaultWindowSize
	}
	if cfg.WindowSize > maxAckMaskBits+1 {
		cfg.WindowSize = maxAckMaskBits + 1
	}

	peer, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		return TransferStats{}, err
	}

	packets, totalBytes, err := buildSendPackets(src, cfg.ChunkSize)
	if err != nil {
		return TransferStats{}, err
	}

	stats := TransferStats{
		BytesSent: totalBytes,
		StartedAt: time.Now(),
	}
	if len(packets) == 0 {
		stats.CompletedAt = stats.StartedAt
		return stats, nil
	}

	buf := make([]byte, 64<<10)
	base := 0
	nextToSend := 0

	for base < len(packets) {
		for nextToSend < len(packets) && nextToSend-base < cfg.WindowSize {
			if packets[nextToSend].sentAt.IsZero() {
				if err := sendOutbound(ctx, conn, peer, &packets[nextToSend], &stats); err != nil {
					return TransferStats{}, err
				}
			}
			nextToSend++
		}

		if err := setReadDeadline(ctx, conn, nextRetransmitDeadline(ctx, packets[base:nextToSend])); err != nil {
			return TransferStats{}, err
		}

		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return TransferStats{}, ctx.Err()
			}
			if isNetTimeout(err) {
				if err := retransmitExpired(ctx, conn, peer, packets[base:nextToSend], &stats); err != nil {
					return TransferStats{}, err
				}
				continue
			}
			return TransferStats{}, err
		}
		if addr.String() != peer.String() {
			continue
		}

		packet, err := UnmarshalPacket(buf[:n], nil)
		if err != nil {
			return TransferStats{}, err
		}
		if packet.Type != PacketTypeAck {
			continue
		}

		stats.PacketsAcked += int64(applyAck(packets, packet.AckFloor, packet.AckMask))
		for base < len(packets) && packets[base].acked {
			base++
		}
	}

	stats.CompletedAt = time.Now()
	return stats, nil
}

func Receive(ctx context.Context, conn net.PacketConn, remoteAddr string, cfg ReceiveConfig) ([]byte, error) {
	if conn == nil {
		return nil, errors.New("nil packet conn")
	}

	peer, err := resolveRemoteAddr(remoteAddr)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	buf := make([]byte, 64<<10)
	var expectedSeq uint64
	buffered := make(map[uint64]Packet)

	for {
		if err := setReadDeadline(ctx, conn, defaultRetryInterval); err != nil {
			return nil, err
		}

		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if isNetTimeout(err) {
				continue
			}
			if errors.Is(err, net.ErrClosed) {
				return nil, err
			}
			continue
		}
		if peer != nil && addr.String() != peer.String() {
			continue
		}

		packet, err := UnmarshalPacket(buf[:n], nil)
		if err != nil {
			return nil, err
		}

		switch packet.Type {
		case PacketTypeData:
			if packet.Seq >= expectedSeq && packet.Seq <= expectedSeq+maxAckMaskBits {
				buffered[packet.Seq] = clonePacket(packet)
			}
			var complete bool
			expectedSeq, complete, err = advanceReceiveWindow(&out, buffered, expectedSeq)
			if err != nil {
				return nil, err
			}
			if err := sendAck(ctx, conn, addr, expectedSeq, ackMaskFor(buffered, expectedSeq)); err != nil {
				return nil, err
			}
			if complete {
				return out.Bytes(), nil
			}
		case PacketTypeDone:
			if packet.Seq >= expectedSeq && packet.Seq <= expectedSeq+maxAckMaskBits {
				buffered[packet.Seq] = clonePacket(packet)
			}
			var complete bool
			expectedSeq, complete, err = advanceReceiveWindow(&out, buffered, expectedSeq)
			if err != nil {
				return nil, err
			}
			if err := sendAck(ctx, conn, addr, expectedSeq, ackMaskFor(buffered, expectedSeq)); err != nil {
				return nil, err
			}
			if complete {
				return out.Bytes(), nil
			}
		}
	}
}

func sendAck(ctx context.Context, conn net.PacketConn, peer net.Addr, ackFloor, ackMask uint64) error {
	packet, err := MarshalPacket(Packet{
		Version:  ProtocolVersion,
		Type:     PacketTypeAck,
		AckFloor: ackFloor,
		AckMask:  ackMask,
	}, nil)
	if err != nil {
		return err
	}
	_, err = writeWithContext(ctx, conn, peer, packet)
	return err
}

type outboundPacket struct {
	seq     uint64
	wire    []byte
	sentAt  time.Time
	acked   bool
	payload int
}

func buildSendPackets(src io.Reader, chunkSize int) ([]outboundPacket, int64, error) {
	buf := make([]byte, chunkSize)
	var packets []outboundPacket
	var seq uint64
	var offset uint64
	var totalBytes int64

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			payload := append([]byte(nil), buf[:n]...)
			wire, err := MarshalPacket(Packet{
				Version: ProtocolVersion,
				Type:    PacketTypeData,
				Seq:     seq,
				Offset:  offset,
				Payload: payload,
			}, nil)
			if err != nil {
				return nil, 0, err
			}
			packets = append(packets, outboundPacket{
				seq:     seq,
				wire:    wire,
				payload: n,
			})
			totalBytes += int64(n)
			seq++
			offset += uint64(n)
		}
		if errors.Is(readErr, io.EOF) {
			wire, err := MarshalPacket(Packet{
				Version: ProtocolVersion,
				Type:    PacketTypeDone,
				Seq:     seq,
				Offset:  offset,
			}, nil)
			if err != nil {
				return nil, 0, err
			}
			packets = append(packets, outboundPacket{
				seq:  seq,
				wire: wire,
			})
			return packets, totalBytes, nil
		}
		if readErr != nil {
			return nil, 0, readErr
		}
	}
}

func sendOutbound(ctx context.Context, conn net.PacketConn, peer net.Addr, packet *outboundPacket, stats *TransferStats) error {
	_, err := writeWithContext(ctx, conn, peer, packet.wire)
	if err != nil {
		return err
	}
	packet.sentAt = time.Now()
	stats.PacketsSent++
	return nil
}

func retransmitExpired(ctx context.Context, conn net.PacketConn, peer net.Addr, packets []outboundPacket, stats *TransferStats) error {
	now := time.Now()
	for i := range packets {
		if packets[i].acked || packets[i].sentAt.IsZero() {
			continue
		}
		if now.Sub(packets[i].sentAt) < defaultRetryInterval {
			continue
		}
		if err := sendOutbound(ctx, conn, peer, &packets[i], stats); err != nil {
			return err
		}
	}
	return nil
}

func nextRetransmitDeadline(ctx context.Context, packets []outboundPacket) time.Duration {
	wait := defaultRetryInterval
	now := time.Now()
	for _, packet := range packets {
		if packet.acked || packet.sentAt.IsZero() {
			continue
		}
		deadline := packet.sentAt.Add(defaultRetryInterval)
		if deadline.Before(now) {
			return 0
		}
		packetWait := deadline.Sub(now)
		if packetWait < wait {
			wait = packetWait
		}
	}
	if ctxDeadline, ok := ctx.Deadline(); ok {
		ctxWait := time.Until(ctxDeadline)
		if ctxWait < wait {
			return ctxWait
		}
	}
	return wait
}

func applyAck(packets []outboundPacket, ackFloor, ackMask uint64) int {
	acked := 0
	for i := range packets {
		if packets[i].acked {
			continue
		}
		if packets[i].seq < ackFloor {
			packets[i].acked = true
			acked++
			continue
		}
		if packets[i].seq <= ackFloor {
			continue
		}
		delta := packets[i].seq - ackFloor - 1
		if delta >= maxAckMaskBits {
			continue
		}
		if ackMask&(uint64(1)<<delta) == 0 {
			continue
		}
		packets[i].acked = true
		acked++
	}
	return acked
}

func advanceReceiveWindow(out *bytes.Buffer, buffered map[uint64]Packet, expectedSeq uint64) (uint64, bool, error) {
	for {
		packet, ok := buffered[expectedSeq]
		if !ok {
			return expectedSeq, false, nil
		}
		delete(buffered, expectedSeq)
		switch packet.Type {
		case PacketTypeData:
			if _, err := out.Write(packet.Payload); err != nil {
				return expectedSeq, false, err
			}
			expectedSeq++
		case PacketTypeDone:
			return expectedSeq + 1, true, nil
		default:
			return expectedSeq, false, nil
		}
	}
}

func ackMaskFor(buffered map[uint64]Packet, ackFloor uint64) uint64 {
	var mask uint64
	for seq := range buffered {
		if seq <= ackFloor || seq > ackFloor+maxAckMaskBits {
			continue
		}
		mask |= uint64(1) << (seq - ackFloor - 1)
	}
	return mask
}

func clonePacket(packet Packet) Packet {
	packet.Payload = append([]byte(nil), packet.Payload...)
	return packet
}

func writeWithContext(ctx context.Context, conn net.PacketConn, peer net.Addr, packet []byte) (int, error) {
	deadline, err := writeDeadline(ctx)
	if err != nil {
		return 0, err
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return 0, err
	}
	return conn.WriteTo(packet, peer)
}

func resolveRemoteAddr(remoteAddr string) (net.Addr, error) {
	if remoteAddr == "" {
		return nil, nil
	}
	return net.ResolveUDPAddr("udp", remoteAddr)
}

func setReadDeadline(ctx context.Context, conn net.PacketConn, fallback time.Duration) error {
	deadline := time.Now().Add(fallback)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	return conn.SetReadDeadline(deadline)
}

func writeDeadline(ctx context.Context) (time.Time, error) {
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		return deadline, nil
	}
	return time.Now().Add(defaultRetryInterval), nil
}

func isNetTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
