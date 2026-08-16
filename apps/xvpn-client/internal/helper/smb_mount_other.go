//go:build !linux

package helper

import "encoding/json"

// Windows usa UNC no Explorer — nada a montar no helper.
func (h *Helper) handleMountSMB(_ json.RawMessage) (any, error) {
	return nil, nil
}

func (h *Helper) handleUnmountSMB(_ json.RawMessage) (any, error) {
	return nil, nil
}
