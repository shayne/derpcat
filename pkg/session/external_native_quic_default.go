//go:build !linux

package session

import "net"

func disablePublicNativeQUICReceiveOffload(net.PacketConn) error {
	return nil
}
