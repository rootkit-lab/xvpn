package forge

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// RefUpdate é uma linha do receive-pack (old new ref).
type RefUpdate struct {
	OldHex string
	NewHex string
	Ref    string
}

// ParseReceivePack lê os pkt-lines de atualização e devolve o body
// original para o git-http-backend (header + restante).
func ParseReceivePack(r io.Reader) ([]RefUpdate, io.Reader, error) {
	br := bufio.NewReader(r)
	var header bytes.Buffer
	var updates []RefUpdate
	for {
		payload, raw, err := readPktLine(br)
		if err != nil {
			return nil, nil, err
		}
		if _, err := header.Write(raw); err != nil {
			return nil, nil, err
		}
		if payload == nil {
			break
		}
		if i := bytes.IndexByte(payload, 0); i >= 0 {
			payload = payload[:i]
		}
		payload = bytes.TrimSpace(payload)
		if len(payload) == 0 {
			continue
		}
		parts := strings.Fields(string(payload))
		if len(parts) < 3 {
			continue
		}
		updates = append(updates, RefUpdate{OldHex: parts[0], NewHex: parts[1], Ref: parts[2]})
	}
	return updates, io.MultiReader(bytes.NewReader(header.Bytes()), br), nil
}

func readPktLine(r *bufio.Reader) (payload []byte, raw []byte, err error) {
	hdr := make([]byte, 4)
	if _, err = io.ReadFull(r, hdr); err != nil {
		return nil, nil, err
	}
	n, err := hex.DecodeString(string(hdr))
	if err != nil || len(n) != 2 {
		return nil, hdr, fmt.Errorf("pkt-line inválido")
	}
	size := int(n[0])<<8 | int(n[1])
	if size == 0 {
		return nil, hdr, nil
	}
	if size < 4 {
		return nil, hdr, fmt.Errorf("pkt-line curto")
	}
	body := make([]byte, size-4)
	if _, err = io.ReadFull(r, body); err != nil {
		return nil, append(hdr, body...), err
	}
	return body, append(hdr, body...), nil
}
