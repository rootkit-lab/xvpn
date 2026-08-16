package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type ackRequest struct {
	MessageIDs []uint `json:"message_ids"`
	State      string `json:"state"`
}

func (a *App) handleSocialAck(c *gin.Context) {
	var req ackRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.MessageIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message_ids é obrigatório"})
		return
	}
	a.applyMessageAck(callerUserID(c), req.MessageIDs, req.State)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) applyMessageAck(from uint, ids []uint, state string) {
	if state != "delivered" && state != "read" {
		return
	}
	now := time.Now()
	for _, id := range ids {
		if id == 0 {
			continue
		}
		var msg store.Message
		if err := a.Store.DB.First(&msg, id).Error; err != nil {
			continue
		}
		if msg.AuthorID == from || !a.canAccessThread(msg.ThreadKind, msg.ThreadID, from) {
			continue
		}
		var rec store.MessageReceipt
		err := a.Store.DB.Where("message_id = ? AND user_id = ?", id, from).First(&rec).Error
		if err == gorm.ErrRecordNotFound {
			rec = store.MessageReceipt{MessageID: id, UserID: from, DeliveredAt: &now}
			if state == "read" {
				rec.ReadAt = &now
			}
			if a.Store.DB.Create(&rec).Error != nil {
				continue
			}
		} else if err != nil {
			continue
		} else {
			updates := map[string]any{}
			if rec.DeliveredAt == nil {
				updates["delivered_at"] = now
			}
			if state == "read" && rec.ReadAt == nil {
				updates["read_at"] = now
			}
			if len(updates) == 0 {
				continue
			}
			if a.Store.DB.Model(&rec).Updates(updates).Error != nil {
				continue
			}
		}
		resp := a.messageResponse(msg)
		batch := []socialMessageResponse{resp}
		a.attachReceipts(batch)
		if a.Hub != nil {
			a.Hub.sendToMany(a.threadMemberIDs(msg.ThreadKind, msg.ThreadID), wsEvent{Type: "message.receipt", Payload: batch[0]})
		}
	}
}

func (a *App) attachReceipts(items []socialMessageResponse) {
	if len(items) == 0 {
		return
	}
	ids := make([]uint, 0, len(items))
	byID := map[uint]*socialMessageResponse{}
	for i := range items {
		ids = append(ids, items[i].ID)
		byID[items[i].ID] = &items[i]
	}
	var recs []store.MessageReceipt
	_ = a.Store.DB.Where("message_id IN ?", ids).Find(&recs).Error
	type tally struct{ others, delivered, read int }
	t := map[uint]*tally{}
	for _, it := range items {
		members := a.threadMemberIDs(it.ThreadKind, it.ThreadID)
		others := 0
		for _, uid := range members {
			if uid != it.AuthorID {
				others++
			}
		}
		t[it.ID] = &tally{others: others}
	}
	for _, r := range recs {
		row := t[r.MessageID]
		if row == nil {
			continue
		}
		if r.DeliveredAt != nil {
			row.delivered++
		}
		if r.ReadAt != nil {
			row.read++
		}
	}
	for id, row := range t {
		it := byID[id]
		if it == nil || row.others == 0 {
			continue
		}
		it.Delivered = row.delivered >= row.others
		it.Read = row.read >= row.others
	}
}
