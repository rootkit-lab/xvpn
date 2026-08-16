package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestSocialAck_DeliveredAndRead(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	createTestUserWithRole(t, app, "bob", "senha-bob-ok", store.RoleMember)
	router := NewRouter(app)
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/social/threads", openThreadRequest{Username: "bob"}, aliceTok)
	var th socialThreadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &th); err != nil || th.ID == 0 {
		t.Fatalf("thread: %d %s", rec.Code, rec.Body.String())
	}
	path := "/api/social/threads/dm/" + strconv.FormatUint(uint64(th.ID), 10) + "/messages"
	rec = doJSON(t, router, http.MethodPost, path, postMessageRequest{Body: "oi"}, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	var msg socialMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil || msg.ID == 0 {
		t.Fatalf("msg: %s", rec.Body.String())
	}
	if msg.Delivered || msg.Read {
		t.Fatalf("recibo inicial deveria ser falso: %+v", msg)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/social/acks", ackRequest{MessageIDs: []uint{msg.ID}, State: "delivered"}, bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("ack delivered: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, path, nil, aliceTok)
	env := decodePage[socialMessageResponse](t, rec.Body.Bytes())
	items := pageItems[socialMessageResponse](t, env)
	if len(items) != 1 || !items[0].Delivered || items[0].Read {
		t.Fatalf("após delivered: %+v", items)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/social/acks", ackRequest{MessageIDs: []uint{msg.ID}, State: "read"}, bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("ack read: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, path, nil, aliceTok)
	env = decodePage[socialMessageResponse](t, rec.Body.Bytes())
	items = pageItems[socialMessageResponse](t, env)
	if len(items) != 1 || !items[0].Delivered || !items[0].Read {
		t.Fatalf("após read: %+v", items)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/social/acks", ackRequest{MessageIDs: []uint{msg.ID}, State: "read"}, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("ack próprio: %d", rec.Code)
	}
	var n int64
	if err := app.Store.DB.Model(&store.MessageReceipt{}).Where("message_id = ? AND user_id = ?", msg.ID, msg.AuthorID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("autor não deveria gravar recibo próprio, n=%d", n)
	}
}
