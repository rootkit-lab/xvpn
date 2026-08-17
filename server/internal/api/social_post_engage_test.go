package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestSocialPost_StarCommentRepost(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	createTestUserWithRole(t, app, "bob", "senha-bob-okxx", store.RoleMember)
	router := NewRouter(app)
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-okxx")

	rec := doJSON(t, router, http.MethodPost, "/api/social/posts", createPostRequest{Body: "olá mundo"}, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var post socialPostResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	path := "/api/social/posts/" + strconv.FormatUint(uint64(post.ID), 10)

	rec = doJSON(t, router, http.MethodPost, path+"/star", nil, bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("star: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/social/feed", nil, bobTok)
	items := pageItems[socialPostResponse](t, decodePage[socialPostResponse](t, rec.Body.Bytes()))
	if len(items) == 0 || !items[0].Starred || items[0].Stars != 1 {
		t.Fatalf("feed após estrela: %+v", items)
	}

	rec = doJSON(t, router, http.MethodPost, path+"/comments", createCommentRequest{Body: "legal"}, bobTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("comment: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, path+"/comments", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list comments: %d %s", rec.Code, rec.Body.String())
	}
	cenv := decodePage[socialCommentResponse](t, rec.Body.Bytes())
	citems := pageItems[socialCommentResponse](t, cenv)
	if cenv.Total != 1 || len(citems) != 1 || citems[0].Body != "legal" {
		t.Fatalf("comentários: %+v", cenv)
	}

	rec = doJSON(t, router, http.MethodPost, path+"/repost", nil, aliceTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("autor não deveria repostar o próprio, obtido %d", rec.Code)
	}
	rec = doJSON(t, router, http.MethodPost, path+"/repost", nil, bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("repost: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/social/u/bob/posts", nil, aliceTok)
	bobItems := pageItems[socialPostResponse](t, decodePage[socialPostResponse](t, rec.Body.Bytes()))
	if len(bobItems) != 1 || bobItems[0].Kind != "repost" || bobItems[0].Original == nil || bobItems[0].Original.Body != "olá mundo" {
		t.Fatalf("repost no perfil: %+v", bobItems)
	}
}
