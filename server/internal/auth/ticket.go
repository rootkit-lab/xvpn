package auth

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const HandoffTicketTTL = 60 * time.Second

type handoffTicket struct {
	token string
	exp   time.Time
}

// TicketStore guarda JWE por um código opaco de uso único — o browser
// nunca recebe o token no corpo (XSS no xauth não exporta a sessão).
type TicketStore struct {
	mu    sync.Mutex
	items map[string]handoffTicket
}

func NewTicketStore() *TicketStore {
	return &TicketStore{items: make(map[string]handoffTicket)}
}

func (s *TicketStore) Issue(token string, ttl time.Duration) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	s.items[id] = handoffTicket{token: token, exp: time.Now().Add(ttl)}
	return id, nil
}

func (s *TicketStore) Redeem(id string) (string, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[id]
	delete(s.items, id)
	if !ok || time.Now().After(t.exp) {
		return "", false
	}
	return t.token, true
}

func (s *TicketStore) gcLocked() {
	now := time.Now()
	for k, t := range s.items {
		if now.After(t.exp) {
			delete(s.items, k)
		}
	}
}

// IsDocumentNavigation aceita só clique/location.replace (Sec-Fetch).
// fetch() de XSS manda dest=empty e mode=cors.
func IsDocumentNavigation(dest, mode string) bool {
	dest = strings.ToLower(strings.TrimSpace(dest))
	mode = strings.ToLower(strings.TrimSpace(mode))
	return dest == "document" && (mode == "navigate" || mode == "nested-navigate")
}
