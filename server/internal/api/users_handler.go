package api

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
)

type userResponse struct {
	ID        uint       `json:"id"`
	Username  string     `json:"username"`
	Role      store.Role `json:"role"`
	CreatedAt time.Time  `json:"created_at"`
	// Products (Fase 33): escopo de admin. Sempre presente — lista vazia
	// significa admin irrestrito (ou N/A para viewer/member).
	Products []store.Product `json:"products"`
	// Acesso a arquivos (Fase 13, PLAN.md §6.9): sempre presentes na
	// resposta (default false/"" quando o usuário nunca teve acesso).
	SFTPEnabled  bool   `json:"sftp_enabled"`
	SambaEnabled bool   `json:"samba_enabled"`
	SSHPublicKey string `json:"ssh_public_key"`
	DiskQuotaMB  uint64 `json:"disk_quota_mb"`
	// XgitEnabled: waffle "Seus apps" — ProjectMember ou ACL do app xgit.
	XgitEnabled bool `json:"xgit_enabled"`
	// XcodespacesEnabled: waffle — ProjectMember ou ACL do app xcodespaces.
	XcodespacesEnabled bool `json:"xcodespaces_enabled"`
}

func toUserResponse(u store.User) userResponse {
	products := u.Products
	if products == nil {
		products = []store.Product{}
	}
	return userResponse{
		ID:           u.ID,
		Username:     u.Username,
		Role:         u.Role,
		CreatedAt:    u.CreatedAt,
		Products:     products,
		SFTPEnabled:  u.SFTPEnabled,
		SambaEnabled: u.SambaEnabled,
		SSHPublicKey: u.SSHPublicKey,
		DiskQuotaMB:  u.DiskQuotaMB,
	}
}

func (a *App) userHasXgit(user store.User) bool {
	var n int64
	_ = a.Store.DB.Model(&store.ProjectMember{}).Where("user_id = ?", user.ID).Count(&n).Error
	if n > 0 {
		return true
	}
	_ = a.Store.DB.Model(&store.OrgMember{}).Where("user_id = ?", user.ID).Count(&n).Error
	if n > 0 {
		return true
	}
	_ = a.Store.DB.Model(&store.OrgTeamMember{}).Where("user_id = ?", user.ID).Count(&n).Error
	if n > 0 {
		return true
	}
	var app store.App
	if err := a.Store.DB.Where("slug = ? AND archived_at IS NULL", "xgit").First(&app).Error; err != nil {
		return false
	}
	if app.Visibility == store.AppVisibilityGlobal {
		return true
	}
	var access int64
	_ = a.Store.DB.Model(&store.AppAccess{}).Where("app_id = ? AND user_id = ?", app.ID, user.ID).Count(&access).Error
	return access > 0
}

func (a *App) userHasAppACL(user store.User, slug string) bool {
	var app store.App
	if err := a.Store.DB.Where("slug = ? AND archived_at IS NULL", slug).First(&app).Error; err != nil {
		return false
	}
	if app.Visibility == store.AppVisibilityGlobal {
		return true
	}
	var access int64
	_ = a.Store.DB.Model(&store.AppAccess{}).Where("app_id = ? AND user_id = ?", app.ID, user.ID).Count(&access).Error
	return access > 0
}

func (a *App) toSessionUser(user store.User) userResponse {
	resp := toUserResponse(user)
	resp.XgitEnabled = a.userHasXgit(user)
	resp.XcodespacesEnabled = a.userHasXgit(user) || a.userHasAppACL(user, "xcodespaces")
	return resp
}

func callerProducts(c *gin.Context) []store.Product {
	return auth.ProductsFromContext(c)
}

func writeProductAssignError(c *gin.Context, err error) {
	if errors.Is(err, store.ErrInvalidProduct) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "produto inválido (use core, marketplace, xgroup ou xdriver)"})
		return
	}
	c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
}

// callerRole/callerUserID leem a identidade definida por auth.RequireAuth no
// gin.Context — pequenos helpers para não repetir a asserção de tipo em
// cada handler que precisa checar permissão (ver store.Role.CanManage).
func callerRole(c *gin.Context) store.Role {
	role, _ := auth.RoleFromContext(c)
	return role
}

func callerUserID(c *gin.Context) uint {
	v, _ := c.Get(auth.ContextUserIDKey)
	id, _ := v.(uint)
	return id
}

