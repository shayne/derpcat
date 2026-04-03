package session

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"time"

	quic "github.com/quic-go/quic-go"
	"github.com/shayne/derpcat/pkg/quicpath"
	"github.com/shayne/derpcat/pkg/telemetry"
	"tailscale.com/tailcfg"
)

type externalNativeQUICConnResult struct {
	conn   *quic.Conn
	dialed bool
	err    error
}

var errExternalNativeQUICNoMatchingStripeCandidate = errors.New("no matching native QUIC stripe candidate")

var externalNativeQUICStripeProbeCandidates = publicProbeCandidates

type externalNativeQUICStripedSession struct {
	packetConns   []net.PacketConn
	transports    []*quic.Transport
	conns         []*quic.Conn
	portmaps      []publicPortmap
	primaryStream *quic.Stream
}

type externalNativeQUICStripeSetup struct {
	CandidateSets [][]string `json:"candidate_sets"`
}

type externalNativeQUICStripeSetupResult struct {
	Ready bool `json:"ready"`
}

func (s *externalNativeQUICStripedSession) Close() {
	if s == nil {
		return
	}
	closeExternalNativeQUICConns(s.conns)
	for _, transport := range s.transports {
		_ = transport.Close()
	}
	for _, pm := range s.portmaps {
		_ = pm.Close()
	}
	for _, packetConn := range s.packetConns {
		_ = packetConn.Close()
	}
}

func (s *externalNativeQUICStripedSession) OpenStreams(ctx context.Context) ([]io.WriteCloser, error) {
	if s.primaryStream != nil {
		return []io.WriteCloser{s.primaryStream}, nil
	}
	streams := make([]io.WriteCloser, 0, len(s.conns))
	for _, conn := range s.conns {
		stream, err := conn.OpenStreamSync(ctx)
		if err != nil {
			closeExternalStripedWriters(streams)
			return nil, err
		}
		streams = append(streams, stream)
	}
	return streams, nil
}

