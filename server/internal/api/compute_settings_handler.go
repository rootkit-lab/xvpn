package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/bitlaunch"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type bitLaunchAccountJSON struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	TokenHint string `json:"token_hint"`
}

type upsertBitLaunchAccountRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Token string `json:"token"`
}

func tokenHint(tok string) string {
	t := strings.TrimSpace(tok)
	if len(t) < 8 {
		return "••••"
	}
	return "…" + t[len(t)-4:]
}

func accountJSON(a store.BitLaunchAccount) bitLaunchAccountJSON {
	return bitLaunchAccountJSON{ID: a.ID, Name: a.Name, Email: a.Email, TokenHint: tokenHint(a.Token)}
}

func (a *App) handleGetComputeSettings(c *gin.Context) {
	var rows []store.BitLaunchAccount
	if err := a.Store.DB.Order("id").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]bitLaunchAccountJSON, 0, len(rows))
	for _, r := range rows {
		items = append(items, accountJSON(r))
	}
	c.JSON(http.StatusOK, gin.H{"accounts": items, "bitlaunch": a.bitLaunchReady()})
}

func (a *App) handleCreateBitLaunchAccount(c *gin.Context) {
	var req upsertBitLaunchAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	name, email, token, err := normalizeBitLaunchAccount(req.Name, req.Email, req.Token, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row := store.BitLaunchAccount{Name: name, Email: email, Token: token}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "já existe uma conta com este e-mail"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.account.create", email)
	c.JSON(http.StatusCreated, accountJSON(row))
}

func (a *App) handleUpdateBitLaunchAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var row store.BitLaunchAccount
	if err := a.Store.DB.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conta não encontrada"})
		return
	}
	var req upsertBitLaunchAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	needToken := strings.TrimSpace(req.Token) != ""
	token := row.Token
	if needToken {
		token = strings.TrimSpace(req.Token)
	}
	name, email, token, err := normalizeBitLaunchAccount(req.Name, req.Email, token, needToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !needToken {
		token = row.Token
	}
	row.Name, row.Email, row.Token = name, email, token
	if err := a.Store.DB.Save(&row).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "já existe uma conta com este e-mail"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.account.update", email)
	c.JSON(http.StatusOK, accountJSON(row))
}

func (a *App) handleDeleteBitLaunchAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	res := a.Store.DB.Delete(&store.BitLaunchAccount{}, id)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "conta não encontrada"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.account.delete", strconv.FormatUint(id, 10))
	c.Status(http.StatusNoContent)
}

func normalizeBitLaunchAccount(name, email, token string, requireToken bool) (string, string, string, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	token = strings.TrimSpace(token)
	if name == "" {
		return "", "", "", errMsg("name é obrigatório")
	}
	if email == "" || !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return "", "", "", errMsg("email inválido")
	}
	if requireToken && len(token) < 16 {
		return "", "", "", errMsg("token BitLaunch inválido")
	}
	return name, email, token, nil
}

type errMsg string

func (e errMsg) Error() string { return string(e) }

func (a *App) bitLaunchReady() bool {
	if a.BitLaunch != nil {
		return true
	}
	var n int64
	if err := a.Store.DB.Model(&store.BitLaunchAccount{}).Count(&n).Error; err != nil {
		return false
	}
	return n > 0
}

func (a *App) listBitLaunchAccounts() []store.BitLaunchAccount {
	var rows []store.BitLaunchAccount
	_ = a.Store.DB.Order("id").Find(&rows).Error
	return rows
}

func (a *App) resolveBitLaunch(accountID uint) (BitLaunchAPI, uint, error) {
	if accountID != 0 {
		var acc store.BitLaunchAccount
		if err := a.Store.DB.First(&acc, accountID).Error; err != nil {
			return nil, 0, err
		}
		if a.BitLaunch != nil {
			return a.BitLaunch, acc.ID, nil
		}
		return bitlaunch.New(acc.Token), acc.ID, nil
	}
	if a.BitLaunch != nil {
		return a.BitLaunch, 0, nil
	}
	var acc store.BitLaunchAccount
	if err := a.Store.DB.Order("id").First(&acc).Error; err != nil {
		return nil, 0, err
	}
	return bitlaunch.New(acc.Token), acc.ID, nil
}

func (a *App) SeedBitLaunchEnvAccount() error {
	if a.Config == nil || strings.TrimSpace(a.Config.BitLaunchToken) == "" {
		return nil
	}
	var n int64
	if err := a.Store.DB.Model(&store.BitLaunchAccount{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return a.Store.DB.Create(&store.BitLaunchAccount{
		Name:  "env",
		Email: "env@bitlaunch.local",
		Token: strings.TrimSpace(a.Config.BitLaunchToken),
	}).Error
}
