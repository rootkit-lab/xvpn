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
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	TokenHint   string   `json:"token_hint"`
	BalanceUSD  *float64 `json:"balance_usd,omitempty"`
	Used        *int     `json:"used,omitempty"`
	Limit       *int     `json:"limit,omitempty"`
	CostPerHr   *float64 `json:"cost_per_hr,omitempty"`
	BillingDays *int     `json:"billing_alert_days,omitempty"`
}

type topUpRequest struct {
	AmountUSD    float64 `json:"amount_usd"`
	CryptoSymbol string  `json:"crypto_symbol"`
}

type topUpResponse struct {
	ID           string  `json:"id"`
	Address      string  `json:"address"`
	CryptoSymbol string  `json:"crypto_symbol"`
	AmountUSD    float64 `json:"amount_usd"`
	AmountCrypto string  `json:"amount_crypto"`
	Status       string  `json:"status"`
	StatusURL    string  `json:"status_url"`
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

func milliUSD(v int) float64 {
	return float64(v) / 1000
}

func (a *App) accountWithBalance(row store.BitLaunchAccount) bitLaunchAccountJSON {
	out := accountJSON(row)
	cli, _, err := a.resolveBitLaunch(row.ID)
	if err != nil {
		return out
	}
	info, err := cli.Account()
	if err != nil {
		return out
	}
	bal := milliUSD(info.Balance)
	cost := milliUSD(info.CostPerHr)
	used, limit, days := info.Used, info.Limit, info.BillingAlert
	out.BalanceUSD = &bal
	out.Used = &used
	out.Limit = &limit
	out.CostPerHr = &cost
	out.BillingDays = &days
	return out
}

func (a *App) handleGetComputeSettings(c *gin.Context) {
	var rows []store.BitLaunchAccount
	if err := a.Store.DB.Order("id").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]bitLaunchAccountJSON, 0, len(rows))
	for _, r := range rows {
		items = append(items, a.accountWithBalance(r))
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

func (a *App) handleCreateBitLaunchTopUp(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var req topUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	sym := strings.ToUpper(strings.TrimSpace(req.CryptoSymbol))
	if req.AmountUSD < 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount_usd mínimo é 5"})
		return
	}
	if sym != "BTC" && sym != "LTC" && sym != "ETH" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "crypto_symbol deve ser BTC, LTC ou ETH"})
		return
	}
	cli, _, err := a.resolveBitLaunch(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conta não encontrada"})
		return
	}
	tx, err := cli.CreateTransaction(bitlaunch.TopUpOpts{AmountUSD: req.AmountUSD, CryptoSymbol: sym})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "falha ao criar recarga no BitLaunch"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.account.topup", sym)
	c.JSON(http.StatusCreated, topUpResponse{
		ID: tx.ID, Address: tx.Address, CryptoSymbol: tx.CryptoSymbol,
		AmountUSD: tx.AmountUSD, AmountCrypto: tx.AmountCrypto,
		Status: tx.Status, StatusURL: tx.StatusURL,
	})
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