func dialExternalNativeQUICStripedConns(
	ctx context.Context,
	packetConn net.PacketConn,
	peerAddr net.Addr,
	dm *tailcfg.DERPMap,
	emitter *telemetry.Emitter,
	clientTLS *tls.Config,
	serverTLS *tls.Config,
	connCount int,
) (*externalNativeQUICStripedSession, error) {
	if connCount < 1 {
		return nil, errors.New("native QUIC connection count must be positive")
	}

	primaryTransport, primaryConn, err := dialOrAcceptExternalNativeQUICConn(
		ctx,
		packetConn,
		peerAddr,
		clientTLS,
		serverTLS,
	)
	if err != nil {
		return nil, err
	}
	session := &externalNativeQUICStripedSession{
		packetConns: []net.PacketConn{packetConn},
		transports:  []*quic.Transport{primaryTransport},
		conns:       []*quic.Conn{primaryConn},
	}
	if connCount == 1 {
		if emitter != nil {
			emitter.Debug("native-quic-primary-only")
		}
		return session, nil
	}

	setupCtx, setupCancel := context.WithTimeout(ctx, externalNativeQUICWait)
	defer setupCancel()

	controlStream, err := primaryConn.OpenStreamSync(setupCtx)
	if err != nil {
		if emitter != nil {
			emitter.Debug("native-quic-primary-fallback=open-control-stream")
		}
		return session, nil
	}
	keepControlStream := false
	defer func() {
		if keepControlStream {
			return
		}
		_ = controlStream.SetDeadline(time.Time{})
		_ = controlStream.Close()
	}()
	_ = controlStream.SetDeadline(time.Now().Add(externalNativeQUICWait))

	localPacketConns, localPortmaps, localCandidateSets, err := openExternalNativeQUICStripePacketConns(setupCtx, dm, emitter, connCount-1)
	if err != nil {
		if emitter != nil {
			emitter.Debug("native-quic-primary-fallback=open-stripe-sockets")
		}
		return session, nil
	}

	if err := json.NewEncoder(controlStream).Encode(externalNativeQUICStripeSetup{CandidateSets: localCandidateSets}); err != nil {
		closeExternalNativeQUICStripePacketConns(localPacketConns, localPortmaps)
		if emitter != nil {
			emitter.Debug("native-quic-primary-fallback=encode-stripe-setup")
		}
		return session, nil
	}
	var peerSetup externalNativeQUICStripeSetup
	if err := json.NewDecoder(controlStream).Decode(&peerSetup); err != nil {
		closeExternalNativeQUICStripePacketConns(localPacketConns, localPortmaps)
		if emitter != nil {
			emitter.Debug("native-quic-primary-fallback=decode-stripe-setup")
		}
		return session, nil
	}
	if len(peerSetup.CandidateSets) != connCount-1 {
		closeExternalNativeQUICStripePacketConns(localPacketConns, localPortmaps)
		if emitter != nil {
			emitter.Debug("native-quic-primary-fallback=stripe-setup-size")
		}
		return session, nil
	}

	extraTransports := make([]*quic.Transport, 0, len(localPacketConns))
	extraConns := make([]*quic.Conn, 0, len(localPacketConns))
	stripeReady := true
	for i, localPacketConn := range localPacketConns {
		stripePeerAddr, err := selectExternalNativeQUICPeerAddr(peerAddr, peerSetup.CandidateSets[i])
		if err != nil {
			stripeReady = false
			if emitter != nil {
				emitter.Debug("native-quic-primary-fallback=select-stripe-peer")
			}
			break
		}
		transport, conn, err := dialOrAcceptExternalNativeQUICConn(
			setupCtx,
			localPacketConn,
			stripePeerAddr,
			clientTLS,
			serverTLS,
		)
		if err != nil {
			stripeReady = false
			if emitter != nil {
				emitter.Debug("native-quic-primary-fallback=dial-stripe-conn")
			}
			break
		}
		extraTransports = append(extraTransports, transport)
		extraConns = append(extraConns, conn)
	}
	if err := json.NewEncoder(controlStream).Encode(externalNativeQUICStripeSetupResult{Ready: stripeReady}); err != nil {
		closeExternalNativeQUICConns(extraConns)
		for _, transport := range extraTransports {
			_ = transport.Close()
		}
		closeExternalNativeQUICStripePacketConns(localPacketConns, localPortmaps)
		if emitter != nil {
			emitter.Debug("native-quic-primary-fallback=encode-stripe-ready")
		}
		return session, nil
	}
	var peerReady externalNativeQUICStripeSetupResult
	if err := json.NewDecoder(controlStream).Decode(&peerReady); err != nil || !stripeReady || !peerReady.Ready {
		closeExternalNativeQUICConns(extraConns)
		for _, transport := range extraTransports {
			_ = transport.Close()
		}
		closeExternalNativeQUICStripePacketConns(localPacketConns, localPortmaps)
		if emitter != nil {
			emitter.Debug("native-quic-primary-fallback=final")
		}
		return session, nil
	}

	session.packetConns = append(session.packetConns, localPacketConns...)
	session.portmaps = append(session.portmaps, localPortmaps...)
	session.transports = append(session.transports, extraTransports...)
	session.conns = append(session.conns, extraConns...)
	if emitter != nil {
		emitter.Debug("native-quic-stripes=4")
	}

	return session, nil
}