// countSuperAdmins conta quantos usuários têm hoje o papel super_admin —
// usado para nunca deixar o sistema sem nenhum (ver ROADMAP.md Fase 10).
func (a *App) countSuperAdmins() (int64, error) {
	var count int64
	err := a.Store.DB.Model(&store.User{}).Where("role = ?", store.RoleSuperAdmin).Count(&count).Error
	return count, err
}

// handleListUsers lista os usuários cadastrados (sem hash de senha).
// GET /api/users?page=&per_page=&q=&role=
func (a *App) handleListUsers(c *gin.Context) {
	p := parsePage(c)
	q := a.Store.DB.Model(&store.User{})
	if p.Q != "" {
		q = q.Where("username LIKE ?", p.like())
	}
	if role := store.Role(c.Query("role")); role.Valid() {
		q = q.Where("role = ?", role)
	}
	if v := c.Query("sftp"); v == "1" || v == "true" {
		q = q.Where("sftp_enabled = ?", true)
	} else if v == "0" || v == "false" {
		q = q.Where("sftp_enabled = ?", false)
	}
	if v := c.Query("samba"); v == "1" || v == "true" {
		q = q.Where("samba_enabled = ?", true)
	} else if v == "0" || v == "false" {
		q = q.Where("samba_enabled = ?", false)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var users []store.User
	if err := p.apply(q.Order("id")).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}
	writePage(c, resp, total, p)
}

// handleGetUser devolve um usuário (sem hash). Leitura — viewer+.
// GET /api/users/:id
func (a *App) handleGetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var user store.User
	if err := a.Store.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

type createUserRequest struct {
	Username string          `json:"username" binding:"required"`
	Password string          `json:"password" binding:"required,min=8"`
	Role     store.Role      `json:"role"`
	Products []store.Product `json:"products"`
}

// handleCreateUser cria um novo usuário do painel. O papel (Fase 10, ver
// PLAN.md §6.7) é opcional no corpo — o default é o mais restritivo
// (member) — mas quem cria nunca pode conceder um papel acima do próprio
// (store.Role.CanManage), ou seja, só super_admin cria outro super_admin.
// POST /api/users
func (a *App) handleCreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username e password (mín. 8 caracteres) são obrigatórios"})
		return
	}

	role := req.Role
	if role == "" {
		role = store.RoleMember
	}
	if !role.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role inválido (use super_admin, admin, viewer ou member)"})
		return
	}
	if !callerRole(c).CanManage(role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "seu papel não pode criar um usuário com esse papel"})
		return
	}

	products, err := store.ResolveAssignedProducts(callerRole(c), callerProducts(c), req.Products, role)
	if err != nil {
		writeProductAssignError(c, err)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	user := store.User{Username: req.Username, PasswordHash: hash, Role: role, Products: products}
	if err := a.Store.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "usuário já existe ou dado inválido"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "user.create", "username="+user.Username+" role="+string(user.Role))

	c.JSON(http.StatusCreated, toUserResponse(user))
}

type updateUserRequest struct {
	Username *string          `json:"username"`
	Role     *store.Role      `json:"role"`
	Products *[]store.Product `json:"products"`
}

