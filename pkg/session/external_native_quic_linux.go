//go:build linux

package session

import (
	"errors"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func disablePublicNativeQUICReceiveOffload(conn net.PacketConn) error {
	if conn == nil {
		return nil
	}
	rawConn, ok := conn.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	if !ok {
		return nil
	}
	sysConn, err := rawConn.SyscallConn()
	if err != nil {
		return err
	}

	var sockErr error
	if err := sysConn.Control(func(fd uintptr) {
		sockErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_UDP, unix.UDP_GRO, 0)
	}); err != nil {
		return err
	}
	if sockErr != nil && !errors.Is(sockErr, unix.ENOPROTOOPT) {
		return sockErr
	}
	return nil
}