func acceptExternalNativeQUICStripedConns(
	ctx context.Context,
	packetConn net.PacketConn,
	peerAddr net.Addr,
	dm *tailcfg.DERPMap,
	emitter *telemetry.Emitter,
	clientTLS *tls.Config,
	serverTLS *tls.Config,
	connCount int,
) (*externalNativeQUICStripedSession, []*quic.Stream, error) {
	if connCount < 1 {
		return nil, nil, errors.New("native QUIC connection count must be positive")
	}

	transport, conns, err := dialOrAcceptExternalNativeQUICConnsWithRole(
		ctx,
		packetConn,
		peerAddr,
		clientTLS,
		serverTLS,
		1,
		false,
	)
	if err != nil {
		return nil, nil, err
	}
	session := &externalNativeQUICStripedSession{
		packetConns: []net.PacketConn{packetConn},
		transports:  []*quic.Transport{transport},
		conns:       conns,
	}
	if connCount > 1 {
		setupCtx, setupCancel := context.WithTimeout(ctx, externalNativeQUICWait)
		defer setupCancel()

		controlStream, err := conns[0].AcceptStream(setupCtx)
		if err != nil {
			stream, streamErr := conns[0].AcceptStream(ctx)
			if streamErr != nil {
				session.Close()
				return nil, nil, streamErr
			}
			return session, []*quic.Stream{stream}, nil
		}
		keepControlStream := false
		defer func() {
			if keepControlStream {
				return
			}
			_ = controlStream.SetDeadline(time.Time{})
			_ = controlStream.Close()
		}()
		_ = controlStream.SetDeadline(time.Now().Add(externalNativeQUICWait))

		var peerSetup externalNativeQUICStripeSetup
		if err := json.NewDecoder(controlStream).Decode(&peerSetup); err != nil {
			if emitter != nil {
				emitter.Debug("native-quic-primary-fallback=decode-stripe-setup")
			}
			stream, streamErr := conns[0].AcceptStream(ctx)
			if streamErr != nil {
				session.Close()
				return nil, nil, streamErr
			}
			return session, []*quic.Stream{stream}, nil
		}
		if len(peerSetup.CandidateSets) != connCount-1 {
			if emitter != nil {
				emitter.Debug("native-quic-primary-fallback=stripe-setup-size")
			}
			stream, streamErr := conns[0].AcceptStream(ctx)
			if streamErr != nil {
				session.Close()
				return nil, nil, streamErr
			}
			return session, []*quic.Stream{stream}, nil
		}

		localPacketConns, localPortmaps, localCandidateSets, err := openExternalNativeQUICStripePacketConns(setupCtx, dm, emitter, connCount-1)
		if err != nil {
			if emitter != nil {
				emitter.Debug("native-quic-primary-fallback=open-stripe-sockets")
			}
			stream, streamErr := conns[0].AcceptStream(ctx)
			if streamErr != nil {
				session.Close()
				return nil, nil, streamErr
			}
			return session, []*quic.Stream{stream}, nil
		}

		if err := json.NewEncoder(controlStream).Encode(externalNativeQUICStripeSetup{CandidateSets: localCandidateSets}); err != nil {
			closeExternalNativeQUICStripePacketConns(localPacketConns, localPortmaps)
			if emitter != nil {
				emitter.Debug("native-quic-primary-fallback=encode-stripe-setup")
			}
			stream, streamErr := conns[0].AcceptStream(ctx)
			if streamErr != nil {
				session.Close()
				return nil, nil, streamErr
			}
			return session, []*quic.Stream{stream}, nil
		}

		extraTransports := make([]*quic.Transport, 0, len(localPacketConns))
		extraConns := make([]*quic.Conn, 0, len(localPacketConns))
		stripeReady := true
		for i, localPacketConn := range localPacketConns {
			stripePeerAddr, err := selectExternalNativeQUICPeerAddr(peerAddr, peerSetup.CandidateSets[i])
			if err != nil {
				stripeReady = false
				break
			}
			transport, conns, err := dialOrAcceptExternalNativeQUICConnsWithRole(
				setupCtx,
				localPacketConn,
				stripePeerAddr,
				clientTLS,
				serverTLS,
				1,
				false,
			)
			if err != nil {
				stripeReady = false
				break
			}
			extraTransports = append(extraTransports, transport)
			extraConns = append(extraConns, conns[0])
		}
		var peerReady externalNativeQUICStripeSetupResult
		if err := json.NewDecoder(controlStream).Decode(&peerReady); err != nil {
			stripeReady = false
		}
		if err := json.NewEncoder(controlStream).Encode(externalNativeQUICStripeSetupResult{Ready: stripeReady}); err != nil || !stripeReady || !peerReady.Ready {
			closeExternalNativeQUICConns(extraConns)
			for _, transport := range extraTransports {
				_ = transport.Close()
			}
			closeExternalNativeQUICStripePacketConns(localPacketConns, localPortmaps)
			if emitter != nil {
				emitter.Debug("native-quic-primary-fallback")
			}
			stream, streamErr := conns[0].AcceptStream(ctx)
			if streamErr != nil {
				session.Close()
				return nil, nil, streamErr
			}
			return session, []*quic.Stream{stream}, nil
		}

		session.packetConns = append(session.packetConns, localPacketConns...)
		session.portmaps = append(session.portmaps, localPortmaps...)
		session.transports = append(session.transports, extraTransports...)
		session.conns = append(session.conns, extraConns...)
		if emitter != nil {
			emitter.Debug("native-quic-stripes=4")
		}
	}

	streams := make([]*quic.Stream, 0, len(session.conns))
	for _, conn := range session.conns {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			closeExternalNativeQUICStreams(streams)
			session.Close()
			return nil, nil, err
		}
		streams = append(streams, stream)
	}
	return session, streams, nil
}