// handleUpdateUser edita o username e/ou o papel de um usuário existente —
// nunca a senha (ver handleResetPassword). Regras de autorização (Fase 10,
// PLAN.md §6.7):
//   - o papel atual do alvo precisa estar no nível do chamador ou abaixo
//     (store.Role.CanManage) — um admin não mexe numa conta super_admin;
//   - o papel NOVO pedido também passa pela mesma checagem — um admin não
//     promove ninguém a super_admin;
//   - ninguém troca o próprio papel por aqui (evita auto-promoção/
//     rebaixamento acidental — use outra conta com o papel adequado);
//   - rebaixar o único super_admin restante é bloqueado, pelo mesmo motivo
//     do guard em handleDeleteUser.
//
// PATCH /api/users/:id
func (a *App) handleUpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.Username == nil && req.Role == nil && req.Products == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "informe username, role e/ou products para atualizar"})
		return
	}

	var target store.User
	if err := a.Store.DB.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}

	role := callerRole(c)
	if !role.CanManage(target.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "seu papel não pode editar este usuário"})
		return
	}
	if !store.CoversAccount(role, callerProducts(c), target.Role, target.Products) {
		c.JSON(http.StatusForbidden, gin.H{"error": "seu escopo não pode editar este usuário"})
		return
	}

	updates := map[string]any{}
	if req.Username != nil {
		username := strings.TrimSpace(*req.Username)
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username não pode ficar vazio"})
			return
		}
		// Rename de username é proibido enquanto o usuário tem SFTP ou
		// Samba habilitado: o provisionamento Unix usa o username como
		// chave (drop-in sshd `Match User <name>`, share Samba
		// `[home-<name>]`, /home/<name>/). Renomear no DB sem
		// reprovisionar deixaria configs órfãos no VPS sob o nome antigo
		// e o reconcile não os limparia (só conhece o nome novo). Em
		// vez de tentar reprovisionar atomicamente (complexo e propenso a
		// deixar metade pronta), exigimos que o admin desligue o acesso
		// a arquivos antes de renomear — fluxo explícito e seguro
		// (Bugbot: "Rename orphans Unix account configs").
		if username != target.Username && (target.SFTPEnabled || target.SambaEnabled) {
			c.JSON(http.StatusConflict, gin.H{"error": "desative o acesso a arquivos (SFTP/Samba) antes de renomear o usuário"})
			return
		}
		updates["username"] = username
	}
	if req.Role != nil {
		newRole := *req.Role
		if !newRole.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "role inválido"})
			return
		}
		if callerUserID(c) == target.ID {
			c.JSON(http.StatusForbidden, gin.H{"error": "você não pode alterar o próprio papel"})
			return
		}
		if !role.CanManage(newRole) {
			c.JSON(http.StatusForbidden, gin.H{"error": "seu papel não pode conceder esse papel"})
			return
		}
		if target.Role == store.RoleSuperAdmin && newRole != store.RoleSuperAdmin {
			// Defesa em profundidade: dado que ninguém troca o próprio
			// papel (guarda acima) e CanManage exige rank >= o do alvo,
			// na prática só outro super_admin chega aqui — e se ele
			// existe, a contagem é >= 2, então este branch nunca deveria
			// disparar de verdade. Mantido caso a guarda de
			// auto-modificação mude no futuro (ver
			// TestHandleUpdateUser_CannotDemoteLastSuperAdmin).
			superAdmins, err := a.countSuperAdmins()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
				return
			}
			if superAdmins <= 1 {
				c.JSON(http.StatusConflict, gin.H{"error": "não é possível rebaixar o único super_admin"})
				return
			}
		}
		updates["role"] = newRole
	}

	if req.Products != nil || req.Role != nil {
		if callerUserID(c) == target.ID && req.Products != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "você não pode alterar o próprio escopo de produtos"})
			return
		}
		targetRole := target.Role
		if req.Role != nil {
			targetRole = *req.Role
		}
		var requested []store.Product
		if req.Products != nil {
			requested = *req.Products
		} else if targetRole == store.RoleAdmin {
			requested = target.Products
		}
		products, err := store.ResolveAssignedProducts(role, callerProducts(c), requested, targetRole)
		if err != nil {
			writeProductAssignError(c, err)
			return
		}
		if products == nil {
			products = []store.Product{}
		}
		// Updates via map bypassa serializer:json — gravamos o JSON
		// explícito na coluna texto.
		raw, err := json.Marshal(products)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		updates["products"] = string(raw)
	}

	if err := a.Store.DB.Model(&store.User{}).Where("id = ?", target.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "não foi possível atualizar (username já em uso?)"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	detail := "user_id=" + c.Param("id")
	if req.Username != nil {
		detail += " new_username=" + *req.Username
	}
	if req.Role != nil {
		detail += " new_role=" + string(*req.Role)
		_ = a.Store.LogAudit(actorString(actor), "user.role_changed", detail)
	} else if req.Products != nil {
		_ = a.Store.LogAudit(actorString(actor), "user.products_changed", detail)
	} else {
		_ = a.Store.LogAudit(actorString(actor), "user.update", detail)
	}

	if err := a.Store.DB.First(&target, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, toUserResponse(target))
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

type resetPasswordResponse struct {
	// Password só vem preenchida quando o admin não informou uma senha no
	// corpo — o servidor gera uma e a devolve nesta única resposta, nunca
	// mais (mesmo padrão do bootstrap em cmd/xvpn-server/main.go).
	Password string `json:"password,omitempty"`
}

// handleResetPassword troca a senha de outro usuário, definida pelo admin —
// não é o fluxo de "esqueci minha senha" do próprio usuário (que exigiria
// e-mail/verificação e não existe no MVP). Mesma checagem de hierarquia de
// handleUpdateUser: só quem gerencia o papel do alvo pode resetar a senha.
// POST /api/users/:id/reset-password
func (a *App) handleResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var req resetPasswordRequest
	// Corpo é opcional (senha gerada automaticamente se omitido) — só
	// rejeita JSON malformado, não a ausência de corpo.
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
			return
		}
	}

	var target store.User
	if err := a.Store.DB.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}

	if !callerRole(c).CanManage(target.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "seu papel não pode redefinir a senha deste usuário"})
		return
	}
	if !store.CoversAccount(callerRole(c), callerProducts(c), target.Role, target.Products) {
		c.JSON(http.StatusForbidden, gin.H{"error": "seu escopo não pode redefinir a senha deste usuário"})
		return
	}

	password := req.Password
	generated := false
	if password == "" {
		password, err = auth.GenerateRandomPassword()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		generated = true
	} else if len(password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "senha deve ter ao menos 8 caracteres"})
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.Store.DB.Model(&store.User{}).Where("id = ?", target.ID).Update("password_hash", hash).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "user.password_reset", "user_id="+c.Param("id"))

	resp := resetPasswordResponse{}
	if generated {
		resp.Password = password
	}
	c.JSON(http.StatusOK, resp)
}

