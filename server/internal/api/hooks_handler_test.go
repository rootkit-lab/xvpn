package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestHooksBroadcastRequiresToken(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.XbotToken = "token-de-servico-com-32-bytes-ok!!"
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodPost, "/api/hooks/chat/broadcast",
		hookBroadcastRequest{Body: "oi"}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem token: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/hooks/chat/broadcast",
		hookBroadcastRequest{Body: "deploy ok"}, "token-de-servico-com-32-bytes-ok!!")
	if rec.Code != http.StatusCreated {
		t.Fatalf("com token: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true {
		t.Fatalf("resp: %v", out)
	}
	var n int64
	if err := app.Store.DB.Model(&store.Message{}).Count(&n).Error; err != nil || n < 1 {
		t.Fatalf("mensagem não persistiu: n=%d err=%v", n, err)
	}
}
