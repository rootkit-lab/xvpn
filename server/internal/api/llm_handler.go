package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/provision"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const (
	defaultGLMBaseURL     = "https://open.bigmodel.cn/api/paas/v4"
	defaultGLMModel       = "glm-4.7-flash"
	maxLLMKeyLen          = 256
	maxLLMDiffBytes       = 8 << 10
	maxLLMMsgBytes        = 4 << 10
	maxLLMToolResultBytes = 16 << 10
	maxLLMChatMsgs        = 80
	maxLLMContextBytes    = 24 << 10
	maxLLMChatTokens      = 2048
	maxLLMTools           = 16
	llmHTTPTimeout        = 45 * time.Second
)

var allowedLLMHosts = map[string]struct{}{
	"open.bigmodel.cn":  {},
	"api.z.ai":          {},
	"api.openai.com":    {},
	"api.anthropic.com": {},
	"api.groq.com":      {},
}

type codespaceSettingsJSON struct {
	Provider string                      `json:"provider"`
	BaseURL  string                      `json:"base_url"`
	Model    string                      `json:"model"`
	HasKey   bool                        `json:"has_key"`
	Catalog  map[string][]llmModelOption `json:"catalog"`
}

type testCodespaceLLMRequest struct {
	Provider *string `json:"provider"`
	BaseURL  *string `json:"base_url"`
	Model    *string `json:"model"`
	APIKey   *string `json:"api_key"`
}

type patchCodespaceSettingsRequest struct {
	Provider *string `json:"provider"`
	BaseURL  *string `json:"base_url"`
	Model    *string `json:"model"`
	APIKey   *string `json:"api_key"`
}

type llmToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type llmToolSpec struct {
	Type     string        `json:"type"`
	Function llmToolFnSpec `json:"function"`
}

type llmToolFnSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type llmMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content"`
	Name       string        `json:"name,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolCalls  []llmToolCall `json:"tool_calls,omitempty"`
}

type llmChatRequest struct {
	Messages []llmMessage  `json:"messages"`
	Context  string        `json:"context"`
	Tools    []llmToolSpec `json:"tools"`
	Model    string        `json:"model"`
	Mode     string        `json:"mode"`
}

type llmModelsJSON struct {
	Provider string           `json:"provider"`
	Model    string           `json:"model"`
	HasKey   bool             `json:"has_key"`
	Catalog  []llmModelOption `json:"catalog"`
	GitName  string           `json:"git_name,omitempty"`
	GitEmail string           `json:"git_email,omitempty"`
}

type llmResult struct {
	Text      string
	ToolCalls []llmToolCall
}

type llmCommitRequest struct {
	Diff string `json:"diff"`
}

func (a *App) handleGetCodespaceSettings(c *gin.Context) {
	row, err := a.loadOrInitCodespaceSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao ler settings"})
		return
	}
	c.JSON(http.StatusOK, codespaceSettingsJSON{
		Provider: displayLLMProvider(row.Provider),
		BaseURL:  displayLLMBaseURL(row.BaseURL, row.Provider),
		Model:    displayLLMModel(row.Model, row.Provider),
		HasKey:   strings.TrimSpace(row.APIKey) != "",
		Catalog:  llmModelCatalog(),
	})
}

func (a *App) handlePatchCodespaceSettings(c *gin.Context) {
	var req patchCodespaceSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}
	row, err := a.loadOrInitCodespaceSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao ler settings"})
		return
	}
	if req.Provider != nil {
		p := normalizeLLMProvider(*req.Provider)
		if p == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provider inválido"})
			return
		}
		row.Provider = p
	}
	if req.BaseURL != nil {
		raw := strings.TrimSpace(*req.BaseURL)
		if raw != "" {
			if _, err := parseLLMBaseURL(raw); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		row.BaseURL = raw
	}
	if req.Model != nil {
		m := strings.TrimSpace(*req.Model)
		if utf8.RuneCountInString(m) > 64 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "modelo inválido"})
			return
		}
		row.Model = m
	}
	if req.APIKey != nil {
		key := strings.TrimSpace(*req.APIKey)
		if key == "" {
			// vazio = mantém a key atual
		} else if len(key) > maxLLMKeyLen || strings.ContainsAny(key, "\n\r") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "api_key inválida"})
			return
		} else {
			row.APIKey = key
		}
	}
	if err := a.Store.DB.Save(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gravar settings"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "xcodespaces.settings", row.Provider)
	a.handleGetCodespaceSettings(c)
}

func (a *App) handleTestCodespaceLLM(c *gin.Context) {
	var req testCodespaceLLMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}
	row, err := a.loadOrInitCodespaceSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao ler settings"})
		return
	}
	if req.Provider != nil {
		p := normalizeLLMProvider(*req.Provider)
		if p == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provider inválido"})
			return
		}
		row.Provider = p
	}
	if req.BaseURL != nil {
		row.BaseURL = strings.TrimSpace(*req.BaseURL)
	}
	if req.Model != nil {
		m := strings.TrimSpace(*req.Model)
		if utf8.RuneCountInString(m) > 64 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "modelo inválido"})
			return
		}
		row.Model = m
	}
	if req.APIKey != nil {
		key := strings.TrimSpace(*req.APIKey)
		if key != "" {
			if len(key) > maxLLMKeyLen || strings.ContainsAny(key, "\n\r") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "api_key inválida"})
				return
			}
			row.APIKey = key
		}
	}
	if strings.TrimSpace(row.APIKey) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "configure a key em xadmin → Settings"})
		return
	}
	base := displayLLMBaseURL(row.BaseURL, row.Provider)
	u, err := parseLLMBaseURL(base)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model := displayLLMModel(row.Model, row.Provider)
	provider := displayLLMProvider(row.Provider)
	text, err := a.completeLLM(u, provider, model, row.APIKey, []llmMessage{
		{Role: "user", Content: "Responda só: ok"},
	}, 8)
	if err != nil {
		writeLLMError(c, err)
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "xcodespaces.llm.test", provider+" "+model)
	c.JSON(http.StatusOK, gin.H{"ok": true, "model": model, "text": firstLine(text)})
}

func (a *App) handleLLMModels(c *gin.Context) {
	row, err := a.loadOrInitCodespaceSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao ler settings"})
		return
	}
	c.JSON(http.StatusOK, llmModelsPayload(row, callerUsername(c)))
}

func (a *App) handleLLMChat(c *gin.Context) {
	var req llmChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}
	mode, err := normalizeLLMMode(req.Mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	msgs, err := sanitizeLLMMessages(req.Messages, req.Context, mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tools := filterToolsForMode(mode, sanitizeLLMTools(req.Tools))
	res, err := a.callProjectLLMChat(c, msgs, tools, maxLLMChatTokens, req.Model)
	if err != nil {
		writeLLMError(c, err)
		return
	}
	if len(res.ToolCalls) > 0 {
		c.JSON(http.StatusOK, gin.H{"tool_calls": res.ToolCalls})
		return
	}
	c.JSON(http.StatusOK, gin.H{"text": res.Text})
}

func (a *App) handleLLMCommitMessage(c *gin.Context) {
	var req llmCommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}
	diff := strings.TrimSpace(req.Diff)
	if diff == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "diff vazio — faça stage no Source Control"})
		return
	}
	if len(diff) > maxLLMDiffBytes {
		diff = diff[:maxLLMDiffBytes]
	}
	msgs := []llmMessage{
		{Role: "system", Content: "Você escreve uma única linha Conventional Commits (feat/fix/docs/chore/refactor/test/security/perf). Sem markdown, sem aspas, sem explicação."},
		{Role: "user", Content: "Diff:\n" + diff},
	}
	text, err := a.callProjectLLM(c, msgs, 256)
	if err != nil {
		writeLLMError(c, err)
		return
	}
	msg := firstLine(text)
	if msg == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "resposta vazia do provedor"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

func (a *App) callProjectLLM(c *gin.Context, msgs []llmMessage, maxTokens int) (string, error) {
	row, err := a.loadOrInitCodespaceSettings()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(row.APIKey) == "" {
		return "", errLLM("configure a key em xadmin → Settings", http.StatusServiceUnavailable)
	}
	base := displayLLMBaseURL(row.BaseURL, row.Provider)
	u, err := parseLLMBaseURL(base)
	if err != nil {
		return "", errLLM(err.Error(), http.StatusBadRequest)
	}
	model := displayLLMModel(row.Model, row.Provider)
	provider := displayLLMProvider(row.Provider)
	_ = a.Store.LogAudit(callerUsername(c), "xcodespaces.llm", provider)
	return a.completeLLM(u, provider, model, row.APIKey, msgs, maxTokens)
}

func (a *App) callProjectLLMChat(c *gin.Context, msgs []llmMessage, tools []llmToolSpec, maxTokens int, modelOverride string) (llmResult, error) {
	row, err := a.loadOrInitCodespaceSettings()
	if err != nil {
		return llmResult{}, err
	}
	if strings.TrimSpace(row.APIKey) == "" {
		return llmResult{}, errLLM("configure a key em xadmin → Settings", http.StatusServiceUnavailable)
	}
	base := displayLLMBaseURL(row.BaseURL, row.Provider)
	u, err := parseLLMBaseURL(base)
	if err != nil {
		return llmResult{}, errLLM(err.Error(), http.StatusBadRequest)
	}
	provider := displayLLMProvider(row.Provider)
	model, err := resolveChatModel(provider, displayLLMModel(row.Model, row.Provider), modelOverride)
	if err != nil {
		return llmResult{}, err
	}
	_ = a.Store.LogAudit(callerUsername(c), "xcodespaces.llm", provider+" "+model)
	return a.completeLLMChat(u, provider, model, row.APIKey, msgs, tools, maxTokens)
}

func (a *App) completeLLM(base *url.URL, provider, model, key string, msgs []llmMessage, maxTokens int) (string, error) {
	res, err := a.completeLLMChat(base, provider, model, key, msgs, nil, maxTokens)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(res.Text) == "" {
		return "", errLLM("resposta vazia do provedor", http.StatusBadGateway)
	}
	return res.Text, nil
}

func (a *App) completeLLMChat(base *url.URL, provider, model, key string, msgs []llmMessage, tools []llmToolSpec, maxTokens int) (llmResult, error) {
	client := a.llmHTTP
	if client == nil {
		client = &http.Client{Timeout: llmHTTPTimeout}
	}
	if provider == "anthropic" {
		text, err := completeAnthropic(client, base, model, key, msgs, maxTokens)
		if err != nil {
			return llmResult{}, err
		}
		return llmResult{Text: text}, nil
	}
	return completeOpenAICompat(client, base, provider, model, key, msgs, tools, maxTokens)
}

func completeOpenAICompat(client *http.Client, base *url.URL, provider, model, key string, msgs []llmMessage, tools []llmToolSpec, maxTokens int) (llmResult, error) {
	endpoint := strings.TrimRight(base.String(), "/") + "/chat/completions"
	payload := map[string]any{
		"model":       model,
		"messages":    providerMessages(msgs),
		"max_tokens":  maxTokens,
		"temperature": 0.2,
	}
	if disableGLMThinking(provider, model) {
		payload["thinking"] = map[string]string{"type": "disabled"}
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return llmResult{}, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return llmResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return llmResult{}, errLLM("provedor indisponível", http.StatusBadGateway)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return llmResult{}, errLLM(providerRejectMessage(raw, resp.StatusCode), http.StatusBadGateway)
	}
	return extractOpenAIChatResult(raw)
}

func disableGLMThinking(provider, model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if displayLLMProvider(provider) != "glm" && !strings.HasPrefix(m, "glm-") {
		return false
	}
	// GLM-5.3 rejeita thinking.type=disabled; o texto vai em reasoning_content.
	return !strings.HasPrefix(m, "glm-5.3")
}

func extractOpenAIChatText(raw []byte) (string, error) {
	res, err := extractOpenAIChatResult(raw)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(res.Text) == "" {
		return "", errLLM("resposta vazia do provedor", http.StatusBadGateway)
	}
	return res.Text, nil
}

func extractOpenAIChatResult(raw []byte) (llmResult, error) {
	var out struct {
		Choices []struct {
			Message struct {
				Content          json.RawMessage `json:"content"`
				ReasoningContent string          `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return llmResult{}, errLLM("resposta inválida do provedor", http.StatusBadGateway)
	}
	msg := out.Choices[0].Message
	text := openAIContentText(msg.Content)
	if text == "" {
		text = strings.TrimSpace(msg.ReasoningContent)
	}
	calls := make([]llmToolCall, 0, len(msg.ToolCalls))
	for _, tc := range msg.ToolCalls {
		name := strings.TrimSpace(tc.Function.Name)
		if name == "" {
			continue
		}
		id := strings.TrimSpace(tc.ID)
		if id == "" {
			id = fmt.Sprintf("call_%d", len(calls)+1)
		}
		args := tc.Function.Arguments
		if len(args) > maxLLMToolResultBytes {
			args = args[:maxLLMToolResultBytes]
		}
		calls = append(calls, llmToolCall{ID: id, Name: name, Arguments: args})
	}
	if text == "" && len(calls) == 0 {
		return llmResult{}, errLLM("resposta vazia do provedor", http.StatusBadGateway)
	}
	return llmResult{Text: text, ToolCalls: calls}, nil
}

