//go:build linux

package ipc

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

func peerFromConn(conn net.Conn) (Peer, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return Peer{}, fmt.Errorf("conexão IPC não é Unix socket")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return Peer{}, fmt.Errorf("SO_PEERCRED: %w", err)
	}
	var cred *unix.Ucred
	var ctrlErr error
	if err := raw.Control(func(fd uintptr) {
		cred, ctrlErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return Peer{}, fmt.Errorf("SO_PEERCRED: %w", err)
	}
	if ctrlErr != nil {
		return Peer{}, fmt.Errorf("SO_PEERCRED: %w", ctrlErr)
	}
	if cred == nil {
		return Peer{}, fmt.Errorf("SO_PEERCRED vazio")
	}
	if cred.Uid < 1 {
		return Peer{}, fmt.Errorf("uid do peer recusado")
	}
	return Peer{UID: int(cred.Uid), GID: int(cred.Gid)}, nil
}
