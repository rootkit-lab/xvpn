//go:build !linux

package ipc

import "net"

func peerFromConn(_ net.Conn) (Peer, error) {
	return Peer{}, nil
}
