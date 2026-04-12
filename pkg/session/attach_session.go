package session

import (
	"context"
	"crypto/rand"
	"net"
	"sync"
	"time"

	"github.com/shayne/derpcat/pkg/token"
)

var (
	attachMu        sync.Mutex
	attachMailboxes = map[string]chan net.Conn{}
)

func issueLocalAttachToken() (string, chan net.Conn, error) {
	var sessionID [16]byte
	if _, err := rand.Read(sessionID[:]); err != nil {
		return "", nil, err
	}

	var bearerSecret [32]byte
	if _, err := rand.Read(bearerSecret[:]); err != nil {
		return "", nil, err
	}

	tok, err := token.Encode(token.Token{
		Version:      token.SupportedVersion,
		SessionID:    sessionID,
		ExpiresUnix:  time.Now().Add(time.Hour).Unix(),
		BearerSecret: bearerSecret,
		Capabilities: token.CapabilityAttach,
	})
	if err != nil {
		return "", nil, err
	}

	mailbox := make(chan net.Conn)
	attachMu.Lock()
	attachMailboxes[tok] = mailbox
	attachMu.Unlock()
	return tok, mailbox, nil
}

func ListenAttach(ctx context.Context, cfg AttachListenConfig) (*AttachListener, error) {
	tok, mailbox, err := issueLocalAttachToken()
	if err != nil {
		return nil, err
	}

	closed := make(chan struct{})
	var closeOnce sync.Once
	listener := &AttachListener{Token: tok}
	listener.accept = func(ctx context.Context) (net.Conn, error) {
		select {
		case conn := <-mailbox:
			return conn, nil
		case <-closed:
			return nil, net.ErrClosed
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	listener.close = func() error {
		closeOnce.Do(func() {
			close(closed)
			attachMu.Lock()
			if attachMailboxes[tok] == mailbox {
				delete(attachMailboxes, tok)
			}
			attachMu.Unlock()
		})
		return nil
	}
	return listener, nil
}

func DialAttach(ctx context.Context, cfg AttachDialConfig) (net.Conn, error) {
	attachMu.Lock()
	mailbox, ok := attachMailboxes[cfg.Token]
	attachMu.Unlock()
	if !ok {
		return nil, ErrUnknownSession
	}

	left, right := net.Pipe()
	select {
	case mailbox <- right:
		return left, nil
	case <-ctx.Done():
		_ = left.Close()
		_ = right.Close()
		return nil, ctx.Err()
	}
}
