package derpbind

import (
	"context"
	"errors"
	"fmt"

	"tailscale.com/derp"
	"tailscale.com/derp/derphttp"
	"tailscale.com/net/netmon"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
)

type Packet struct {
	From    key.NodePublic
	Payload []byte
}

type Client struct {
	pub key.NodePublic
	dc  *derphttp.Client
}

func NewClient(ctx context.Context, node *tailcfg.DERPNode, serverURL string) (*Client, error) {
	if node == nil {
		return nil, errors.New("nil DERP node")
	}

	priv := key.NewNode()
	dc, err := derphttp.NewClient(priv, serverURL, func(string, ...any) {}, netmon.NewStatic())
	if err != nil {
		return nil, err
	}
	if err := dc.Connect(ctx); err != nil {
		_ = dc.Close()
		return nil, fmt.Errorf("connect derp client: %w", err)
	}
	if err := waitForServerInfo(dc); err != nil {
		_ = dc.Close()
		return nil, fmt.Errorf("wait for server info: %w", err)
	}

	return &Client{pub: priv.Public(), dc: dc}, nil
}

func waitForServerInfo(dc *derphttp.Client) error {
	for {
		msg, err := dc.Recv()
		if err != nil {
			return err
		}
		if _, ok := msg.(derp.ServerInfoMessage); ok {
			return nil
		}
	}
}

func (c *Client) PublicKey() key.NodePublic { return c.pub }

func (c *Client) Close() error {
	if c == nil || c.dc == nil {
		return nil
	}
	return c.dc.Close()
}

func (c *Client) Send(ctx context.Context, dst key.NodePublic, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.dc.Send(dst, payload)
}

func (c *Client) Receive(ctx context.Context) (Packet, error) {
	type result struct {
		msg derp.ReceivedMessage
		err error
	}

	ch := make(chan result, 1)
	go func() {
		for {
			msg, err := c.dc.Recv()
			if err != nil {
				ch <- result{err: err}
				return
			}
			pkt, ok := msg.(derp.ReceivedPacket)
			if !ok {
				continue
			}
			ch <- result{msg: pkt}
			return
		}
	}()

	select {
	case <-ctx.Done():
		_ = c.dc.Close()
		return Packet{}, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return Packet{}, res.err
		}
		pkt := res.msg.(derp.ReceivedPacket)
		return Packet{
			From:    pkt.Source,
			Payload: append([]byte(nil), pkt.Data...),
		}, nil
	}
}
