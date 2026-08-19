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
	if !strings.Contains(rec.Body.String(), `"glm-4.7-flash"`) {
		t.Fatal("GET deve incluir o catálogo de modelos")
	}

	rec = doJSON(t, router, http.MethodPatch, "/api/config/xcodespaces", patchCodespaceSettingsRequest{
		Provider: ptr("glm"),
		APIKey:   ptr("sk-test-key-not-a-real-secret-xx"),
		Model:    ptr("glm-4.7-flash"),
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", rec.Code, rec.Body.String())
	}
	var got codespaceSettingsJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.HasKey || got.Provider != "glm" || got.Model != "glm-4.7-flash" {
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

func TestLLMSettings_TestPingUsesDraft(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, router, adminTok := setupGitApp(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer draft-llm-key-value-xx" {
			t.Errorf("auth: %s", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "ok"}},
			},
		})
	}))
	t.Cleanup(upstream.Close)
	app.llmHTTP = upstream.Client()
	if err := app.Store.DB.Save(&store.CodespaceSettings{
		ID: 1, Provider: "glm", BaseURL: upstream.URL, Model: "glm-4-flash", APIKey: "stored-key-not-used-xx",
	}).Error; err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, router, http.MethodPost, "/api/config/xcodespaces/test", testCodespaceLLMRequest{
		Model:  ptr("glm-4.7-flash"),
		APIKey: ptr("draft-llm-key-value-xx"),
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("test: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("body: %s", rec.Body.String())
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

func TestDisableGLMThinking(t *testing.T) {
	if !disableGLMThinking("glm", "glm-4.7-flash") {
		t.Fatal("4.7-flash deve desligar thinking")
	}
	if disableGLMThinking("glm", "glm-5.3") {
		t.Fatal("5.3 rejeita thinking disabled")
	}
	if disableGLMThinking("openai", "gpt-4o-mini") {
		t.Fatal("openai não manda thinking")
	}
}

func TestExtractOpenAIChatText(t *testing.T) {
	got, err := extractOpenAIChatText([]byte(`{"choices":[{"message":{"content":"","reasoning_content":"feat(x): y"}}]}`))
	if err != nil || got != "feat(x): y" {
		t.Fatalf("reasoning: %q %v", got, err)
	}
	got, err = extractOpenAIChatText([]byte(`{"choices":[{"message":{"content":[{"type":"text","text":"ok"}]}}]}`))
	if err != nil || got != "ok" {
		t.Fatalf("parts: %q %v", got, err)
	}
	if _, err := extractOpenAIChatText([]byte(`{"choices":[{"message":{"content":""}}]}`)); err == nil {
		t.Fatal("vazio deveria falhar")
	}
	res, err := extractOpenAIChatResult([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c1","function":{"name":"read_file","arguments":"{\"path\":\"AGENTS.md\"}"}}]}}]}`))
	if err != nil || len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "read_file" {
		t.Fatalf("tool_calls: %+v %v", res, err)
	}
}

func TestLLMCommitMessage_UsesAdminSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, router, adminTok := setupGitApp(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-llm-key-value-xx" {
			t.Errorf("auth: %s", r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		thinking, _ := body["thinking"].(map[string]any)
		if thinking["type"] != "disabled" {
			t.Errorf("thinking: %#v", body["thinking"])
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

func TestLLMCommitMessage_UsesReasoningWhenContentEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, router, adminTok := setupGitApp(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "", "reasoning_content": "feat(codespace): gera commit"}},
			},
		})
	}))
	t.Cleanup(upstream.Close)
	app.llmHTTP = upstream.Client()
	if err := app.Store.DB.Save(&store.CodespaceSettings{
		ID: 1, Provider: "glm", BaseURL: upstream.URL, Model: "glm-4.7-flash", APIKey: "test-llm-key-value-xx",
	}).Error; err != nil {
		t.Fatal(err)
	}

	rec := doJSONHost(t, router, http.MethodPost, "/api/xcodespaces/llm/commit-message", llmCommitRequest{
		Diff: "diff --git a/x.go b/x.go\n+func main() {}",
	}, adminTok, "xcodespaces.corp.ihuull.com")
	if rec.Code != http.StatusOK {
		t.Fatalf("commit-message: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "feat(codespace):") {
		t.Fatalf("body: %s", rec.Body.String())
	}
}

func TestLLMCommitMessage_EmptyProviderResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, router, adminTok := setupGitApp(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": ""}},
			},
		})
	}))
	t.Cleanup(upstream.Close)
	app.llmHTTP = upstream.Client()
	if err := app.Store.DB.Save(&store.CodespaceSettings{
		ID: 1, Provider: "glm", BaseURL: upstream.URL, Model: "glm-4.7-flash", APIKey: "test-llm-key-value-xx",
	}).Error; err != nil {
		t.Fatal(err)
	}

	rec := doJSONHost(t, router, http.MethodPost, "/api/xcodespaces/llm/commit-message", llmCommitRequest{
		Diff: "diff --git a/x.go b/x.go\n+func main() {}",
	}, adminTok, "xcodespaces.corp.ihuull.com")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("vazio: %d %s", rec.Code, rec.Body.String())
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

func TestLLMChat_ContextAndToolCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, router, adminTok := setupGitApp(t)
	var saw struct {
		hasContext bool
		hasTools   bool
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(body["messages"])
		saw.hasContext = strings.Contains(string(raw), "AGENTS.md")
		_, saw.hasTools = body["tools"]
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{
					"content": "",
					"tool_calls": []map[string]any{
						{"id": "c1", "function": map[string]string{"name": "read_file", "arguments": `{"path":"AGENTS.md"}`}},
					},
				}},
			},
		})
	}))
	t.Cleanup(upstream.Close)
	app.llmHTTP = upstream.Client()
	if err := app.Store.DB.Save(&store.CodespaceSettings{
		ID: 1, Provider: "glm", BaseURL: upstream.URL, Model: "glm-4.7-flash", APIKey: "test-llm-key-value-xx",
	}).Error; err != nil {
		t.Fatal(err)
	}

	rec := doJSONHost(t, router, http.MethodPost, "/api/xcodespaces/llm/chat", llmChatRequest{
		Messages: []llmMessage{{Role: "user", Content: "leia o AGENTS"}},
		Context:  "## AGENTS.md\nVPN privada",
		Tools: []llmToolSpec{{
			Type: "function",
			Function: llmToolFnSpec{
				Name:        "read_file",
				Description: "lê arquivo",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		}},
	}, adminTok, "xcodespaces.corp.ihuull.com")
	if rec.Code != http.StatusOK {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"tool_calls"`) || !strings.Contains(rec.Body.String(), "read_file") {
		t.Fatalf("body: %s", rec.Body.String())
	}
	if !saw.hasContext || !saw.hasTools {
		t.Fatalf("upstream context=%v tools=%v", saw.hasContext, saw.hasTools)
	}
}

func TestSanitizeLLMMessages_CapsContext(t *testing.T) {
	huge := strings.Repeat("x", maxLLMContextBytes+80)
	out, err := sanitizeLLMMessages([]llmMessage{{Role: "user", Content: "oi"}}, huge)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out[0].Content, "Contexto do workspace") {
		t.Fatal("system deve incluir context")
	}
	if len(out[0].Content) > maxLLMContextBytes+400 {
		t.Fatalf("context não foi capado: %d", len(out[0].Content))
	}
}

func ptr[T any](v T) *T { return &v }
