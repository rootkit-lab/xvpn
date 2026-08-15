package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestSocialPeople_MemberCanList(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	createTestUserWithRole(t, app, "bob", "senha-bob-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodGet, "/api/social/people?q=bob", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	env := decodePage[socialProfileResponse](t, rec.Body.Bytes())
	items := pageItems[socialProfileResponse](t, env)
	if env.Total != 1 || len(items) != 1 || items[0].Username != "bob" {
		t.Fatalf("q=bob deveria achar só bob: %+v", env)
	}
}

func TestSocialDM_OnlyExistingUsers(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	createTestUserWithRole(t, app, "bob", "senha-bob-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/social/threads", openThreadRequest{Username: "naoexiste"}, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("DM com usuário inexistente deveria 404, obtido %d: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/social/threads", openThreadRequest{Username: "bob"}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("DM com bob deveria 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var th socialThreadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &th); err != nil {
		t.Fatalf("thread inválida: %v", err)
	}
	if th.Kind != "dm" || th.ID == 0 {
		t.Fatalf("thread inesperada: %+v", th)
	}

	path := "/api/social/threads/dm/" + strconv.FormatUint(uint64(th.ID), 10) + "/messages"
	rec = doJSON(t, router, http.MethodPost, path, postMessageRequest{Body: "oi bob"}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("post mensagem deveria 201, obtido %d: %s", rec.Code, rec.Body.String())
	}

	var logs []store.AuditLog
	if err := app.Store.DB.Where("action = ?", "social.message").Find(&logs).Error; err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("esperava 1 audit social.message, obtido %d", len(logs))
	}
	if logs[0].Detail == "" || strings.Contains(logs[0].Detail, "oi bob") {
		t.Fatalf("audit não pode conter o corpo da mensagem: %q", logs[0].Detail)
	}
}

func TestSocialGroup_DoesNotLeakToNonMember(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	createTestUserWithRole(t, app, "bob", "senha-bob-ok", store.RoleMember)
	createTestUserWithRole(t, app, "eve", "senha-eve-ok", store.RoleMember)
	router := NewRouter(app)
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-ok")
	eveTok := loginAndGetToken(t, app, router, "eve", "senha-eve-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/social/groups", createGroupRequest{Name: "ops"}, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar grupo: %d %s", rec.Code, rec.Body.String())
	}
	var g socialGroupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &g); err != nil {
		t.Fatalf("grupo: %v", err)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/social/groups/"+strconv.FormatUint(uint64(g.ID), 10)+"/invite",
		joinGroupRequest{Username: "bob"}, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("convidar bob: %d %s", rec.Code, rec.Body.String())
	}

	msgPath := "/api/social/threads/group/" + strconv.FormatUint(uint64(g.ID), 10) + "/messages"
	rec = doJSON(t, router, http.MethodPost, msgPath, postMessageRequest{Body: "segredo"}, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("alice post: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, msgPath, nil, bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("bob membro deveria ler: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, msgPath, nil, eveTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("eve não-membro deveria 403, obtido %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, msgPath, postMessageRequest{Body: "intrusa"}, eveTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("eve post deveria 403, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSocialMessages_Pagination(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	createTestUserWithRole(t, app, "bob", "senha-bob-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/social/threads", openThreadRequest{Username: "bob"}, token)
	var th socialThreadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &th); err != nil {
		t.Fatalf("thread: %v", err)
	}
	path := "/api/social/threads/dm/" + strconv.FormatUint(uint64(th.ID), 10) + "/messages"
	for i := 0; i < 12; i++ {
		rec = doJSON(t, router, http.MethodPost, path, postMessageRequest{Body: "m" + strconv.Itoa(i)}, token)
		if rec.Code != http.StatusCreated {
			t.Fatalf("post %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}

	rec = doJSON(t, router, http.MethodGet, path+"?page=1&per_page=5", nil, token)
	env := decodePage[socialMessageResponse](t, rec.Body.Bytes())
	items := pageItems[socialMessageResponse](t, env)
	if env.Total != 12 || env.PerPage != 5 || len(items) != 5 {
		t.Fatalf("página 1 inesperada: total=%d per=%d n=%d", env.Total, env.PerPage, len(items))
	}

	rec = doJSON(t, router, http.MethodGet, path+"?page=3&per_page=5", nil, token)
	env = decodePage[socialMessageResponse](t, rec.Body.Bytes())
	items = pageItems[socialMessageResponse](t, env)
	if env.Total != 12 || len(items) != 2 {
		t.Fatalf("última página deveria ter 2 items, obtido %d total=%d", len(items), env.Total)
	}
}
