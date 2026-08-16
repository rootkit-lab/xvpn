package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestSocialFeedCreateAndList(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/social/posts", createPostRequest{Body: "primeiro post"}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/social/feed", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed: %d %s", rec.Code, rec.Body.String())
	}
	env := decodePage[socialPostResponse](t, rec.Body.Bytes())
	items := pageItems[socialPostResponse](t, env)
	if env.Total < 1 || len(items) < 1 || items[0].Body != "primeiro post" {
		t.Fatalf("feed inesperado: %+v", env)
	}
}

func TestSocialCreatePostRejectsTooLong(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "bob", "senha-bob-okxx", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "bob", "senha-bob-okxx")
	rec := doJSON(t, router, http.MethodPost, "/api/social/posts", createPostRequest{Body: strings.Repeat("a", 400)}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400, veio %d", rec.Code)
	}
}
