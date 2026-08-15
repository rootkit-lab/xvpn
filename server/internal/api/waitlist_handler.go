package api

import (
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const (
	waitlistStatusPending  = "pending"
	waitlistStatusApproved = "approved"
	waitlistStatusRejected = "rejected"

	// waitlistMessageMaxLen evita que o único formulário público e sem
	// autenticação da API seja usado para gravar blobs arbitrariamente
	// grandes no banco.
	waitlistMessageMaxLen = 2000
)

type waitlistResponse struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	Message    string     `json:"message"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
}

func toWaitlistResponse(e store.WaitlistEntry) waitlistResponse {
	return waitlistResponse{
		ID:         e.ID,
		Name:       e.Name,
		Email:      e.Email,
		Message:    e.Message,
		Status:     e.Status,
		CreatedAt:  e.CreatedAt,
		ReviewedAt: e.ReviewedAt,
	}
}

type joinWaitlistRequest struct {
	Name    string `json:"name" binding:"required"`
	Email   string `json:"email" binding:"required"`
	Message string `json:"message"`
}

// handleJoinWaitlist recebe um cadastro de interesse da landing pública.
// Único endpoint de escrita da API sem autenticação — protegido por rate
// limit por IP (ver server.go) e validação estrita, já que qualquer
// visitante da internet pode chamá-lo diretamente.
// POST /api/waitlist
func (a *App) handleJoinWaitlist(c *gin.Context) {
	var req joinWaitlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nome e e-mail são obrigatórios"})
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	message := strings.TrimSpace(req.Message)

	if name == "" || len(name) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nome inválido"})
		return
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "e-mail inválido"})
		return
	}
	email = addr.Address
	if len(message) > waitlistMessageMaxLen {
		message = message[:waitlistMessageMaxLen]
	}

	var existing store.WaitlistEntry
	err = a.Store.DB.Where("email = ?", email).First(&existing).Error
	if err == nil {
		// Cadastro duplicado não é erro pro visitante (evita confirmar/
		// negar por enumeração se um e-mail já está na lista) — só
		// devolve a entrada existente sem criar duplicata.
		c.JSON(http.StatusOK, toWaitlistResponse(existing))
		return
	}
	if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	entry := store.WaitlistEntry{
		Name:    name,
		Email:   email,
		Message: message,
		Status:  waitlistStatusPending,
	}
	if err := a.Store.DB.Create(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	_ = a.Store.LogAudit("público", "waitlist.join", "email="+entry.Email)

	c.JSON(http.StatusCreated, toWaitlistResponse(entry))
}

// handleListWaitlist lista cadastros (pendentes e já avaliados).
// GET /api/waitlist?page=&per_page=&q=&status=
func (a *App) handleListWaitlist(c *gin.Context) {
	p := parsePage(c)
	q := a.Store.DB.Model(&store.WaitlistEntry{})
	if p.Q != "" {
		like := p.like()
		q = q.Where("name LIKE ? OR email LIKE ?", like, like)
	}
	if status := c.Query("status"); status == waitlistStatusPending || status == waitlistStatusApproved || status == waitlistStatusRejected {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var entries []store.WaitlistEntry
	if err := p.apply(q.Order("created_at desc")).Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	resp := make([]waitlistResponse, 0, len(entries))
	for _, e := range entries {
		resp = append(resp, toWaitlistResponse(e))
	}
	writePage(c, resp, total, p)
}

// handleApproveWaitlist e handleRejectWaitlist só atualizam o status
// exibido no painel — não criam usuário/convite sozinhos. Quem quer
// provisionar de fato usa handleProvisionWaitlist (ou a tela Usuários já
// existente, manualmente).
// POST /api/waitlist/:id/approve
func (a *App) handleApproveWaitlist(c *gin.Context) {
	a.reviewWaitlist(c, waitlistStatusApproved)
}

// POST /api/waitlist/:id/reject
func (a *App) handleRejectWaitlist(c *gin.Context) {
	a.reviewWaitlist(c, waitlistStatusRejected)
}

type provisionWaitlistRequest struct {
	Username string     `json:"username" binding:"required"`
	Role     store.Role `json:"role"`
}

type provisionWaitlistResponse struct {
	User userResponse `json:"user"`
	// Password é gerada pelo servidor e devolvida uma única vez nesta
	// resposta — a tela de waitlist não coleta senha do admin (ver
	// PLAN.md §6.7, "não inventa segundo caminho de credencial").
	Password string         `json:"password"`
	Invite   inviteResponse `json:"invite"`
}

// handleProvisionWaitlist orquestra "aprovar e provisionar" (Fase 10, ver
// ROADMAP.md): cria um User de verdade + um InviteToken para o cadastro da
// waitlist, na mesma transação, e marca o cadastro como aprovado. É
// exatamente o mesmo par POST /users + POST /users/:id/invite que o admin
// já faria manualmente pela tela Usuários — só que orquestrado num clique,
// sem inventar um segundo caminho de credencial.
// POST /api/waitlist/:id/provision
func (a *App) handleProvisionWaitlist(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var req provisionWaitlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username é obrigatório"})
		return
	}

	role := req.Role
	if role == "" {
		role = store.RoleMember
	}
	if !role.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role inválido"})
		return
	}
	if !callerRole(c).CanManage(role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "seu papel não pode provisionar um usuário com esse papel"})
		return
	}

	var entry store.WaitlistEntry
	if err := a.Store.DB.First(&entry, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cadastro não encontrado"})
		return
	}

	password, err := auth.GenerateRandomPassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	user := store.User{Username: req.Username, PasswordHash: hash, Role: role}
	var invite store.InviteToken
	now := time.Now()

	err = a.Store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		token, err := generateInviteToken()
		if err != nil {
			return err
		}
		invite = store.InviteToken{
			UserID:    user.ID,
			Token:     token,
			ExpiresAt: now.Add(time.Duration(a.Config.InviteTokenTTLMinutes) * time.Minute),
		}
		if err := tx.Create(&invite).Error; err != nil {
			return err
		}
		entry.Status = waitlistStatusApproved
		entry.ReviewedAt = &now
		return tx.Save(&entry).Error
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "não foi possível provisionar (username já em uso?)"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "waitlist.provisioned",
		"waitlist_id="+c.Param("id")+" user_id="+strconv.FormatUint(uint64(user.ID), 10)+" username="+user.Username+" role="+string(user.Role))

	c.JSON(http.StatusCreated, provisionWaitlistResponse{
		User:     toUserResponse(user),
		Password: password,
		Invite:   inviteResponse{Token: invite.Token, ExpiresAt: invite.ExpiresAt},
	})
}

func (a *App) reviewWaitlist(c *gin.Context, status string) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var entry store.WaitlistEntry
	if err := a.Store.DB.First(&entry, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cadastro não encontrado"})
		return
	}

	now := time.Now()
	entry.Status = status
	entry.ReviewedAt = &now
	if err := a.Store.DB.Save(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "waitlist."+status, "id="+c.Param("id")+" email="+entry.Email)

	c.JSON(http.StatusOK, toWaitlistResponse(entry))
}