func providerMessages(msgs []llmMessage) []map[string]any {
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		row := map[string]any{"role": m.Role}
		if m.ToolCallID != "" {
			row["tool_call_id"] = m.ToolCallID
		}
		if m.Name != "" {
			row["name"] = m.Name
		}
		if len(m.ToolCalls) > 0 {
			calls := make([]map[string]any, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				calls = append(calls, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": tc.Arguments,
					},
				})
			}
			row["tool_calls"] = calls
			if m.Content == "" {
				row["content"] = nil
			} else {
				row["content"] = m.Content
			}
		} else {
			row["content"] = m.Content
		}
		out = append(out, row)
	}
	return out
}

func openAIContentText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Text == "" || (p.Type != "" && p.Type != "text") {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p.Text)
	}
	return strings.TrimSpace(b.String())
}

func completeAnthropic(client *http.Client, base *url.URL, model, key string, msgs []llmMessage, maxTokens int) (string, error) {
	sys := ""
	var chat []map[string]string
	for _, m := range msgs {
		if m.Role == "system" {
			sys = m.Content
			continue
		}
		role := m.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		chat = append(chat, map[string]string{"role": role, "content": m.Content})
	}
	payload := map[string]any{"model": model, "max_tokens": maxTokens, "messages": chat}
	if sys != "" {
		payload["system"] = sys
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(base.String(), "/") + "/v1/messages"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", errLLM("provedor indisponível", http.StatusBadGateway)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", errLLM(providerRejectMessage(raw, resp.StatusCode), http.StatusBadGateway)
	}
	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Content) == 0 {
		return "", errLLM("resposta inválida do provedor", http.StatusBadGateway)
	}
	return strings.TrimSpace(out.Content[0].Text), nil
}

