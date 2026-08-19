package api

import (
	"encoding/json"
	"sync"
)

type wsEvent struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type Hub struct {
	mu      sync.Mutex
	clients map[uint]map[chan []byte]struct{}
	status  map[uint]string
}

func newHub() *Hub {
	return &Hub{
		clients: make(map[uint]map[chan []byte]struct{}),
		status:  make(map[uint]string),
	}
}

var presenceAllowed = map[string]struct{}{
	"online":    {},
	"away":      {},
	"dnd":       {},
	"invisible": {},
}

func normalizePresence(status string) string {
	if _, ok := presenceAllowed[status]; ok {
		return status
	}
	return "online"
}

func visiblePresence(status string) string {
	if status == "" || status == "invisible" {
		return "offline"
	}
	return status
}

const maxWSConnsPerUser = 4

func (h *Hub) connectionCount(userID uint) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients[userID])
}

func (h *Hub) subscribe(userID uint) chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[chan []byte]struct{})
	}
	first := len(h.clients[userID]) == 0
	h.clients[userID][ch] = struct{}{}
	if first {
		if _, ok := h.status[userID]; !ok {
			h.status[userID] = "online"
		}
	}
	return ch
}

func (h *Hub) unsubscribe(userID uint, ch chan []byte) {
	h.mu.Lock()
	last := false
	if set, ok := h.clients[userID]; ok {
		delete(set, ch)
		if len(set) == 0 {
			delete(h.clients, userID)
			delete(h.status, userID)
			last = true
		}
	}
	h.mu.Unlock()
	close(ch)
	if last {
		h.broadcastPresence(userID, "offline")
	}
}

func (h *Hub) setStatus(userID uint, status string) string {
	status = normalizePresence(status)
	h.mu.Lock()
	prevVisible := visiblePresence(h.status[userID])
	h.status[userID] = status
	h.mu.Unlock()
	visible := visiblePresence(status)
	// invisível deve parecer disconnect: só anuncia se o status visível mudou
	// (online→offline ao ficar invisível; offline→online ao reaparecer).
	if visible != prevVisible {
		h.broadcastPresence(userID, visible)
	}
	return visible
}

func (h *Hub) broadcastPresence(userID uint, status string) {
	h.sendToMany(h.connectedIDs(), wsEvent{Type: "presence", Payload: map[string]any{
		"user_id": userID,
		"status":  status,
	}})
}

// announceJoin avisa os outros que o usuário entrou. Quem está invisível
// não é anunciado — equivalente a nunca ter conectado.
func (h *Hub) announceJoin(userID uint) {
	if h.statusOf(userID) == "offline" {
		return
	}
	h.broadcastPresence(userID, h.statusOf(userID))
}

func (h *Hub) connectedIDs() []uint {
	h.mu.Lock()
	defer h.mu.Unlock()
	ids := make([]uint, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

func (h *Hub) presenceSnapshot() []map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]map[string]any, 0, len(h.clients))
	for id := range h.clients {
		vis := visiblePresence(h.status[id])
		if vis == "offline" {
			continue
		}
		out = append(out, map[string]any{
			"user_id": id,
			"status":  vis,
		})
	}
	return out
}

func (h *Hub) ownStatus(userID uint) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.status[userID]
	if s == "" {
		return "online"
	}
	return s
}

func (h *Hub) statusOf(userID uint) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return visiblePresence(h.status[userID])
}

func (h *Hub) sendTo(userID uint, ev wsEvent) {
	raw, err := json.Marshal(ev)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients[userID] {
		select {
		case ch <- raw:
		default:
		}
	}
}

func (h *Hub) sendToMany(userIDs []uint, ev wsEvent) {
	seen := make(map[uint]struct{}, len(userIDs))
	for _, id := range userIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		h.sendTo(id, ev)
	}
}
