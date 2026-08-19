package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func setupSvcApp(t *testing.T) (*App, http.Handler, string, *fakeUserProvisioner) {
	t.Helper()
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	app.Config.WireGuardAllowedSubnet = "10.66.66.0/24"
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	return app, router, tok, fp
}

func TestCreateLocalRedisAppliesAndHidesPasswordOnGet(t *testing.T) {
	app, router, tok, fp := setupSvcApp(t)
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "Lab"}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("project: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/services", createServiceRequest{
		Slug: "cache", Kind: "redis", Host: "local", Bind: "wg0", ProjectSlug: "lab",
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created serviceJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Password == "" || created.Status != store.SvcReady {
		t.Fatalf("create: %+v", created)
	}
	if created.Hostname != "svc-cache.corp.ihuull.com" {
		t.Fatalf("hostname: %s", created.Hostname)
	}
	if created.Listen != "10.66.66.1" || created.Port != 6379 {
		t.Fatalf("listen: %s:%d", created.Listen, created.Port)
	}
	if !strings.Contains(strings.Join(fp.calls, "\n"), "ApplySvc(") {
		t.Fatalf("ApplySvc não chamado: %v", fp.calls)
	}

	var recDNS store.DNSRecord
	if err := app.Store.DB.Where("hostname = ?", "svc-cache.corp.ihuull.com").First(&recDNS).Error; err != nil {
		t.Fatal(err)
	}
	if recDNS.IPv4 != "10.66.66.1" {
		t.Fatalf("dns: %+v", recDNS)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/services/cache", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	var got serviceJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Password != "" {
		t.Fatal("GET não deve devolver a senha")
	}
	if !strings.Contains(got.Endpoint, "svc-cache.corp.ihuull.com") {
		t.Fatalf("endpoint: %s", got.Endpoint)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/services", nil, tok)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"cache"`) {
		t.Fatalf("project services: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateManagedMongoRejectsControlPlanePort(t *testing.T) {
	_, router, tok, _ := setupSvcApp(t)
	rec := doJSON(t, router, http.MethodPost, "/api/services", createServiceRequest{
		Slug: "appdb", Kind: "mongo", Host: "local", Bind: "wg0", Port: 27017,
	}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400, veio %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateMeshRedisWithoutWgIPRejected(t *testing.T) {
	app, router, tok, _ := setupSvcApp(t)
	srv := store.MeshServer{
		BitLaunchID: "m1", Name: "edge", Hostname: "edge1",
		Role: store.ServerRoleMesh, Status: "active",
	}
	if err := app.Store.DB.Create(&srv).Error; err != nil {
		t.Fatal(err)
	}
	id := srv.ID
	rec := doJSON(t, router, http.MethodPost, "/api/services", createServiceRequest{
		Slug: "q", Kind: "redis", Host: "mesh", MeshServerID: &id, Bind: "wg0",
	}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400, veio %d %s", rec.Code, rec.Body.String())
	}
}

func TestSvcAgentDesiredAndStatus(t *testing.T) {
	app, router, tok, _ := setupSvcApp(t)
	plain := "agent-token-svc-test-ok"
	hash, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatal(err)
	}
	srv := store.MeshServer{
		BitLaunchID: "m2", Name: "mesh-1", Hostname: "mesh1",
		Role: store.ServerRoleMesh, WgIP: "10.66.66.8", Status: "active",
		AgentTokenHash: hash,
	}
	if err := app.Store.DB.Create(&srv).Error; err != nil {
		t.Fatal(err)
	}
	id := srv.ID
	rec := doJSON(t, router, http.MethodPost, "/api/services", createServiceRequest{
		Slug: "queue", Kind: "redis", Host: "mesh", MeshServerID: &id, Bind: "wg0",
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created serviceJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != store.SvcPending {
		t.Fatalf("mesh deveria ficar pending: %+v", created)
	}

	rec = doJSONFrom(t, router, http.MethodGet, "/api/svc/desired", nil, "203.0.113.8:9", map[string]string{
		"Authorization": "Bearer " + plain,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("fora da VPN deveria 403, veio %d", rec.Code)
	}

	rec = doJSONFrom(t, router, http.MethodGet, "/api/svc/desired", nil, "10.66.66.8:9", map[string]string{
		"Authorization": "Bearer " + plain,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("desired: %d %s", rec.Code, rec.Body.String())
	}
	var desired struct {
		Items []svcDesiredJSON `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &desired); err != nil {
		t.Fatal(err)
	}
	if len(desired.Items) != 1 || desired.Items[0].Password == "" || desired.Items[0].Bind != "10.66.66.8" {
		t.Fatalf("desired: %+v", desired.Items)
	}

	rec = doJSONFrom(t, router, http.MethodPost, "/api/svc/"+itoa(created.ID)+"/status",
		svcAgentStatusRequest{Status: "ready"}, "10.66.66.8:9", map[string]string{
			"Authorization": "Bearer " + plain,
		})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/services/queue", nil, tok)
	var got serviceJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != store.SvcReady {
		t.Fatalf("após agent: %+v", got)
	}
}

func TestIssueAgentTokenRejectsControl(t *testing.T) {
	app, router, tok, _ := setupSvcApp(t)
	s := store.MeshServer{
		BitLaunchID: "ctrl", Name: "vps", Hostname: "control",
		Role: store.ServerRoleControl, Status: "active",
	}
	if err := app.Store.DB.Create(&s).Error; err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, router, http.MethodPost, "/api/servers/"+itoa(s.ID)+"/agent-token", nil, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("control token: %d %s", rec.Code, rec.Body.String())
	}

	s2 := store.MeshServer{
		BitLaunchID: "mesh3", Name: "n2", Hostname: "n2",
		Role: store.ServerRoleMesh, WgIP: "10.66.66.7", Status: "active",
	}
	if err := app.Store.DB.Create(&s2).Error; err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodPost, "/api/servers/"+itoa(s2.ID)+"/agent-token", nil, tok)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "agent_token") {
		t.Fatalf("mesh token: %d %s", rec.Code, rec.Body.String())
	}
}

func TestGetServiceOmitsAuthSecret(t *testing.T) {
	app, router, tok, _ := setupSvcApp(t)
	rec := doJSON(t, router, http.MethodPost, "/api/services", createServiceRequest{
		Slug: "secret", Kind: "redis", Host: "local", Bind: "loopback",
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "auth_secret") {
		t.Fatal("auth_secret vazou no JSON")
	}
	var row store.ServiceInstance
	if err := app.Store.DB.Where("slug = ?", "secret").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.AuthSecret == "" {
		t.Fatal("senha deveria persistir no banco")
	}
}
