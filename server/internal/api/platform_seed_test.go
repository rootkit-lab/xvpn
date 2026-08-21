package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestRegisterManualMeshServer(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/servers/register", registerManualMeshServerRequest{
		Hostname: "data", IPv4: store.DataNodeIPv4, Role: "mesh", Notes: "nó de dados",
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	var created meshServerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Hostname != store.DataHostname || created.Provider != "manual" || created.EnrollToken == "" {
		t.Fatalf("resp: %+v", created)
	}
	if created.Bootstrap == "" ||
		!strings.Contains(created.Bootstrap, "wg genkey") ||
		!strings.Contains(created.Bootstrap, "xvpn.ihuull.com/api/servers/enroll") {
		t.Fatalf("bootstrap ausente ou incompleto")
	}
	if !created.Protected {
		t.Fatal("data node deve ser protegido (sem destroy BitLaunch)")
	}

	rec = doJSON(t, router, http.MethodPost, "/api/servers/register", map[string]string{
		"hostname": "evil", "ipv4": "8.8.8.8", "ssh_private_key": "-----BEGIN",
	}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("private key deveria 400, veio %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/servers/register", registerManualMeshServerRequest{
		Hostname: "data", IPv4: store.DataNodeIPv4,
	}, tok)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicata deveria 409, veio %d", rec.Code)
	}
}

func TestSeedDataNodeAndPlatformRepo(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	if err := app.SeedDataNode(); err != nil {
		t.Fatal(err)
	}
	if err := app.SeedDataNode(); err != nil {
		t.Fatal(err)
	}
	if err := app.SeedPlatformRepo(); err != nil {
		t.Fatal(err)
	}
	if err := app.SeedPlatformRepo(); err != nil {
		t.Fatal(err)
	}
	var mesh, proj int64
	if err := app.Store.DB.Model(&store.MeshServer{}).Where("ipv4 = ?", store.DataNodeIPv4).Count(&mesh).Error; err != nil {
		t.Fatal(err)
	}
	if mesh != 1 {
		t.Fatalf("data nodes=%d", mesh)
	}
	if err := app.Store.DB.Model(&store.Project{}).Where("slug = ?", store.PlatformRepoSlug).Count(&proj).Error; err != nil {
		t.Fatal(err)
	}
	if proj != 1 {
		t.Fatalf("platform repos=%d", proj)
	}
	var s store.MeshServer
	if err := app.Store.DB.Where("ipv4 = ?", store.DataNodeIPv4).First(&s).Error; err != nil {
		t.Fatal(err)
	}
	if s.Role != store.ServerRoleMesh || !strings.HasPrefix(s.BitLaunchID, store.ManualIDPrefix) {
		t.Fatalf("server: %+v", s)
	}

	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	rec := doJSON(t, router, http.MethodPost, "/api/servers/"+strconv.FormatUint(uint64(s.ID), 10)+"/enroll-token", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("enroll-token: %d %s", rec.Code, rec.Body.String())
	}
	var revealed meshServerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &revealed); err != nil {
		t.Fatal(err)
	}
	if revealed.EnrollToken == "" || revealed.Bootstrap == "" {
		t.Fatalf("token/bootstrap deveriam vir na resposta: %+v", revealed)
	}
	listRec := doJSON(t, router, http.MethodGet, "/api/servers", nil, tok)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: %d", listRec.Code)
	}
	if strings.Contains(listRec.Body.String(), revealed.EnrollToken) {
		t.Fatal("list não deve vazar enroll_token")
	}
}

func TestAdminWithoutComputeCannotRegisterManual(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	shop := createTestUserWithRole(t, app, "shop", "senha-shop-okkk", store.RoleAdmin)
	shop.Products = []store.Product{store.ProductMarketplace}
	if err := app.Store.DB.Save(&shop).Error; err != nil {
		t.Fatal(err)
	}
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "shop", "senha-shop-okkk")
	rec := doJSON(t, router, http.MethodPost, "/api/servers/register", registerManualMeshServerRequest{
		Hostname: "edge", IPv4: "9.9.9.9",
	}, tok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("sem compute deveria 403, veio %d %s", rec.Code, rec.Body.String())
	}
}