func (a *App) loadOrInitCodespaceSettings() (store.CodespaceSettings, error) {
	var s store.CodespaceSettings
	err := a.Store.DB.First(&s, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s = store.CodespaceSettings{ID: 1, Provider: "glm"}
		if err := a.Store.DB.Create(&s).Error; err != nil {
			return s, err
		}
		return s, nil
	}
	return s, err
}

const codespaceIDHeader = "X-Codespace-ID"

func (a *App) requireLLMCaller() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.bindJWELLM(c) || a.bindCodespaceGitLLM(c) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "credenciais ausentes"})
	}
}

func (a *App) bindJWELLM(c *gin.Context) bool {
	if a == nil || a.Tokens == nil {
		return false
	}
	token := auth.TokenFromRequest(c)
	if token == "" {
		return false
	}
	claims, err := a.Tokens.Parse(token)
	if err != nil {
		ck, ckErr := c.Request.Cookie(auth.SessionCookieName)
		if ckErr == nil && ck != nil {
			if alt := strings.TrimSpace(ck.Value); alt != "" && alt != token {
				claims, err = a.Tokens.Parse(alt)
			}
		}
	}
	if err != nil {
		return false
	}
	if auth.IsPackagesScoped(claims) {
		return false
	}
	c.Set(auth.ContextUserIDKey, claims.UserID)
	c.Set(auth.ContextUsernameKey, claims.Username)
	c.Set(auth.ContextRoleKey, claims.Role)
	return true
}

