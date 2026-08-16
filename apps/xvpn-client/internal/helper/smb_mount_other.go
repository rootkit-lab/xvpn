//go:build !linux

package helper

import (
	"encoding/json"

	"github.com/rootkit-lab/xvpn/client/internal/ipc"
)

// Windows usa UNC no Explorer — nada a montar no helper.
func (h *Helper) handleMountSMB(_ json.RawMessage, _ ipc.Peer) (any, error) {
	return nil, nil
}

func (h *Helper) handleUnmountSMB(_ json.RawMessage, _ ipc.Peer) (any, error) {
	return nil, nil
}
