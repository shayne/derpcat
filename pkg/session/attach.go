package session

import (
	"context"
	"io"

	"github.com/shayne/derpcat/pkg/stream"
)

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

type nopReadCloser struct {
	io.Reader
}

func (nopReadCloser) Close() error { return nil }

func openSendSource(ctx context.Context, cfg SendConfig) (io.ReadCloser, error) {
	if cfg.Attachment != nil {
		return nopReadCloser{Reader: cfg.Attachment}, nil
	}
	if cfg.TCPListen != "" {
		conn, err := stream.ListenOnce(ctx, cfg.TCPListen)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
	if cfg.TCPConnect != "" {
		conn, err := stream.Connect(ctx, cfg.TCPConnect)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
	if cfg.StdioIn != nil {
		return nopReadCloser{Reader: cfg.StdioIn}, nil
	}
	return nopReadCloser{Reader: io.LimitReader(nilReader{}, 0)}, nil
}

func openListenSink(ctx context.Context, cfg ListenConfig) (io.WriteCloser, error) {
	if cfg.Attachment != nil {
		if wc, ok := cfg.Attachment.(io.WriteCloser); ok {
			return wc, nil
		}
		return nopWriteCloser{Writer: cfg.Attachment}, nil
	}
	if cfg.TCPListen != "" {
		conn, err := stream.ListenOnce(ctx, cfg.TCPListen)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
	if cfg.TCPConnect != "" {
		conn, err := stream.Connect(ctx, cfg.TCPConnect)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
	if cfg.StdioOut != nil {
		return nopWriteCloser{Writer: cfg.StdioOut}, nil
	}
	return nopWriteCloser{Writer: io.Discard}, nil
}

type nilReader struct{}

func (nilReader) Read(_ []byte) (int, error) { return 0, io.EOF }