func (a *App) bindCodespaceGitLLM(c *gin.Context) bool {
	tok := ""
	if h := c.GetHeader("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
		tok = strings.TrimSpace(h[7:])
	}
	if tok == "" {
		return false
	}
	id := codespaceRuntimeHost(c.Request.Host)
	if id == "" {
		id = strings.ToLower(strings.TrimSpace(c.GetHeader(codespaceIDHeader)))
	}
	if id == "" {
		return false
	}
	user, _, ok := a.authenticateCodespaceGit("codespace-"+id, tok)
	if !ok {
		return false
	}
	c.Set(auth.ContextUserIDKey, user.ID)
	c.Set(auth.ContextUsernameKey, user.Username)
	c.Set(auth.ContextRoleKey, user.Role)
	return true
}

func (a *App) requireCodespaceLLMHost() gin.HandlerFunc {
	return func(c *gin.Context) {
		if codespaceLLMHostOK(c.Request.Host) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "assistente só na intranet do codespace"})
	}
}

func codespaceLLMHostOK(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	h = strings.ToLower(h)
	if codespaceRuntimeHost(h) != "" {
		return true
	}
	switch h {
	case "xcodespaces.corp.ihuull.com", "xcodespaces.corp.localhost":
		return true
	}
	return false
}

func isCodespaceGinPath(path string) bool {
	p := strings.TrimRight(path, "/")
	if p == "/api/auth/session" || p == "/api/auth/redeem" {
		return true
	}
	return strings.HasPrefix(p, "/api/xcodespaces/llm/")
}

func parseLLMBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil {
		return nil, fmt.Errorf("base_url deve ser https sem credencial")
	}
	host := strings.ToLower(u.Hostname())
	if gin.Mode() == gin.TestMode && (host == "127.0.0.1" || host == "localhost") && (u.Scheme == "http" || u.Scheme == "https") {
		return u, nil
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("base_url deve ser https sem credencial")
	}
	if _, ok := allowedLLMHosts[host]; !ok {
		return nil, fmt.Errorf("host do provedor não está na allowlist")
	}
	return u, nil
}

func normalizeLLMProvider(p string) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "glm", "openai", "anthropic", "compatible":
		return strings.ToLower(strings.TrimSpace(p))
	}
	return ""
}

func displayLLMProvider(p string) string {
	if n := normalizeLLMProvider(p); n != "" {
		return n
	}
	return "glm"
}

func displayLLMBaseURL(raw, provider string) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	switch displayLLMProvider(provider) {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com"
	case "compatible":
		return ""
	default:
		return defaultGLMBaseURL
	}
}

