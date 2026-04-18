package session

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	quic "github.com/quic-go/quic-go"
	"github.com/shayne/derphole/pkg/derpbind"
	"github.com/shayne/derphole/pkg/derptun"
	"github.com/shayne/derphole/pkg/quicpath"
	"github.com/shayne/derphole/pkg/rendezvous"
	"github.com/shayne/derphole/pkg/stream"
	"github.com/shayne/derphole/pkg/telemetry"
	sessiontoken "github.com/shayne/derphole/pkg/token"
	"go4.org/mem"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
)

type DerptunServeConfig struct {
	Token         string
	TargetAddr    string
	Emitter       *telemetry.Emitter
	ForceRelay    bool
	UsePublicDERP bool
}

type DerptunOpenConfig struct {
	Token         string
	ListenAddr    string
	BindAddrSink  chan<- string
	Emitter       *telemetry.Emitter
	ForceRelay    bool
	UsePublicDERP bool
}

type DerptunConnectConfig struct {
	Token         string
	StdioIn       io.Reader
	StdioOut      io.Writer
	Emitter       *telemetry.Emitter
	ForceRelay    bool
	UsePublicDERP bool
}

func decodeDerptunCredential(raw string) (derptun.Credential, error) {
	return derptun.DecodeToken(raw, time.Now())
}

func DerptunServe(ctx context.Context, cfg DerptunServeConfig) error {
	cred, err := decodeDerptunCredential(cfg.Token)
	if err != nil {
		return err
	}
	tok, err := cred.SessionToken()
	if err != nil {
		return err
	}
	derpPriv, err := cred.DERPKey()
	if err != nil {
		return err
	}
	quicPriv, err := cred.QUICPrivateKey()
	if err != nil {
		return err
	}
	identity, err := quicpath.SessionIdentityFromEd25519PrivateKey(quicPriv, time.Now())
	if err != nil {
		return err
	}
	dm, err := derpbind.FetchMap(ctx, publicDERPMapURL())
	if err != nil {
		return err
	}
	node := firstDERPNode(dm, int(tok.BootstrapRegion))
	if node == nil {
		return errors.New("no bootstrap DERP node available")
	}
	derpClient, err := derpbind.NewClientWithPrivateKey(ctx, node, publicDERPServerURL(node), derpPriv)
	if err != nil {
		return err
	}
	defer derpClient.Close()
	probeConn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return err
	}
	defer probeConn.Close()
	pm := newBoundPublicPortmap(probeConn, cfg.Emitter)
	defer pm.Close()

	emitStatus(cfg.Emitter, StateWaiting)
	gate := rendezvous.NewDurableGate(tok)
	for {
		if err := serveDerptunOnce(ctx, cfg, tok, identity, dm, derpClient, probeConn, pm, gate); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
}

func serveDerptunOnce(
	ctx context.Context,
	cfg DerptunServeConfig,
	tok sessiontoken.Token,
	identity quicpath.SessionIdentity,
	dm *tailcfg.DERPMap,
	derpClient *derpbind.Client,
	probeConn net.PacketConn,
	pm publicPortmap,
	gate *rendezvous.DurableGate,
) error {
	claimCh, unsubscribeClaims := derpClient.SubscribeLossless(func(pkt derpbind.Packet) bool {
		return isClaimPayload(pkt.Payload)
	})
	defer unsubscribeClaims()

	for {
		pkt, err := receiveSubscribedPacket(ctx, claimCh)
		if err != nil {
			return err
		}
		env, err := decodeEnvelope(pkt.Payload)
		if err != nil || env.Type != envelopeClaim || env.Claim == nil {
			continue
		}
		peerDERP := key.NodePublicFromRaw32(mem.B(env.Claim.DERPPublic[:]))
		decision, _ := gate.Accept(time.Now(), *env.Claim)
		if !decision.Accepted {
			if err := sendEnvelope(ctx, derpClient, peerDERP, envelope{Type: envelopeDecision, Decision: &decision}); err != nil {
				return err
			}
			continue
		}
		if decision.Accept != nil && !cfg.ForceRelay {
			decision.Accept.Candidates = publicProbeCandidates(ctx, probeConn, dm, pm)
		}

		emitStatus(cfg.Emitter, StateClaimed)
		pathEmitter := newTransportPathEmitter(cfg.Emitter)
		pathEmitter.Emit(StateProbing)
		transportCtx, transportCancel := context.WithCancel(ctx)
		transportManager, transportCleanup, err := startExternalTransportManager(
			transportCtx,
			probeConn,
			dm,
			derpClient,
			peerDERP,
			parseCandidateStrings(decision.Accept.Candidates),
			pm,
			cfg.ForceRelay,
		)
		if err != nil {
			transportCancel()
			gate.Release(env.Claim.DERPPublic)
			return err
		}
		pathEmitter.Watch(transportCtx, transportManager)
		pathEmitter.Flush(transportManager)
		seedAcceptedClaimCandidates(transportCtx, transportManager, *env.Claim)

		adapter := quicpath.NewAdapter(transportManager.PeerDatagramConn(transportCtx))
		quicListener, err := quic.Listen(adapter, quicpath.ServerTLSConfig(identity, env.Claim.QUICPublic), quicpath.DefaultQUICConfig())
		if err != nil {
			_ = adapter.Close()
			transportCleanup()
			transportCancel()
			gate.Release(env.Claim.DERPPublic)
			return err
		}
		if err := sendEnvelope(ctx, derpClient, peerDERP, envelope{Type: envelopeDecision, Decision: &decision}); err != nil {
			_ = quicListener.Close()
			_ = adapter.Close()
			transportCleanup()
			transportCancel()
			gate.Release(env.Claim.DERPPublic)
			return err
		}
		quicConn, err := quicListener.Accept(ctx)
		if err != nil {
			_ = quicListener.Close()
			_ = adapter.Close()
			transportCleanup()
			transportCancel()
			gate.Release(env.Claim.DERPPublic)
			return err
		}
		carrier, err := quicConn.AcceptStream(ctx)
		if err != nil {
			_ = quicConn.CloseWithError(1, "accept derptun carrier failed")
			_ = quicListener.Close()
			_ = adapter.Close()
			transportCleanup()
			transportCancel()
			gate.Release(env.Claim.DERPPublic)
			return err
		}

		mux := derptun.NewMux(derptun.MuxConfig{Role: derptun.MuxRoleServer, ReconnectTimeout: 30 * time.Second})
		mux.ReplaceCarrier(quicpath.WrapStream(quicConn, carrier))
		err = serveDerptunMuxTarget(ctx, mux, cfg.TargetAddr, cfg.Emitter)
		_ = mux.Close()
		_ = quicConn.CloseWithError(0, "")
		_ = quicListener.Close()
		_ = adapter.Close()
		pathEmitter.Complete(transportManager)
		transportCleanup()
		transportCancel()
		gate.Release(env.Claim.DERPPublic)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil && !errors.Is(err, net.ErrClosed) {
			return err
		}
	}
}

