package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/bitlaunch"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestBitLaunchAccountCRUDHidesToken(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/compute/settings/accounts", upsertBitLaunchAccountRequest{
		Name: "Pessoal", Email: "eu@ihuull.com", Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9xxxx",
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9xxxx") {
		t.Fatal("token completo não pode voltar no JSON")
	}
	var created bitLaunchAccountJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Email != "eu@ihuull.com" || created.TokenHint == "" || created.ID == 0 {
		t.Fatalf("create resp: %+v", created)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/compute/settings/accounts", upsertBitLaunchAccountRequest{
		Name: "Trabalho", Email: "ops@ihuull.com", Token: "segundo-token-bitlaunch-ok",
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("segunda conta: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/compute/settings", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "segundo-token-bitlaunch-ok") {
		t.Fatal("GET não pode vazar token")
	}
	var settings struct {
		Accounts  []bitLaunchAccountJSON `json:"accounts"`
		BitLaunch bool                   `json:"bitlaunch"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if !settings.BitLaunch || len(settings.Accounts) != 2 {
		t.Fatalf("settings: %+v", settings)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/servers", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list servers: %d", rec.Code)
	}
	var listed struct {
		BitLaunch bool                   `json:"bitlaunch"`
		Accounts  []bitLaunchAccountJSON `json:"accounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if !listed.BitLaunch || len(listed.Accounts) != 2 {
		t.Fatalf("servers deve listar contas: %+v", listed)
	}

	rec = doJSON(t, router, http.MethodPatch, "/api/compute/settings/accounts/"+strconv.FormatUint(uint64(created.ID), 10), upsertBitLaunchAccountRequest{
		Name: "Pessoal 2", Email: "eu@ihuull.com",
	}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch sem token: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9xxxx") {
		t.Fatal("PATCH não pode vazar token")
	}
	var kept store.BitLaunchAccount
	if err := app.Store.DB.First(&kept, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if kept.Token != "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9xxxx" || kept.Name != "Pessoal 2" {
		t.Fatalf("PATCH deveria manter o token: %+v", kept)
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/compute/settings/accounts/"+strconv.FormatUint(uint64(created.ID), 10), nil, tok)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithoutComputeCannotWriteBitLaunchAccount(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductCore})
	rec := doJSON(t, f.router, http.MethodPost, "/api/compute/settings/accounts", upsertBitLaunchAccountRequest{
		Name: "x", Email: "x@ihuull.com", Token: "token-com-mais-de-16",
	}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin só-core não deveria gravar conta BitLaunch, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSeedBitLaunchEnvAccountOnce(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.BitLaunchToken = "token-env-com-mais-16"
	if err := app.SeedBitLaunchEnvAccount(); err != nil {
		t.Fatal(err)
	}
	if err := app.SeedBitLaunchEnvAccount(); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := app.Store.DB.Model(&store.BitLaunchAccount{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("seed deveria ser idempotente, n=%d", n)
	}
}

func TestBitLaunchAccountBalanceAndTopUp(t *testing.T) {
	app, _ := newTestApp(t)
	fake := &fakeBitLaunch{account: bitlaunch.Account{
		Email: "ops@ihuull.com", Balance: 45000, Used: 2, Limit: 8, CostPerHr: 30, BillingAlert: 5,
	}}
	app.BitLaunch = fake
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	acc := store.BitLaunchAccount{Name: "Ops", Email: "ops@ihuull.com", Token: "token-conta-saldo-16"}
	if err := app.Store.DB.Create(&acc).Error; err != nil {
		t.Fatal(err)
	}
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodGet, "/api/compute/settings", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "token-conta-saldo-16") {
		t.Fatal("GET não pode vazar token")
	}
	var settings struct {
		Accounts []bitLaunchAccountJSON `json:"accounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings.Accounts) != 1 || settings.Accounts[0].BalanceUSD == nil || *settings.Accounts[0].BalanceUSD != 45 {
		t.Fatalf("saldo: %+v", settings.Accounts)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/compute/settings/accounts/"+strconv.FormatUint(uint64(acc.ID), 10)+"/topup",
		topUpRequest{AmountUSD: 20, CryptoSymbol: "btc"}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("topup: %d %s", rec.Code, rec.Body.String())
	}
	var tx topUpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tx); err != nil {
		t.Fatal(err)
	}
	if tx.Address == "" || tx.CryptoSymbol != "BTC" || fake.topUp.AmountUSD != 20 {
		t.Fatalf("invoice: %+v topUp=%+v", tx, fake.topUp)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/compute/settings/accounts/"+strconv.FormatUint(uint64(acc.ID), 10)+"/topup",
		topUpRequest{AmountUSD: 20, CryptoSymbol: "DOGE"}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("DOGE deveria 400, veio %d", rec.Code)
	}
}
