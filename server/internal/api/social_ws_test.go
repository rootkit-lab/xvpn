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

func TestSocialWS_PresenceStatusBroadcast(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	createTestUserWithRole(t, app, "bob", "senha-bob-ok-ok", store.RoleMember)
	router := NewRouter(app)
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-ok-ok")

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws"

	dial := func(token string) *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := conn.WriteJSON(map[string]string{"type": "auth", "token": token}); err != nil {
			t.Fatalf("auth: %v", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, _ = conn.ReadMessage()
		_, _, _ = conn.ReadMessage()
		return conn
	}

	alice := dial(aliceTok)
	defer alice.Close()
	bob := dial(bobTok)
	defer bob.Close()

	if err := alice.WriteJSON(map[string]any{"type": "presence", "payload": map[string]string{"status": "away"}}); err != nil {
		t.Fatalf("presence: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		_ = bob.SetReadDeadline(deadline)
		_, raw, err := bob.ReadMessage()
		if err != nil {
			t.Fatalf("bob deveria receber presence away: %v", err)
		}
		var ev wsEvent
		if json.Unmarshal(raw, &ev) != nil {
			continue
		}
		if ev.Type != "presence" {
			continue
		}
		rawPayload, _ := json.Marshal(ev.Payload)
		var p struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(rawPayload, &p)
		if p.Status == "away" {
			return
		}
	}
}

func TestSocialWS_InvisibleAbsentFromSnapshot(t *testing.T) {
	app, _ := newTestApp(t)
	aliceUser := createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	createTestUserWithRole(t, app, "bob", "senha-bob-ok-ok", store.RoleMember)
	router := NewRouter(app)
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-ok-ok")

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws"

	alice, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial alice: %v", err)
	}
	defer alice.Close()
	if err := alice.WriteJSON(map[string]string{"type": "auth", "token": aliceTok}); err != nil {
		t.Fatalf("auth alice: %v", err)
	}
	_ = alice.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = alice.ReadMessage()
	_, _, _ = alice.ReadMessage()
	if err := alice.WriteJSON(map[string]any{"type": "presence", "payload": map[string]string{"status": "invisible"}}); err != nil {
		t.Fatalf("invisible: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	bob, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial bob: %v", err)
	}
	defer bob.Close()
	if err := bob.WriteJSON(map[string]string{"type": "auth", "token": bobTok}); err != nil {
		t.Fatalf("auth bob: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var snapshot []map[string]any
	for snapshot == nil {
		_ = bob.SetReadDeadline(deadline)
		_, raw, err := bob.ReadMessage()
		if err != nil {
			t.Fatalf("bob deveria receber snapshot: %v", err)
		}
		var ev wsEvent
		if json.Unmarshal(raw, &ev) != nil {
			continue
		}
		if ev.Type != "presence.snapshot" {
			continue
		}
		b, _ := json.Marshal(ev.Payload)
		if err := json.Unmarshal(b, &snapshot); err != nil {
			t.Fatalf("snapshot json: %v (%s)", err, raw)
		}
	}
	for _, row := range snapshot {
		id, _ := row["user_id"].(float64)
		if uint(id) == aliceUser.ID {
			t.Fatalf("alice invisível vazou no snapshot: %+v", snapshot)
		}
		if st, _ := row["status"].(string); st == "offline" {
			t.Fatalf("snapshot não deve listar offline (invisível disfarçado): %+v", snapshot)
		}
	}
}