func openExternalNativeQUICStripePacketConns(
	ctx context.Context,
	dm *tailcfg.DERPMap,
	emitter *telemetry.Emitter,
	count int,
) ([]net.PacketConn, []publicPortmap, [][]string, error) {
	packetConns := make([]net.PacketConn, 0, count)
	portmaps := make([]publicPortmap, 0, count)
	candidateSets := make([][]string, 0, count)
	for range count {
		packetConn, err := net.ListenPacket("udp4", ":0")
		if err != nil {
			closeExternalNativeQUICStripePacketConns(packetConns, portmaps)
			return nil, nil, nil, err
		}
		pm := newBoundPublicPortmap(packetConn, emitter)
		packetConns = append(packetConns, packetConn)
		portmaps = append(portmaps, pm)
		candidateSets = append(candidateSets, externalNativeQUICStripeProbeCandidates(ctx, packetConn, dm, pm))
	}
	return packetConns, portmaps, candidateSets, nil
}

func closeExternalNativeQUICStripePacketConns(packetConns []net.PacketConn, portmaps []publicPortmap) {
	for _, pm := range portmaps {
		_ = pm.Close()
	}
	for _, packetConn := range packetConns {
		_ = packetConn.Close()
	}
}

func selectExternalNativeQUICPeerAddr(primaryPeerAddr net.Addr, candidates []string) (net.Addr, error) {
	parsed := parseCandidateStrings(candidates)
	if len(parsed) == 0 {
		if primaryPeerAddr != nil {
			return cloneSessionAddr(primaryPeerAddr), nil
		}
		return nil, errors.New("no native QUIC peer candidates")
	}
	primaryUDPAddr, ok := primaryPeerAddr.(*net.UDPAddr)
	if !ok || primaryUDPAddr == nil {
		return cloneSessionAddr(parsed[0]), nil
	}
	primaryIP, ok := netip.AddrFromSlice(primaryUDPAddr.IP)
	if !ok {
		return cloneSessionAddr(parsed[0]), nil
	}
	primaryIP = primaryIP.Unmap()
	for _, candidate := range parsed {
		candidateUDPAddr, ok := candidate.(*net.UDPAddr)
		if !ok || candidateUDPAddr == nil {
			continue
		}
		candidateIP, ok := netip.AddrFromSlice(candidateUDPAddr.IP)
		if !ok {
			continue
		}
		if candidateIP.Unmap() == primaryIP {
			return cloneSessionAddr(candidate), nil
		}
	}
	return nil, errExternalNativeQUICNoMatchingStripeCandidate
}

func dialOrAcceptExternalNativeQUICConn(
	ctx context.Context,
	packetConn net.PacketConn,
	peerAddr net.Addr,
	clientTLS *tls.Config,
	serverTLS *tls.Config,
) (*quic.Transport, *quic.Conn, error) {
	transport, conns, err := dialOrAcceptExternalNativeQUICConns(ctx, packetConn, peerAddr, clientTLS, serverTLS, 1)
	if err != nil {
		return nil, nil, err
	}
	return transport, conns[0], nil
}

func dialOrAcceptExternalNativeQUICConns(
	ctx context.Context,
	packetConn net.PacketConn,
	peerAddr net.Addr,
	clientTLS *tls.Config,
	serverTLS *tls.Config,
	connCount int,
) (*quic.Transport, []*quic.Conn, error) {
	return dialOrAcceptExternalNativeQUICConnsWithRole(ctx, packetConn, peerAddr, clientTLS, serverTLS, connCount, true)
}

func dialOrAcceptExternalNativeQUICConnsWithRole(
	ctx context.Context,
	packetConn net.PacketConn,
	peerAddr net.Addr,
	clientTLS *tls.Config,
	serverTLS *tls.Config,
	connCount int,
	preferDial bool,
) (*quic.Transport, []*quic.Conn, error) {
	if connCount < 1 {
		return nil, nil, errors.New("native QUIC connection count must be positive")
	}
	transport, listener, err := startExternalNativeQUICTransport(packetConn, serverTLS)
	if err != nil {
		return nil, nil, err
	}

	connectCtx, cancel := context.WithCancel(ctx)
	firstConn, dialRemainder, err := dialOrAcceptExternalNativeQUICConnOnTransport(
		connectCtx,
		transport,
		listener,
		peerAddr,
		clientTLS,
		preferDial,
	)
	if err != nil {
		cancel()
		_ = listener.Close()
		_ = transport.Close()
		return nil, nil, err
	}

	conns := []*quic.Conn{firstConn}
	for len(conns) < connCount {
		var nextConn *quic.Conn
		if dialRemainder {
			nextConn, err = transport.Dial(connectCtx, peerAddr, clientTLS, quicpath.DefaultQUICConfig())
		} else {
			nextConn, err = listener.Accept(connectCtx)
		}
		if err != nil {
			cancel()
			_ = listener.Close()
			closeExternalNativeQUICConns(conns)
			_ = transport.Close()
			return nil, nil, err
		}
		conns = append(conns, nextConn)
	}

	cancel()
	_ = listener.Close()
	return transport, conns, nil
}