func displayLLMModel(raw, provider string) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	switch displayLLMProvider(provider) {
	case "openai":
		return "gpt-4o-mini"
	case "anthropic":
		return "claude-3-5-haiku-20241022"
	default:
		return defaultGLMModel
	}
}

func llmModelsPayload(row store.CodespaceSettings, username string) llmModelsJSON {
	provider := displayLLMProvider(row.Provider)
	model := displayLLMModel(row.Model, row.Provider)
	catalog := append([]llmModelOption(nil), llmModelCatalog()[provider]...)
	if model != "" && !llmModelInCatalog(provider, model) {
		catalog = append([]llmModelOption{{ID: model, Label: model + " (settings)"}}, catalog...)
	}
	if catalog == nil {
		catalog = []llmModelOption{}
	}
	out := llmModelsJSON{
		Provider: provider,
		Model:    model,
		HasKey:   strings.TrimSpace(row.APIKey) != "",
		Catalog:  catalog,
	}
	if name, email, ok := provision.CodespaceGitIdentity(username); ok {
		out.GitName = name
		out.GitEmail = email
	}
	return out
}

func normalizeLLMMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "agent", "agents":
		return "agent", nil
	case "ask":
		return "ask", nil
	case "plan":
		return "plan", nil
	case "debug":
		return "debug", nil
	default:
		return "", fmt.Errorf("modo inválido")
	}
}

func llmModePrompt(mode string) string {
	switch mode {
	case "ask":
		return "Modo Ask: responda perguntas sobre o código. Não edite arquivos e não use tools."
	case "plan":
		return "Modo Plan: produza um plano passo a passo. Só leia o workspace (read_file, list_dir, grep, read_skill). Não escreva arquivos nem rode terminal."
	case "debug":
		return "Modo Debug: diagnostique o erro no arquivo ou na seleção atuais. Use tools para inspecionar. Só edite se o usuário pediu a correção."
	default:
		return "Modo Agent: use as tools para cumprir o pedido. Leia o workspace antes de editar."
	}
}

func filterToolsForMode(mode string, tools []llmToolSpec) []llmToolSpec {
	if mode == "ask" {
		return nil
	}
	if mode != "plan" {
		return tools
	}
	allow := map[string]struct{}{
		"read_file": {}, "list_dir": {}, "grep": {}, "read_skill": {}, "glob": {},
	}
	out := make([]llmToolSpec, 0, len(tools))
	for _, t := range tools {
		if _, ok := allow[t.Function.Name]; ok {
			out = append(out, t)
		}
	}
	return out
}

func llmModelInCatalog(provider, model string) bool {
	for _, opt := range llmModelCatalog()[provider] {
		if opt.ID == model {
			return true
		}
	}
	return false
}

func resolveChatModel(provider, current, requested string) (string, error) {
	req := strings.TrimSpace(requested)
	if req == "" || req == current {
		return current, nil
	}
	if llmModelInCatalog(provider, req) {
		return req, nil
	}
	return "", errLLM("modelo não está no catálogo do provedor", http.StatusBadRequest)
}

