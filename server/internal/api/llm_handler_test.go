package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestCodespaceSettings_KeyIsWriteOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, router, adminTok := setupGitApp(t)

	rec := doJSON(t, router, http.MethodGet, "/api/config/xcodespaces", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "f9d3") || strings.Contains(rec.Body.String(), `"api_key"`) {
		t.Fatal("GET não pode devolver a key")
	}

	rec = doJSON(t, router, http.MethodPatch, "/api/config/xcodespaces", patchCodespaceSettingsRequest{
		Provider: ptr("glm"),
		APIKey:   ptr("sk-test-key-not-a-real-secret-xx"),
		Model:    ptr("glm-4-flash"),
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", rec.Code, rec.Body.String())
	}
	var got codespaceSettingsJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.HasKey || got.Provider != "glm" || got.Model != "glm-4-flash" {
		t.Fatalf("settings: %+v", got)
	}
	if strings.Contains(rec.Body.String(), "sk-test") {
		t.Fatal("PATCH response vazou a key")
	}

	rec = doJSON(t, router, http.MethodPatch, "/api/config/xcodespaces", patchCodespaceSettingsRequest{
		APIKey: ptr(""),
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH keep: %d", rec.Code)
	}
}

func TestParseLLMBaseURL_Allowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if _, err := parseLLMBaseURL("http://evil.example/v1"); err == nil {
		t.Fatal("http fora da allowlist")
	}
	if _, err := parseLLMBaseURL("https://evil.example/v1"); err == nil {
		t.Fatal("host fora da allowlist")
	}
	if _, err := parseLLMBaseURL("https://user:pass@open.bigmodel.cn/api/paas/v4"); err == nil {
		t.Fatal("userinfo")
	}
	if _, err := parseLLMBaseURL("https://open.bigmodel.cn/api/paas/v4"); err != nil {
		t.Fatal(err)
	}
}

func TestLLMCommitMessage_UsesAdminSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, router, adminTok := setupGitApp(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-llm-key-value-xx" {
			t.Errorf("auth: %s", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "feat(teste): adiciona playground"}},
			},
		})
	}))
	t.Cleanup(upstream.Close)
	app.llmHTTP = upstream.Client()
	if err := app.Store.DB.Save(&store.CodespaceSettings{
		ID: 1, Provider: "glm", BaseURL: upstream.URL, Model: "glm-4-flash", APIKey: "test-llm-key-value-xx",
	}).Error; err != nil {
		t.Fatal(err)
	}

	rec := doJSONHost(t, router, http.MethodPost, "/api/xcodespaces/llm/commit-message", llmCommitRequest{
		Diff: "diff --git a/x.go b/x.go\n+func main() {}",
	}, adminTok, "xcodespaces.corp.ihuull.com")
	if rec.Code != http.StatusOK {
		t.Fatalf("commit-message: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "feat(teste):") {
		t.Fatalf("body: %s", rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/xcodespaces/llm/commit-message", llmCommitRequest{Diff: "x"}, adminTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("host público deveria bloquear: %d", rec.Code)
	}
}

func TestLLMCommitMessage_AcceptsCodespaceGitToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, router, adminTok := setupGitApp(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "fix(codespace): usa url absoluta"}},
			},
		})
	}))
	t.Cleanup(upstream.Close)
	app.llmHTTP = upstream.Client()
	if err := app.Store.DB.Save(&store.CodespaceSettings{
		ID: 1, Provider: "glm", BaseURL: upstream.URL, Model: "glm-4-flash", APIKey: "test-llm-key-value-xx",
	}).Error; err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", rec.Code, rec.Body.String())
	}
	var admin store.User
	if err := app.Store.DB.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	var lab store.Project
	if err := app.Store.DB.Where("slug = ?", "lab").First(&lab).Error; err != nil {
		t.Fatal(err)
	}
	gitTok := "tokentokentoken1"
	cs := store.CodeSpace{
		PublicID:     "aabbccddeeff",
		UserID:       admin.ID,
		ProjectID:    lab.ID,
		Branch:       "main",
		RelPath:      "admin/lab/aabbccddeeff",
		Kind:         store.CodespaceKindRemote,
		Status:       store.CodespaceRunning,
		GitTokenHash: hashCodespaceToken(gitTok),
	}
	if err := app.Store.DB.Create(&cs).Error; err != nil {
		t.Fatal(err)
	}

	rec = doJSONHost(t, router, http.MethodPost, "/api/xcodespaces/llm/commit-message", llmCommitRequest{
		Diff: "diff --git a/x.go b/x.go\n+func main() {}",
	}, gitTok, "cs-aabbccddeeff.corp.ihuull.com")
	if rec.Code != http.StatusOK {
		t.Fatalf("git token no cs-*: %d %s", rec.Code, rec.Body.String())
	}

	rec = doLLMPost(t, router, "/api/xcodespaces/llm/commit-message", llmCommitRequest{Diff: "x"}, gitTok, "xcodespaces.corp.ihuull.com", map[string]string{
		codespaceIDHeader: "aabbccddeeff",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("git token + header: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSONHost(t, router, http.MethodPost, "/api/xcodespaces/llm/commit-message", llmCommitRequest{Diff: "x"}, gitTok, "xvpn.ihuull.com")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("host público: %d", rec.Code)
	}

	rec = doJSONHost(t, router, http.MethodPost, "/api/xcodespaces/llm/commit-message", llmCommitRequest{Diff: "x"}, "wrongtokenwrongtok", "cs-aabbccddeeff.corp.ihuull.com")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("token errado: %d", rec.Code)
	}

	rec = doJSONHost(t, router, http.MethodGet, "/api/xcodespaces", nil, gitTok, "xcodespaces.corp.ihuull.com")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("token git não pode listar codespaces: %d", rec.Code)
	}
}

func doLLMPost(t *testing.T, router http.Handler, path string, body any, token, host string, extra map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(raw)))
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	return out
}

func TestCodespaceHostAllowsLLMNotControlAPI(t *testing.T) {
	_, router, adminTok := setupGitApp(t)
	host := "cs-aabbccddeeff.corp.ihuull.com"

	rec := doJSONHost(t, router, http.MethodGet, "/api/xcodespaces", nil, adminTok, host)
	if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), `"items"`) {
		t.Fatal("lista vazou no cs-*")
	}

	rec = doJSONHost(t, router, http.MethodPost, "/api/xcodespaces/llm/commit-message", llmCommitRequest{Diff: "x"}, adminTok, host)
	if rec.Code == http.StatusNotFound {
		t.Fatalf("LLM no cs-* deve chegar ao Gin: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
		t.Fatalf("LLM no cs-* inesperado: %d %s", rec.Code, rec.Body.String())
	}
}

func ptr[T any](v T) *T { return &v }
