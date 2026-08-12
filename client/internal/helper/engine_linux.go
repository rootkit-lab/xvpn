//go:build linux

package helper

import (
	linuxengine "github.com/rootkit-lab/xvpn/client/internal/platform/linux"
	"github.com/rootkit-lab/xvpn/client/internal/tunnel"
)

func newEngine() (tunnel.Engine, error) {
	return linuxengine.New()
}
