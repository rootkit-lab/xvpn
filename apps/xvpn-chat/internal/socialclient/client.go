// Package socialclient é o cliente HTTP/WebSocket do protocolo social
// (Fase 19.3) usado pelo xvpn-chat. O JWT fica só em memória — nunca em
// disco (mesmo padrão da tela Apps do xvpn-client, Fase 12).
package socialclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DefaultBaseURL é a API do messenger na intranet (só resolve com VPN).
// Login/enroll do painel público fica em https://xvpn.ihuull.com.
const DefaultBaseURL = "https://xchat.corp.ihuull.com"

// PublicIssuerURL é o issuer JWE / painel (Fase 24/27).
const PublicIssuerURL = "https://xvpn.ihuull.com"

var ErrNotLoggedIn = errors.New("sessão expirada ou inexistente — faça login novamente")

type Client struct {
	httpClient *http.Client

	mu       sync.Mutex
	baseURL  string
	token    string
	username string
	role     string
	userID   uint
	ws       *websocket.Conn
	pending  []any
}

func New() *Client {
	return &Client{httpClient: &http.Client{Timeout: 30 * time.Second}}
}

type Session struct {
	LoggedIn bool   `json:"loggedIn"`
	Username string `json:"username"`
	Role     string `json:"role"`
	UserID   uint   `json:"userId"`
}

func (c *Client) Session() Session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Session{LoggedIn: c.token != "", Username: c.username, Role: c.role, UserID: c.userID}
}

func (c *Client) Logout() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ws != nil {
		_ = c.ws.Close()
		c.ws = nil
	}
	c.pending = nil
	c.baseURL = ""
	c.token = ""
	c.username = ""
	c.role = ""
	c.userID = 0
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Aud      string `json:"aud"`
}

type loginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"user"`
}

func (c *Client) Login(ctx context.Context, baseURL, username, password string) (Session, error) {
	baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if username == "" || password == "" {
		return Session{}, errors.New("usuário e senha são obrigatórios")
	}
	var resp loginResponse
	req := loginRequest{Username: username, Password: password, Aud: "xchat"}
	if err := doJSON(ctx, c.httpClient, http.MethodPost, baseURL, "/api/auth/login", "", req, &resp); err != nil {
		return Session{}, err
	}
	if resp.Token == "" {
		return Session{}, errors.New("servidor não devolveu um token de sessão")
	}
	c.mu.Lock()
	c.baseURL = baseURL
	c.token = resp.Token
	c.username = resp.User.Username
	c.role = resp.User.Role
	c.userID = resp.User.ID
	c.mu.Unlock()
	return Session{LoggedIn: true, Username: resp.User.Username, Role: resp.User.Role, UserID: resp.User.ID}, nil
}

type ProfilePage struct {
	Items   []Profile `json:"items"`
	Total   int64     `json:"total"`
	Page    int       `json:"page"`
	PerPage int       `json:"per_page"`
}

type GroupPage struct {
	Items   []Group `json:"items"`
	Total   int64   `json:"total"`
	Page    int     `json:"page"`
	PerPage int     `json:"per_page"`
}

type ThreadPage struct {
	Items   []Thread `json:"items"`
	Total   int64    `json:"total"`
	Page    int      `json:"page"`
	PerPage int      `json:"per_page"`
}

type MessagePage struct {
	Items   []Message `json:"items"`
	Total   int64     `json:"total"`
	Page    int       `json:"page"`
	PerPage int       `json:"per_page"`
}

type Profile struct {
	UserID      uint   `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatar_url"`
	Following   bool   `json:"following"`
	Followers   int64  `json:"followers"`
}

type Group struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerUserID uint   `json:"owner_user_id"`
	MemberCount int64  `json:"member_count"`
}

