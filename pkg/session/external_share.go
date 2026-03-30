package session

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/shayne/derpcat/pkg/derpbind"
	"github.com/shayne/derpcat/pkg/rendezvous"
	"github.com/shayne/derpcat/pkg/stream"
	"github.com/shayne/derpcat/pkg/telemetry"
	"github.com/shayne/derpcat/pkg/token"
	"github.com/shayne/derpcat/pkg/wg"
	"go4.org/mem"
	"tailscale.com/types/key"
)

func issuePublicShareSession(ctx context.Context, cfg ShareConfig) (string, *relaySession, error) {
	dm, err := derpbind.FetchMap(ctx, derpbind.PublicDERPMapURL)
	if err != nil {
		return "", nil, err
	}
	node := firstDERPNode(dm, 0)
	if node == nil {
		return "", nil, errors.New("no DERP node available")
	}

	derpClient, err := derpbind.NewClient(ctx, node, derpServerURL(node))
	if err != nil {
		return "", nil, err
	}

	var sessionID [16]byte
	if _, err := rand.Read(sessionID[:]); err != nil {
		_ = derpClient.Close()
		return "", nil, err
	}
	var bearerSecret [32]byte
	if _, err := rand.Read(bearerSecret[:]); err != nil {
		_ = derpClient.Close()
		return "", nil, err
	}
	wgPrivate, wgPublic, err := wg.GenerateKeypair()
	if err != nil {
		_ = derpClient.Close()
		return "", nil, err
	}
	_, discoPublic, err := wg.GenerateKeypair()
	if err != nil {
		_ = derpClient.Close()
		return "", nil, err
	}

	tokValue := token.Token{
		Version:         token.SupportedVersion,
		SessionID:       sessionID,
		ExpiresUnix:     time.Now().Add(10 * time.Minute).Unix(),
		BootstrapRegion: uint16(node.RegionID),
		DERPPublic:      derpPublicKeyRaw32(derpClient.PublicKey()),
		WGPublic:        wgPublic,
		DiscoPublic:     discoPublic,
		BearerSecret:    bearerSecret,
		Capabilities:    token.CapabilityShare,
		ShareTargetAddr: cfg.TargetAddr,
		DefaultBindHost: "127.0.0.1",
		DefaultBindPort: 0,
	}
	tok, err := token.Encode(tokValue)
	if err != nil {
		_ = derpClient.Close()
		return "", nil, err
	}

	probeConn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		_ = derpClient.Close()
		return "", nil, err
	}

	session := &relaySession{
		probeConn: probeConn,
		derp:      derpClient,
		token:     tokValue,
		gate:      rendezvous.NewGate(tokValue),
		derpMap:   dm,
		wgPrivate: wgPrivate,
	}
	return tok, session, nil
}

func shareExternal(ctx context.Context, cfg ShareConfig) (string, error) {
	tok, session, err := issuePublicShareSession(ctx, cfg)
	if err != nil {
		return "", err
	}
	defer session.derp.Close()
	defer session.probeConn.Close()

	emitStatus(cfg.Emitter, StateWaiting)
	if cfg.TokenSink != nil {
		select {
		case cfg.TokenSink <- tok:
		case <-ctx.Done():
			return tok, ctx.Err()
		}
	}

	for {
		pkt, err := session.derp.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return tok, ctx.Err()
			}
			return tok, err
		}
		env, err := decodeEnvelope(pkt.Payload)
		if err != nil || env.Type != envelopeClaim || env.Claim == nil {
			continue
		}

		peerDERP := key.NodePublicFromRaw32(mem.B(env.Claim.DERPPublic[:]))
		decision, _ := session.gate.Accept(time.Now(), *env.Claim)
		if !decision.Accepted {
			if err := sendEnvelope(ctx, session.derp, peerDERP, envelope{Type: envelopeDecision, Decision: &decision}); err != nil {
				return tok, err
			}
			continue
		}

		emitStatus(cfg.Emitter, StateClaimed)
		if decision.Accept != nil {
			decision.Accept.Candidates = publicProbeCandidates(ctx, session.probeConn, session.derpMap)
		}
		if !cfg.ForceRelay {
			probeCtx, cancel := context.WithTimeout(ctx, directProbeWindow)
			serveDirectProbes(probeCtx, session.probeConn)
			cancel()
		}

		_, listenerAddr, senderAddr := wg.DeriveAddresses(session.token.SessionID)
		sessionNode, err := wg.NewNode(wg.Config{
			PrivateKey:    session.wgPrivate,
			PeerPublicKey: env.Claim.WGPublic,
			LocalAddr:     listenerAddr,
			PeerAddr:      senderAddr,
			PacketConn:    session.probeConn,
			DERPClient:    session.derp,
			PeerDERP:      peerDERP,
		})
		if err != nil {
			return tok, err
		}
		defer sessionNode.Close()

		ln, err := sessionNode.ListenTCP(overlayPort)
		if err != nil {
			return tok, err
		}
		defer ln.Close()

		if err := sendEnvelope(ctx, session.derp, peerDERP, envelope{Type: envelopeDecision, Decision: &decision}); err != nil {
			return tok, err
		}
		return tok, serveOverlayListener(ctx, ln, cfg.TargetAddr, sessionNode, cfg.Emitter)
	}
}

