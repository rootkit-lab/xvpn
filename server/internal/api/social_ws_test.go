package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestSocialWS_RejectsTokenInQuery(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodGet, "/api/ws?token="+token, nil, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("token na query deveria 400, obtido %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "query") {
		t.Fatalf("mensagem deveria citar query string: %s", rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/ws?access_token="+token, nil, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("access_token na query deveria 400, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSocialWS_AuthFirstFrame(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]string{"type": "auth", "token": token}); err != nil {
		t.Fatalf("auth frame: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("lendo presence: %v", err)
	}
	var ev wsEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatalf("json: %v (%s)", err, raw)
	}
	if ev.Type != "presence" {
		t.Fatalf("esperado presence, obtido %s (%s)", ev.Type, raw)
	}
}
