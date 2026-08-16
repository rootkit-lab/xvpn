package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func uploadSocialFile(t *testing.T, router http.Handler, token, filename, mime string, content []byte) uint {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("form: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/social/attachments", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == 0 {
		t.Fatalf("resposta de upload inválida: %s", rec.Body.String())
	}
	_ = mime
	return out.ID
}

func TestSocialAttachment_UploadAndACL(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	createTestUserWithRole(t, app, "bob", "senha-bob-ok", store.RoleMember)
	createTestUserWithRole(t, app, "eve", "senha-eve-ok", store.RoleMember)
	router := NewRouter(app)
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-ok")
	eveTok := loginAndGetToken(t, app, router, "eve", "senha-eve-ok")

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	attID := uploadSocialFile(t, router, aliceTok, "foto.png", "image/png", png)

	rec := doJSON(t, router, http.MethodPost, "/api/social/threads", openThreadRequest{Username: "bob"}, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("thread: %d %s", rec.Code, rec.Body.String())
	}
	var th socialThreadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &th); err != nil {
		t.Fatal(err)
	}
	path := "/api/social/threads/dm/" + strconv.FormatUint(uint64(th.ID), 10) + "/messages"
	rec = doJSON(t, router, http.MethodPost, path, postMessageRequest{Kind: "image", AttachmentID: &attID, Body: "olha"}, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("post mídia: %d %s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/social/attachments/"+strconv.FormatUint(uint64(attID), 10), nil)
	req.Header.Set("Authorization", "Bearer "+bobTok)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bob deveria baixar o anexo, obtido %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("Content-Type deveria ser image/png, obtido %q", ct)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/social/attachments/"+strconv.FormatUint(uint64(attID), 10), nil)
	req.Header.Set("Authorization", "Bearer "+eveTok)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("eve não deveria baixar, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSocialStory_CreateListView(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	createTestUserWithRole(t, app, "bob", "senha-bob-ok", store.RoleMember)
	router := NewRouter(app)
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/social/stories", createStoryRequest{Kind: "text", Body: "olá"}, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar story: %d %s", rec.Code, rec.Body.String())
	}
	var created storyItemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == 0 {
		t.Fatalf("story inválida: %s", rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/social/stories", nil, bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list stories: %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []storyAuthorResponse `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || !list.Items[0].Unseen {
		t.Fatalf("bob deveria ver story não vista: %+v", list)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/social/stories/"+strconv.FormatUint(uint64(created.ID), 10)+"/view", nil, bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("view: %d %s", rec.Code, rec.Body.String())
	}
}