func openExternal(ctx context.Context, cfg OpenConfig, tok token.Token) error {
	listenerDERP := key.NodePublicFromRaw32(mem.B(tok.DERPPublic[:]))
	if listenerDERP.IsZero() {
		return ErrUnknownSession
	}

	dm, err := derpbind.FetchMap(ctx, derpbind.PublicDERPMapURL)
	if err != nil {
		return err
	}
	node := firstDERPNode(dm, int(tok.BootstrapRegion))
	if node == nil {
		return errors.New("no bootstrap DERP node available")
	}
	derpClient, err := derpbind.NewClient(ctx, node, derpServerURL(node))
	if err != nil {
		return err
	}
	defer derpClient.Close()

	probeConn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return err
	}
	defer probeConn.Close()

	senderPrivate, senderPublic, err := wg.GenerateKeypair()
	if err != nil {
		return err
	}
	_, senderDisco, err := wg.GenerateKeypair()
	if err != nil {
		return err
	}

	claim := rendezvous.Claim{
		Version:      tok.Version,
		SessionID:    tok.SessionID,
		DERPPublic:   derpPublicKeyRaw32(derpClient.PublicKey()),
		WGPublic:     senderPublic,
		DiscoPublic:  senderDisco,
		Candidates:   publicProbeCandidates(ctx, probeConn, dm),
		Capabilities: tok.Capabilities,
	}
	claim.BearerMAC = rendezvous.ComputeBearerMAC(tok.BearerSecret, claim)
	if err := sendEnvelope(ctx, derpClient, listenerDERP, envelope{Type: envelopeClaim, Claim: &claim}); err != nil {
		return err
	}

	decision, err := receiveDecision(ctx, derpClient, listenerDERP)
	if err != nil {
		return err
	}
	if !decision.Accepted {
		if decision.Reject != nil {
			return errors.New(decision.Reject.Reason)
		}
		return errors.New("claim rejected")
	}

	directCandidate := ""
	if !cfg.ForceRelay && decision.Accept != nil {
		if candidate, ok := firstDirectCandidate(ctx, probeConn, decision.Accept.Candidates); ok {
			directCandidate = candidate
		}
	}

	_, listenerAddr, senderAddr := wg.DeriveAddresses(tok.SessionID)
	sessionNode, err := wg.NewNode(wg.Config{
		PrivateKey:    senderPrivate,
		PeerPublicKey: tok.WGPublic,
		LocalAddr:     senderAddr,
		PeerAddr:      listenerAddr,
		PacketConn:    probeConn,
		DERPClient:    derpClient,
		PeerDERP:      listenerDERP,
	})
	if err != nil {
		return err
	}
	defer sessionNode.Close()

	if directCandidate != "" {
		if err := sessionNode.SetDirectEndpoint(directCandidate); err != nil {
			return err
		}
	}

	listener, err := openLocalListener(cfg, tok)
	if err != nil {
		return err
	}
	defer listener.Close()
	notifyBindAddr(cfg.BindAddrSink, listener.Addr().String(), ctx)

	var pathOnce sync.Once
	return serveOpenListener(ctx, listener, func(ctx context.Context) (net.Conn, error) {
		conn, err := dialOverlay(ctx, sessionNode, netip.AddrPortFrom(listenerAddr, overlayPort))
		if err == nil {
			pathOnce.Do(func() {
				emitStatus(cfg.Emitter, transportState(sessionNode))
			})
		}
		return conn, err
	}, cfg.Emitter)
}

func serveOverlayListener(ctx context.Context, listener net.Listener, targetAddr string, sessionNode *wg.Node, emitter *telemetry.Emitter) error {
	var pathOnce sync.Once
	for {
		overlayConn, err := acceptOverlay(ctx, listener)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		pathOnce.Do(func() {
			emitStatus(emitter, transportState(sessionNode))
		})

		backendConn, err := (&net.Dialer{}).DialContext(ctx, "tcp", targetAddr)
		if err != nil {
			if emitter != nil {
				emitter.Debug("backend-dial-failed")
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