func serveDerptunMuxTarget(ctx context.Context, mux *derptun.Mux, targetAddr string, emitter *telemetry.Emitter) error {
	for {
		overlayConn, err := mux.Accept(ctx)
		if err != nil {
			return err
		}
		backendConn, err := (&net.Dialer{}).DialContext(ctx, "tcp", targetAddr)
		if err != nil {
			if emitter != nil {
				emitter.Debug("derptun-backend-dial-failed")
			}
			_ = overlayConn.Close()
			continue
		}
		go func() {
			defer overlayConn.Close()
			defer backendConn.Close()
			_ = stream.Bridge(ctx, overlayConn, backendConn)
		}()
	}
}

func DerptunOpen(ctx context.Context, cfg DerptunOpenConfig) error {
	mux, cleanup, err := dialDerptunMux(ctx, cfg.Token, cfg.Emitter, cfg.ForceRelay)
	if err != nil {
		return err
	}
	defer cleanup()
	defer mux.Close()

	listenAddr := cfg.ListenAddr
	if listenAddr == "" {
		listenAddr = "127.0.0.1:0"
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	notifyBindAddr(cfg.BindAddrSink, listener.Addr().String(), ctx)

	return serveOpenListener(ctx, listener, func(ctx context.Context) (net.Conn, error) {
		return mux.OpenStream(ctx)
	}, cfg.Emitter)
}

func DerptunConnect(ctx context.Context, cfg DerptunConnectConfig) error {
	conn, cleanup, err := dialDerptunMuxStream(ctx, cfg.Token, cfg.Emitter, cfg.ForceRelay)
	if err != nil {
		return err
	}
	defer cleanup()
	defer conn.Close()
	return bridgeDerptunStdio(ctx, conn, cfg.StdioIn, cfg.StdioOut)
}

func bridgeDerptunStdio(ctx context.Context, conn net.Conn, in io.Reader, out io.Writer) error {
	if in == nil {
		in = io.Reader(&emptyReader{})
	}
	if out == nil {
		out = io.Discard
	}
	inErr := make(chan error, 1)
	outErr := make(chan error, 1)
	go func() {
		_, err := io.Copy(conn, in)
		inErr <- err
	}()
	go func() {
		_, err := io.Copy(out, conn)
		outErr <- err
	}()

	for {
		select {
		case err := <-inErr:
			if err != nil && !errors.Is(err, io.EOF) {
				_ = conn.Close()
				return err
			}
			inErr = nil
		case err := <-outErr:
			if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) {
				return err
			}
			return nil
		case <-ctx.Done():
			_ = conn.Close()
			return ctx.Err()
		}
	}
}

type emptyReader struct{}

func (*emptyReader) Read([]byte) (int, error) { return 0, io.EOF }

