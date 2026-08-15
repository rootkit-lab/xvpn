package main

import (
	"context"
	"sync"

	"github.com/rootkit-lab/xvpn/chat/internal/socialclient"
	"github.com/rootkit-lab/xvpn/chat/internal/version"
)

// ChatService é a ponte Wails entre a GUI e o protocolo social (Fase 19.3).
// O JWT nunca é exposto ao frontend.
type ChatService struct {
	client *socialclient.Client

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewChatService() *ChatService {
	return &ChatService{client: socialclient.New()}
}

func (s *ChatService) Version() string {
	return version.String()
}

func (s *ChatService) Login(username, password string) (socialclient.Session, error) {
	return s.client.Login(context.Background(), socialclient.DefaultBaseURL, username, password)
}

func (s *ChatService) Logout() {
	s.stopWS()
	s.client.Logout()
}

func (s *ChatService) Session() socialclient.Session {
	return s.client.Session()
}

func (s *ChatService) ListPeople(page int, q string) (socialclient.ProfilePage, error) {
	return s.client.ListPeople(context.Background(), page, q)
}

func (s *ChatService) ListThreads(page int) (socialclient.ThreadPage, error) {
	return s.client.ListThreads(context.Background(), page)
}

func (s *ChatService) ListGroups(page int) (socialclient.GroupPage, error) {
	return s.client.ListGroups(context.Background(), page)
}

func (s *ChatService) OpenThread(username string) (socialclient.Thread, error) {
	return s.client.OpenThread(context.Background(), username)
}

func (s *ChatService) ListMessages(kind string, id uint, page int) (socialclient.MessagePage, error) {
	return s.client.ListMessages(context.Background(), kind, id, page)
}

func (s *ChatService) PostMessage(kind string, id uint, body string) (socialclient.Message, error) {
	return s.client.PostMessage(context.Background(), kind, id, body)
}

func (s *ChatService) CreateGroup(name, description string) (socialclient.Group, error) {
	return s.client.CreateGroup(context.Background(), name, description)
}

func (s *ChatService) SendTyping(kind string, threadID uint) error {
	return s.client.SendTyping(kind, threadID)
}

func (s *ChatService) stopWS() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// StartEvents abre o WebSocket. Eventos chegam via Events.On("social:event") no frontend.
func (s *ChatService) StartEvents() error {
	s.stopWS()
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	go func() {
		_ = s.client.ListenWS(ctx, func(ev socialclient.WSEvent) {
			emitSocialEvent(ev)
		})
	}()
	return nil
}
