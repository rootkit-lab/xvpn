package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type wsAuthFrame struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// handleSocialWS faz upgrade. Token na query é rejeitado (cai no access log).
// Auth no primeiro frame: {"type":"auth","token":"..."}.
func (a *App) handleSocialWS(c *gin.Context) {
	if c.Query("token") != "" || c.Query("access_token") != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "não envie o token na query string"})
		return
	}
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var frame wsAuthFrame
	if err := json.Unmarshal(raw, &frame); err != nil || frame.Type != "auth" || frame.Token == "" {
		_ = conn.WriteJSON(gin.H{"type": "error", "payload": "auth inválido"})
		return
	}
	claims, err := a.Tokens.Parse(frame.Token)
	if err != nil {
		_ = conn.WriteJSON(gin.H{"type": "error", "payload": "token inválido"})
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	if a.Hub == nil {
		a.Hub = newHub()
	}
	if a.Hub.connectionCount(claims.UserID) >= maxWSConnsPerUser {
		_ = conn.WriteJSON(gin.H{"type": "error", "payload": "muitas conexões"})
		return
	}
	ch := a.Hub.subscribe(claims.UserID)
	defer a.Hub.unsubscribe(claims.UserID, ch)

	selfStatus := a.Hub.statusOf(claims.UserID)
	_ = conn.WriteJSON(wsEvent{Type: "presence", Payload: gin.H{"user_id": claims.UserID, "status": selfStatus}})
	_ = conn.WriteJSON(wsEvent{Type: "presence.snapshot", Payload: a.Hub.presenceSnapshot()})
	a.Hub.broadcastPresence(claims.UserID, selfStatus)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var incoming wsEvent
			if json.Unmarshal(msg, &incoming) != nil {
				continue
			}
			switch incoming.Type {
			case "typing":
				a.relayTyping(claims.UserID, incoming.Payload)
			case "presence":
				a.applyPresence(claims.UserID, incoming.Payload)
			case "message.ack":
				// cliente confirma recebimento — sem persistência extra
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case payload, ok := <-ch:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}
}

func (a *App) applyPresence(from uint, payload any) {
	raw, _ := json.Marshal(payload)
	var body struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(raw, &body) != nil {
		return
	}
	a.Hub.setStatus(from, body.Status)
}

func (a *App) relayTyping(from uint, payload any) {
	raw, _ := json.Marshal(payload)
	var body struct {
		ThreadKind string `json:"thread_kind"`
		ThreadID   uint   `json:"thread_id"`
	}
	if json.Unmarshal(raw, &body) != nil || body.ThreadID == 0 {
		return
	}
	ids := a.threadMemberIDs(body.ThreadKind, body.ThreadID)
	a.Hub.sendToMany(ids, wsEvent{Type: "typing", Payload: gin.H{"user_id": from, "thread_kind": body.ThreadKind, "thread_id": body.ThreadID}})
}

func (a *App) threadMemberIDs(kind string, threadID uint) []uint {
	if kind == "group" {
		var members []store.SocialGroupMember
		_ = a.Store.DB.Where("group_id = ?", threadID).Find(&members).Error
		ids := make([]uint, 0, len(members))
		for _, m := range members {
			ids = append(ids, m.UserID)
		}
		return ids
	}
	var members []store.DirectThreadMember
	_ = a.Store.DB.Where("thread_id = ?", threadID).Find(&members).Error
	ids := make([]uint, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.UserID)
	}
	return ids
}

func callerUsername(c *gin.Context) string {
	v, _ := c.Get(auth.ContextUsernameKey)
	s, _ := v.(string)
	return s
}
