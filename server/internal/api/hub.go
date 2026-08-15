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
}

func newHub() *Hub {
	return &Hub{clients: make(map[uint]map[chan []byte]struct{})}
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
	h.clients[userID][ch] = struct{}{}
	return ch
}

func (h *Hub) unsubscribe(userID uint, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.clients[userID]; ok {
		delete(set, ch)
		if len(set) == 0 {
			delete(h.clients, userID)
		}
	}
	close(ch)
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
