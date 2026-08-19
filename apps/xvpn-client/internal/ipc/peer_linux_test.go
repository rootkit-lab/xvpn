//go:build linux

package ipc

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestPeerFromConn_MatchesCaller(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "peer.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	type result struct {
		peer Peer
		err  error
	}
	got := make(chan result, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			got <- result{err: err}
			return
		}
		defer c.Close()
		p, err := peerFromConn(c)
		got <- result{peer: p, err: err}
	}()

	client, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	r := <-got
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.peer.UID != os.Getuid() {
		t.Fatalf("uid %d want %d", r.peer.UID, os.Getuid())
	}
	if r.peer.GID != os.Getgid() {
		t.Fatalf("gid %d want %d", r.peer.GID, os.Getgid())
	}
}