type Thread struct {
	ID         uint       `json:"id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	PeerUserID uint       `json:"peer_user_id,omitempty"`
	LastBody   string     `json:"last_body,omitempty"`
	LastAt     *time.Time `json:"last_at,omitempty"`
}

type Message struct {
	ID           uint      `json:"id"`
	ThreadKind   string    `json:"thread_kind"`
	ThreadID     uint      `json:"thread_id"`
	AuthorID     uint      `json:"author_id"`
	Kind         string    `json:"kind"`
	Body         string    `json:"body"`
	AttachmentID *uint     `json:"attachment_id,omitempty"`
	Filename     string    `json:"filename,omitempty"`
	Mime         string    `json:"mime,omitempty"`
	SizeBytes    int64     `json:"size_bytes,omitempty"`
	Delivered    bool      `json:"delivered"`
	Read         bool      `json:"read"`
	CreatedAt    time.Time `json:"created_at"`
}

type Attachment struct {
	ID        uint   `json:"id"`
	Filename  string `json:"filename"`
	Mime      string `json:"mime"`
	SizeBytes int64  `json:"size_bytes"`
	Kind      string `json:"kind"`
}

type StoryItem struct {
	ID           uint      `json:"id"`
	AuthorID     uint      `json:"author_id"`
	Username     string    `json:"username"`
	Kind         string    `json:"kind"`
	Body         string    `json:"body"`
	AttachmentID *uint     `json:"attachment_id,omitempty"`
	Mime         string    `json:"mime,omitempty"`
	Viewed       bool      `json:"viewed"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type StoryAuthor struct {
	AuthorID uint        `json:"author_id"`
	Username string      `json:"username"`
	Unseen   bool        `json:"unseen"`
	Items    []StoryItem `json:"items"`
}

func (c *Client) creds() (base, token string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == "" {
		return "", "", ErrNotLoggedIn
	}
	return c.baseURL, c.token, nil
}

func (c *Client) ListPeople(ctx context.Context, page int, q string) (ProfilePage, error) {
	return getPage[ProfilePage](ctx, c, "/api/social/people", page, q)
}

func (c *Client) ListThreads(ctx context.Context, page int) (ThreadPage, error) {
	return getPage[ThreadPage](ctx, c, "/api/social/threads", page, "")
}

func (c *Client) ListGroups(ctx context.Context, page int) (GroupPage, error) {
	return getPage[GroupPage](ctx, c, "/api/social/groups", page, "")
}

func (c *Client) OpenThread(ctx context.Context, username string) (Thread, error) {
	var th Thread
	err := c.doAuthed(ctx, http.MethodPost, "/api/social/threads", map[string]string{"username": username}, &th)
	return th, err
}

func (c *Client) ListMessages(ctx context.Context, kind string, id uint, page int) (MessagePage, error) {
	path := fmt.Sprintf("/api/social/threads/%s/%d/messages", kind, id)
	return getPage[MessagePage](ctx, c, path, page, "")
}

func (c *Client) PostMessage(ctx context.Context, kind string, id uint, body string) (Message, error) {
	return c.PostMediaMessage(ctx, kind, id, body, "text", 0)
}

func (c *Client) PostMediaMessage(ctx context.Context, kind string, id uint, body, msgKind string, attachmentID uint) (Message, error) {
	var msg Message
	path := fmt.Sprintf("/api/social/threads/%s/%d/messages", kind, id)
	payload := map[string]any{"body": body, "kind": msgKind}
	if attachmentID > 0 {
		payload["attachment_id"] = attachmentID
	}
	err := c.doAuthed(ctx, http.MethodPost, path, payload, &msg)
	return msg, err
}

func (c *Client) CreateGroup(ctx context.Context, name, description string) (Group, error) {
	var g Group
	err := c.doAuthed(ctx, http.MethodPost, "/api/social/groups", map[string]string{"name": name, "description": description}, &g)
	return g, err
}

func (c *Client) InviteToGroup(ctx context.Context, groupID uint, username string) error {
	path := fmt.Sprintf("/api/social/groups/%d/invite", groupID)
	return c.doAuthed(ctx, http.MethodPost, path, map[string]string{"username": username}, nil)
}

func (c *Client) AckMessages(ctx context.Context, ids []uint, state string) error {
	if len(ids) == 0 {
		return nil
	}
	return c.doAuthed(ctx, http.MethodPost, "/api/social/acks", map[string]any{"message_ids": ids, "state": state}, nil)
}

func (c *Client) UploadAttachment(ctx context.Context, filename, mime string, data []byte) (Attachment, error) {
	base, token, err := c.creds()
	if err != nil {
		return Attachment{}, err
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	if mime != "" {
		hdr.Set("Content-Type", mime)
	}
	part, err := w.CreatePart(hdr)
	if err != nil {
		return Attachment{}, err
	}
	if _, err := part.Write(data); err != nil {
		return Attachment{}, err
	}
	if err := w.Close(); err != nil {
		return Attachment{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(base, "/")+"/api/social/attachments", &buf)
	if err != nil {
		return Attachment{}, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Attachment{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Error != "" {
			return Attachment{}, errors.New(e.Error)
		}
		return Attachment{}, fmt.Errorf("erro HTTP %d", resp.StatusCode)
	}
	var att Attachment
	if err := json.Unmarshal(raw, &att); err != nil {
		return Attachment{}, err
	}
	return att, nil
}

func (c *Client) DownloadAttachment(ctx context.Context, id uint) ([]byte, string, error) {
	base, token, err := c.creds()
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/social/attachments/%d", strings.TrimSuffix(base, "/"), id), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("erro HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 33<<20))
	return raw, resp.Header.Get("Content-Type"), err
}

func (c *Client) ListStories(ctx context.Context) ([]StoryAuthor, error) {
	var out struct {
		Items []StoryAuthor `json:"items"`
	}
	err := c.doAuthed(ctx, http.MethodGet, "/api/social/stories", nil, &out)
	return out.Items, err
}

func (c *Client) CreateStory(ctx context.Context, body, kind string, attachmentID uint) (StoryItem, error) {
	var st StoryItem
	payload := map[string]any{"body": body, "kind": kind}
	if attachmentID > 0 {
		payload["attachment_id"] = attachmentID
	}
	err := c.doAuthed(ctx, http.MethodPost, "/api/social/stories", payload, &st)
	return st, err
}

func (c *Client) ViewStory(ctx context.Context, id uint) error {
	return c.doAuthed(ctx, http.MethodPost, fmt.Sprintf("/api/social/stories/%d/view", id), map[string]any{}, nil)
}

func (c *Client) SendSignal(evType string, payload any) error {
	return c.writeWS(map[string]any{"type": evType, "payload": payload})
}

type WSEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ListenWS autentica no primeiro frame e entrega eventos até o ctx cancelar.
func (c *Client) ListenWS(ctx context.Context, onEvent func(WSEvent)) error {
	base, token, err := c.creds()
	if err != nil {
		return err
	}
	u, err := url.Parse(base)
	if err != nil {
		return err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = "/api/ws"
	u.RawQuery = ""
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return err
	}
	defer func() {
		c.mu.Lock()
		if c.ws == conn {
			c.ws = nil
		}
		c.mu.Unlock()
		_ = conn.Close()
	}()

	if err := conn.WriteJSON(map[string]string{"type": "auth", "token": token}); err != nil {
		return err
	}
	c.mu.Lock()
	c.ws = conn
	queued := c.pending
	c.pending = nil
	c.mu.Unlock()
	for _, payload := range queued {
		if err := conn.WriteJSON(payload); err != nil {
			return err
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var ev WSEvent
		if json.Unmarshal(raw, &ev) != nil {
			continue
		}
		onEvent(ev)
	}
}

func (c *Client) SendTyping(kind string, threadID uint) error {
	return c.writeWS(map[string]any{
		"type":    "typing",
		"payload": map[string]any{"thread_kind": kind, "thread_id": threadID},
	})
}

func (c *Client) SetPresence(status string) error {
	return c.writeWS(map[string]any{
		"type":    "presence",
		"payload": map[string]any{"status": status},
	})
}

func (c *Client) writeWS(payload any) error {
	c.mu.Lock()
	conn := c.ws
	if conn == nil {
		c.pending = append(c.pending, payload)
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	return conn.WriteJSON(payload)
}

func getPage[P any](ctx context.Context, c *Client, path string, page int, q string) (P, error) {
	if page < 1 {
		page = 1
	}
	vals := url.Values{}
	vals.Set("page", strconv.Itoa(page))
	vals.Set("per_page", "25")
	if q != "" {
		vals.Set("q", q)
	}
	var out P
	err := c.doAuthed(ctx, http.MethodGet, path+"?"+vals.Encode(), nil, &out)
	return out, err
}

func (c *Client) doAuthed(ctx context.Context, method, path string, body any, dest any) error {
	base, token, err := c.creds()
	if err != nil {
		return err
	}
	return doJSON(ctx, c.httpClient, method, base, path, token, body, dest)
}

func doJSON(ctx context.Context, httpClient *http.Client, method, baseURL, path, token string, body any, dest any) error {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(baseURL, "/")+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrNotLoggedIn
	}
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("erro HTTP %d", resp.StatusCode)
	}
	if dest == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