// handleDeleteUser remove um usuário e revoga todos os dispositivos dele
// (removendo os peers correspondentes da interface WireGuard). Bloqueia
// duas coisas (Fase 10, PLAN.md §6.7): remover um usuário cujo papel esteja
// acima do que o chamador gerencia (store.Role.CanManage), e remover o
// único super_admin restante — "único super_admin não pode se auto-apagar"
// generalizado para "ninguém apaga o único super_admin", mais simples e
// estrito.
// DELETE /api/users/:id
func (a *App) handleDeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var target store.User
	if err := a.Store.DB.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}

	if !callerRole(c).CanManage(target.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "seu papel não pode remover este usuário"})
		return
	}
	if !store.CoversAccount(callerRole(c), callerProducts(c), target.Role, target.Products) {
		c.JSON(http.StatusForbidden, gin.H{"error": "seu escopo não pode remover este usuário"})
		return
	}

	if target.Role == store.RoleSuperAdmin {
		superAdmins, err := a.countSuperAdmins()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		if superAdmins <= 1 {
			c.JSON(http.StatusConflict, gin.H{"error": "não é possível remover o único super_admin"})
			return
		}
	}

	// Revoga o acesso a arquivos Unix (SFTP/Samba) ANTES de apagar a
	// linha do DB — depois do delete não teríamos mais o username pra
	// chamar o provisionador, e os drop-ins/shares/home ficariam órfãos
	// no VPS (Bugbot: "Delete leaves file access active"). Se o
	// provisionador falhar, NÃO apagamos o usuário (return error) — o
	// admin resolve e tenta de novo; prefiro deixar o usuário intacto
	// no DB do que deixar configs órfãos no sistema. Se a Fase 13 não
	// está configurada (provisioner nil) ou o usuário nunca teve
	// acesso, nada a fazer aqui.
	//
	// fileAccessDisabled rastreia se o Disable efetivamente removeu o
	// acesso — se um passo POSTERIOR (WireGuard ou DB) falhar, os
	// caminhos de compensação precisam restaurar o acesso a arquivos
	// também (Bugbot: "Delete skips file-access compensate"), senão o
	// OS fica off enquanto o DB ainda marca on, e só o reconcile no
	// próximo boot reconvergiria.
	fileAccessDisabled := false
	if a.UserProvisioner != nil && (target.SFTPEnabled || target.SambaEnabled) {
		if err := a.UserProvisioner.Disable(c.Request.Context(), target.Username); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao revogar acesso a arquivos do usuário: " + provisionerErrMsg(err)})
			return
		}
		fileAccessDisabled = true
	}
	// compensateFileAccess restaura o acesso a arquivos que o Disable
	// removeu, se um passo posterior falhar. Best-effort: se falhar,
	// o reconcile no boot converge (DB ainda marca enabled).
	compensateFileAccess := func() {
		if !fileAccessDisabled || a.UserProvisioner == nil {
			return
		}
		// applyAuthorizedKeys (não EnableSFTP com a chave manual) para
		// restaurar também as chaves auto-registradas dos dispositivos, que
		// nesse ponto ainda estão no banco — o delete falhou. A condição
		// também deixou de exigir chave manual não vazia: desde a Fase 14
		// um usuário pode ter SFTP válido só com as chaves dos
		// dispositivos.
		if target.SFTPEnabled {
			_ = a.applyAuthorizedKeys(c.Request.Context(), target)
		}
		if target.SambaEnabled {
			_ = a.UserProvisioner.EnableSamba(c.Request.Context(), target.Username)
		}
	}

	var devices []store.Device
	if err := a.Store.DB.Where("user_id = ?", id).Find(&devices).Error; err != nil {
		compensateFileAccess()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	removed := make([]store.Device, 0, len(devices))
	for _, d := range devices {
		if err := a.WG.RemovePeer(d.PublicKey); err != nil {
			// Compensa as remoções já feitas antes de falhar no meio do
			// loop — sem isso, um subconjunto dos devices do usuário
			// ficaria removido do WG enquanto o registro dele continua
			// inteiro no banco (ver ROADMAP.md Fase 9). Também restaura
			// o acesso a arquivos (Disable já tinha removido).
			a.compensateRestorePeers(removed, "user.delete", id)
			compensateFileAccess()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao revogar dispositivo do usuário na interface WireGuard"})
			return
		}
		removed = append(removed, d)
	}

	err = a.Store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&store.Device{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&store.InviteToken{}).Error; err != nil {
			return err
		}
		// Sem isso, um AppAccess apontando pro usuário apagado sobrevive
		// (Fase 11, PLAN.md §6.8): handleSetMarketplaceAppAccess valida que
		// todo user_id enviado existe, então esse ID órfão nunca mais
		// conseguiria ser removido de volta via PUT .../access (a lista
		// pré-carregada no painel inclui o ID morto, mas nenhum checkbox
		// existe pra desmarcá-lo, travando o admin no primeiro "salvar").
		if err := tx.Where("user_id = ?", id).Delete(&store.AppAccess{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&store.User{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		// A transação de banco falhou (ou o usuário nem existia) depois
		// de já termos removido os peers do kernel — compensa
		// re-adicionando todos, para não deixar o WG "à frente" do
		// banco até um eventual restart reconciliar (ver ROADMAP.md
		// Fase 9). Sem devices removidos (ex.: usuário inexistente),
		// isso é um no-op. Também restaura o acesso a arquivos.
		a.compensateRestorePeers(removed, "user.delete", id)
		compensateFileAccess()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "user.delete", "user_id="+c.Param("id"))

	c.Status(http.StatusNoContent)
}

type inviteResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// handleCreateInvite gera um código de convite de curta duração para o
// usuário indicado, usado pelo cliente desktop no fluxo de enrollment.
// POST /api/users/:id/invite
func (a *App) handleCreateInvite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var user store.User
	if err := a.Store.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}

	token, err := generateInviteToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	invite := store.InviteToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(time.Duration(a.Config.InviteTokenTTLMinutes) * time.Minute),
	}
	if err := a.Store.DB.Create(&invite).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	// Nunca logar o token completo (ver go-backend.mdc) — só o fato de que
	// um convite foi gerado e para quem.
	_ = a.Store.LogAudit(actorString(actor), "invite.create", "user_id="+c.Param("id"))

	c.JSON(http.StatusCreated, inviteResponse{Token: invite.Token, ExpiresAt: invite.ExpiresAt})
}

