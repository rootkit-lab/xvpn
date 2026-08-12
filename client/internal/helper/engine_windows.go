//go:build windows

package helper

import (
	windowsengine "github.com/rootkit-lab/xvpn/client/internal/platform/windows"
	"github.com/rootkit-lab/xvpn/client/internal/tunnel"
)

func newEngine() (tunnel.Engine, error) {
	return windowsengine.New()
}