func dialOrAcceptExternalNativeQUICConnOnTransport(
	ctx context.Context,
	transport *quic.Transport,
	listener *quic.Listener,
	peerAddr net.Addr,
	clientTLS *tls.Config,
	preferDial bool,
) (*quic.Conn, bool, error) {
	connectCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan externalNativeQUICConnResult, 2)
	pendingResults := 1
	if peerAddr != nil {
		pendingResults++
		go func() {
			conn, err := transport.Dial(connectCtx, peerAddr, clientTLS, quicpath.DefaultQUICConfig())
			results <- externalNativeQUICConnResult{conn: conn, dialed: true, err: err}
		}()
	}
	go func() {
		conn, err := listener.Accept(connectCtx)
		results <- externalNativeQUICConnResult{conn: conn, err: err}
	}()

	var firstErr error
	var firstConn *quic.Conn
	var firstDialed bool
	for range pendingResults {
		result := <-results
		if result.err == nil {
			if firstConn == nil {
				firstConn = result.conn
				firstDialed = result.dialed
				continue
			}
			cancel()
			if result.dialed == preferDial {
				_ = firstConn.CloseWithError(0, "")
				return result.conn, preferDial, nil
			}
			_ = result.conn.CloseWithError(0, "")
			return firstConn, preferDial, nil
		}
		if firstErr == nil {
			firstErr = result.err
		}
	}
	if firstConn != nil {
		cancel()
		return firstConn, firstDialed, nil
	}

	cancel()
	if firstErr == nil {
		firstErr = errors.New("native QUIC connection unavailable")
	}
	return nil, false, firstErr
}

func acceptExternalNativeQUICStream(
	ctx context.Context,
	packetConn net.PacketConn,
	peerAddr net.Addr,
	clientTLS *tls.Config,
	serverTLS *tls.Config,
) (*quic.Transport, *quic.Conn, *quic.Stream, error) {
	transport, conns, streams, err := acceptExternalNativeQUICStreams(ctx, packetConn, peerAddr, clientTLS, serverTLS, 1)
	if err != nil {
		return nil, nil, nil, err
	}
	return transport, conns[0], streams[0], nil
}

func acceptExternalNativeQUICStreams(
	ctx context.Context,
	packetConn net.PacketConn,
	peerAddr net.Addr,
	clientTLS *tls.Config,
	serverTLS *tls.Config,
	streamCount int,
) (*quic.Transport, []*quic.Conn, []*quic.Stream, error) {
	transport, conns, err := dialOrAcceptExternalNativeQUICConnsWithRole(ctx, packetConn, peerAddr, clientTLS, serverTLS, streamCount, false)
	if err != nil {
		return nil, nil, nil, err
	}

	streams := make([]*quic.Stream, 0, len(conns))
	for _, conn := range conns {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			closeExternalNativeQUICStreams(streams)
			closeExternalNativeQUICConns(conns)
			_ = transport.Close()
			return nil, nil, nil, err
		}
		streams = append(streams, stream)
	}
	return transport, conns, streams, nil
}

func closeExternalNativeQUICConns(conns []*quic.Conn) {
	for _, conn := range conns {
		_ = conn.CloseWithError(0, "")
	}
}

func closeExternalNativeQUICStreams(streams []*quic.Stream) {
	for _, stream := range streams {
		_ = stream.Close()
	}
}

func startExternalNativeQUICTransport(packetConn net.PacketConn, serverTLS *tls.Config) (*quic.Transport, *quic.Listener, error) {
	transport := &quic.Transport{Conn: packetConn}
	listener, err := transport.Listen(serverTLS, quicpath.DefaultQUICConfig())
	if err != nil {
		_ = transport.Close()
		return nil, nil, err
	}
	return transport, listener, nil
}