func sanitizeLLMMessages(in []llmMessage, context, mode string) ([]llmMessage, error) {
	if len(in) == 0 || len(in) > maxLLMChatMsgs {
		return nil, fmt.Errorf("informe de 1 a %d mensagens", maxLLMChatMsgs)
	}
	sys := "Você é o agente do XCODESPACES (intranet ihuull). Entende skills (.cursor/skills), AGENTS.md, rules e slash commands. Sem pedir login Microsoft.\n\n" + llmModePrompt(mode)
	if ctx := strings.TrimSpace(context); ctx != "" {
		if len(ctx) > maxLLMContextBytes {
			ctx = ctx[:maxLLMContextBytes]
		}
		sys += "\n\nContexto do workspace:\n" + ctx
	}
	out := make([]llmMessage, 0, len(in)+1)
	out = append(out, llmMessage{Role: "system", Content: sys})
	for _, m := range in {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "user", "assistant", "tool":
		case "system":
			continue
		default:
			role = "user"
		}
		capBytes := maxLLMMsgBytes
		if role == "tool" {
			capBytes = maxLLMToolResultBytes
		}
		content := m.Content
		if len(content) > capBytes {
			content = content[:capBytes]
		}
		item := llmMessage{
			Role:       role,
			Content:    strings.TrimSpace(content),
			Name:       strings.TrimSpace(m.Name),
			ToolCallID: strings.TrimSpace(m.ToolCallID),
		}
		if role == "assistant" {
			item.ToolCalls = sanitizeToolCalls(m.ToolCalls)
		}
		if item.Content == "" && len(item.ToolCalls) == 0 && role != "tool" {
			continue
		}
		if role == "tool" && item.ToolCallID == "" && item.Content == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) < 2 {
		return nil, fmt.Errorf("mensagem vazia")
	}
	return out, nil
}

func sanitizeLLMTools(in []llmToolSpec) []llmToolSpec {
	if len(in) == 0 {
		return nil
	}
	if len(in) > maxLLMTools {
		in = in[:maxLLMTools]
	}
	out := make([]llmToolSpec, 0, len(in))
	for _, t := range in {
		name := strings.TrimSpace(t.Function.Name)
		if name == "" || utf8.RuneCountInString(name) > 64 {
			continue
		}
		typ := strings.TrimSpace(t.Type)
		if typ == "" {
			typ = "function"
		}
		desc := t.Function.Description
		if utf8.RuneCountInString(desc) > 512 {
			desc = string([]rune(desc)[:512])
		}
		params := t.Function.Parameters
		if len(params) > 8<<10 {
			params = nil
		}
		out = append(out, llmToolSpec{
			Type: typ,
			Function: llmToolFnSpec{
				Name:        name,
				Description: desc,
				Parameters:  params,
			},
		})
	}
	return out
}

func sanitizeToolCalls(in []llmToolCall) []llmToolCall {
	if len(in) == 0 {
		return nil
	}
	if len(in) > maxLLMTools {
		in = in[:maxLLMTools]
	}
	out := make([]llmToolCall, 0, len(in))
	for _, tc := range in {
		name := strings.TrimSpace(tc.Name)
		if name == "" {
			continue
		}
		id := strings.TrimSpace(tc.ID)
		if id == "" {
			id = fmt.Sprintf("call_%d", len(out)+1)
		}
		args := tc.Arguments
		if len(args) > maxLLMToolResultBytes {
			args = args[:maxLLMToolResultBytes]
		}
		out = append(out, llmToolCall{ID: id, Name: name, Arguments: args})
	}
	return out
}

func providerRejectMessage(raw []byte, status int) string {
	var e struct {
		Error json.RawMessage `json:"error"`
		Msg   string          `json:"msg"`
	}
	if json.Unmarshal(raw, &e) == nil {
		var obj struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(e.Error, &obj) == nil && strings.TrimSpace(obj.Message) != "" {
			return firstLine(obj.Message)
		}
		if s := strings.Trim(string(e.Error), `"`); s != "" && !strings.HasPrefix(s, "{") {
			return firstLine(s)
		}
		if strings.TrimSpace(e.Msg) != "" {
			return firstLine(e.Msg)
		}
	}
	return fmt.Sprintf("provedor recusou o pedido (%d)", status)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.Trim(s, "`\"' ")
}

type llmHTTPError struct {
	msg  string
	code int
}

func (e llmHTTPError) Error() string { return e.msg }

func errLLM(msg string, code int) error { return llmHTTPError{msg: msg, code: code} }

func writeLLMError(c *gin.Context, err error) {
	var le llmHTTPError
	if errors.As(err, &le) {
		c.JSON(le.code, gin.H{"error": le.msg})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": "falha no assistente"})
}