func dialDerptunMuxStream(ctx context.Context, tokenValue string, emitter *telemetry.Emitter, forceRelay bool) (net.Conn, func(), error) {
	mux, cleanup, err := dialDerptunMux(ctx, tokenValue, emitter, forceRelay)
	if err != nil {
		return nil, nil, err
	}
	conn, err := mux.OpenStream(ctx)
	if err != nil {
		cleanup()
		_ = mux.Close()
		return nil, nil, err
	}
	return conn, func() {
		_ = mux.Close()
		cleanup()
	}, nil
}

func dialDerptunMux(ctx context.Context, tokenValue string, emitter *telemetry.Emitter, forceRelay bool) (*derptun.Mux, func(), error) {
	cred, err := decodeDerptunCredential(tokenValue)
	if err != nil {
		return nil, nil, err
	}
	tok, err := cred.SessionToken()
	if err != nil {
		return nil, nil, err
	}
	listenerDERP := key.NodePublicFromRaw32(mem.B(tok.DERPPublic[:]))
	if listenerDERP.IsZero() {
		return nil, nil, ErrUnknownSession
	}
	dm, err := derpbind.FetchMap(ctx, publicDERPMapURL())
	if err != nil {
		return nil, nil, err
	}
	node := firstDERPNode(dm, int(tok.BootstrapRegion))
	if node == nil {
		return nil, nil, errors.New("no bootstrap DERP node available")
	}
	derpClient, err := derpbind.NewClient(ctx, node, publicDERPServerURL(node))
	if err != nil {
		return nil, nil, err
	}
	probeConn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		_ = derpClient.Close()
		return nil, nil, err
	}
	pm := newBoundPublicPortmap(probeConn, emitter)
	clientIdentity, err := quicpath.GenerateSessionIdentity()
	if err != nil {
		_ = pm.Close()
		_ = probeConn.Close()
		_ = derpClient.Close()
		return nil, nil, err
	}

	var localCandidates []string
	if !forceRelay {
		localCandidates = publicProbeCandidates(ctx, probeConn, dm, pm)
	}
	claim := rendezvous.Claim{
		Version:      tok.Version,
		SessionID:    tok.SessionID,
		DERPPublic:   derpPublicKeyRaw32(derpClient.PublicKey()),
		QUICPublic:   clientIdentity.Public,
		Candidates:   localCandidates,
		Capabilities: tok.Capabilities,
	}
	claim.BearerMAC = rendezvous.ComputeBearerMAC(tok.BearerSecret, claim)
	decision, err := sendClaimAndReceiveDecision(ctx, derpClient, listenerDERP, claim)
	if err != nil {
		_ = pm.Close()
		_ = probeConn.Close()
		_ = derpClient.Close()
		return nil, nil, err
	}
	if !decision.Accepted {
		_ = pm.Close()
		_ = probeConn.Close()
		_ = derpClient.Close()
		if decision.Reject != nil {
			return nil, nil, errors.New(decision.Reject.Reason)
		}
		return nil, nil, errors.New("claim rejected")
	}

	pathEmitter := newTransportPathEmitter(emitter)
	pathEmitter.Emit(StateProbing)
	transportCtx, transportCancel := context.WithCancel(ctx)
	transportManager, transportCleanup, err := startExternalTransportManager(
		transportCtx,
		probeConn,
		dm,
		derpClient,
		listenerDERP,
		parseCandidateStrings(localCandidates),
		pm,
		forceRelay,
	)
	if err != nil {
		transportCancel()
		_ = pm.Close()
		_ = probeConn.Close()
		_ = derpClient.Close()
		return nil, nil, err
	}
	pathEmitter.Watch(transportCtx, transportManager)
	pathEmitter.Flush(transportManager)
	seedAcceptedDecisionCandidates(transportCtx, transportManager, decision)

	peerConn := transportManager.PeerDatagramConn(transportCtx)
	adapter := quicpath.NewAdapter(peerConn)
	quicConn, err := quic.Dial(ctx, adapter, peerConn.RemoteAddr(), quicpath.ClientTLSConfig(clientIdentity, tok.QUICPublic), quicpath.DefaultQUICConfig())
	if err != nil {
		_ = adapter.Close()
		transportCleanup()
		transportCancel()
		_ = pm.Close()
		_ = probeConn.Close()
		_ = derpClient.Close()
		return nil, nil, err
	}
	carrier, err := quicConn.OpenStreamSync(ctx)
	if err != nil {
		_ = quicConn.CloseWithError(1, "open derptun carrier failed")
		_ = adapter.Close()
		transportCleanup()
		transportCancel()
		_ = pm.Close()
		_ = probeConn.Close()
		_ = derpClient.Close()
		return nil, nil, err
	}
	mux := derptun.NewMux(derptun.MuxConfig{Role: derptun.MuxRoleClient, ReconnectTimeout: 30 * time.Second})
	mux.ReplaceCarrier(quicpath.WrapStream(quicConn, carrier))
	cleanup := func() {
		_ = quicConn.CloseWithError(0, "")
		_ = adapter.Close()
		pathEmitter.Complete(transportManager)
		transportCleanup()
		transportCancel()
		_ = pm.Close()
		_ = probeConn.Close()
		_ = derpClient.Close()
	}
	return mux, cleanup, nil
}