// generateInviteToken gera um código legível no formato XVPN-XXXX-XXXX,
// usando Base32 (sem caracteres ambíguos como 0/O, 1/I) para ser fácil de
// digitar manualmente se necessário.
func generateInviteToken() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	if len(encoded) < 8 {
		return "", errors.New("falha ao gerar token")
	}
	return "XVPN-" + encoded[:4] + "-" + encoded[4:8], nil
}

func actorString(actor any) string {
	if s, ok := actor.(string); ok && s != "" {
		return s
	}
	return "desconhecido"
}

// compensateRestorePeers re-adiciona peers já removidos do WireGuard quando
// uma operação subsequente (banco) falha no meio de uma revogação em lote —
// mantém o kernel consistente com o banco (que ainda tem usuário/devices
// intactos) em vez de deixar peers "a menos" até o próximo restart
// reconciliar sozinho (ver ROADMAP.md Fase 9).
func (a *App) compensateRestorePeers(removed []store.Device, action string, userID uint64) {
	for _, d := range removed {
		if err := a.WG.AddPeer(wireguard.PeerSpec{PublicKey: d.PublicKey, AllowedIP: d.AllowedIP}); err != nil {
			slog.Error("falha ao compensar remoção de peers", "action", action, "user_id", userID, "device_id", d.ID, "err", err)
		}
	}
}
