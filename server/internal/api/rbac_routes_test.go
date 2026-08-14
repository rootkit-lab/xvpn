package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// rbacFixtures agrupa os recursos que os testes de matriz de rotas
// precisam para exercitar cada endpoint autenticado sem que checagens
// internas do handler (CanManage, "device de outro dono" etc.) se
// confundam com a checagem de rota (RequireRole) que este arquivo quer
// isolar.
type rbacFixtures struct {
	app            *App
	router         http.Handler
	token          string
	targetUserID   uint
	targetDeviceID uint
	ownDeviceID    uint
	waitlistID     uint
}

// setupRBACFixtures monta um App novo com um usuário autenticado no papel
// pedido, mais um usuário-alvo de rank sempre gerenciável (member) — assim
// super_admin/admin nunca tomam um 403 "de negócio" (CanManage) que
// mascare o resultado da checagem de rota que estamos testando aqui.
func setupRBACFixtures(t *testing.T, role store.Role) rbacFixtures {
	t.Helper()
	app, _ := newTestApp(t)
	router := NewRouter(app)

	caller := createTestUserWithRole(t, app, "caller", "senha-caller-123", role)
	token := loginAndGetToken(t, app, router, "caller", "senha-caller-123")

	target := createTestUserWithRole(t, app, "target", "senha-target-123", store.RoleMember)

	targetDevice := store.Device{UserID: target.ID, Name: "device-do-alvo", PublicKey: testPublicKey, AllowedIP: "10.66.66.50/32"}
	if err := app.Store.DB.Create(&targetDevice).Error; err != nil {
		t.Fatalf("erro criando device do alvo: %v", err)
	}
	ownDevice := store.Device{UserID: caller.ID, Name: "device-proprio", PublicKey: testPublicKey2, AllowedIP: "10.66.66.51/32"}
	if err := app.Store.DB.Create(&ownDevice).Error; err != nil {
		t.Fatalf("erro criando device próprio: %v", err)
	}

	entry := store.WaitlistEntry{Name: "Fulano", Email: "fulano@example.com", Status: waitlistStatusPending, CreatedAt: time.Now()}
	if err := app.Store.DB.Create(&entry).Error; err != nil {
		t.Fatalf("erro criando cadastro de waitlist: %v", err)
	}

	return rbacFixtures{
		app:            app,
		router:         router,
		token:          token,
		targetUserID:   target.ID,
		targetDeviceID: targetDevice.ID,
		ownDeviceID:    ownDevice.ID,
		waitlistID:     entry.ID,
	}
}

// rbacRouteCase descreve um endpoint autenticado e o tier mínimo de papel
// exigido pela matriz da Fase 10 (ver PLAN.md §6.7 e server.go). path é um
// template com "{user}"/"{device}"/"{waitlist}" substituídos pelos IDs dos
// fixtures de cada subteste.
type rbacRouteCase struct {
	name   string
	method string
	path   string
	body   any
	// tier: "any" (qualquer papel autenticado), "viewerUp" (viewer/admin/
	// super_admin) ou "adminOnly" (admin/super_admin).
	tier string
}

var rbacRouteCases = []rbacRouteCase{
	{"auth-me", http.MethodGet, "/api/auth/me", nil, "any"},
	{"list-my-devices", http.MethodGet, "/api/me/devices", nil, "any"},
	{"delete-my-device", http.MethodDelete, "/api/me/devices/{ownDevice}", nil, "any"},

	{"list-users", http.MethodGet, "/api/users", nil, "viewerUp"},
	{"list-devices", http.MethodGet, "/api/devices", nil, "viewerUp"},
	{"list-waitlist", http.MethodGet, "/api/waitlist", nil, "viewerUp"},
	{"list-audit", http.MethodGet, "/api/audit", nil, "viewerUp"},
	{"get-config", http.MethodGet, "/api/config", nil, "viewerUp"},

	{"create-user", http.MethodPost, "/api/users", createUserRequest{Username: "gerado-pelo-teste", Password: "senha-valida-123", Role: store.RoleMember}, "adminOnly"},
	{"update-user", http.MethodPatch, "/api/users/{user}", updateUserRequest{Username: strPtr("renomeado-pelo-teste")}, "adminOnly"},
	{"delete-user", http.MethodDelete, "/api/users/{user}", nil, "adminOnly"},
	{"create-invite", http.MethodPost, "/api/users/{user}/invite", nil, "adminOnly"},
	{"reset-password", http.MethodPost, "/api/users/{user}/reset-password", nil, "adminOnly"},
	{"delete-device", http.MethodDelete, "/api/devices/{device}", nil, "adminOnly"},
	{"approve-waitlist", http.MethodPost, "/api/waitlist/{waitlist}/approve", nil, "adminOnly"},
	{"reject-waitlist", http.MethodPost, "/api/waitlist/{waitlist}/reject", nil, "adminOnly"},
	{"provision-waitlist", http.MethodPost, "/api/waitlist/{waitlist}/provision", provisionWaitlistRequest{Username: "gerado-pelo-teste-2"}, "adminOnly"},
}

func strPtr(s string) *string { return &s }

func tierAllows(tier string, role store.Role) bool {
	switch tier {
	case "any":
		return true
	case "viewerUp":
		for _, r := range store.ViewerUpRoles {
			if r == role {
				return true
			}
		}
		return false
	case "adminOnly":
		for _, r := range store.AdminRoles {
			if r == role {
				return true
			}
		}
		return false
	default:
		panic("tier desconhecido: " + tier)
	}
}

func resolveRoutePath(path string, f rbacFixtures) string {
	path = strings.ReplaceAll(path, "{user}", strconv.FormatUint(uint64(f.targetUserID), 10))
	path = strings.ReplaceAll(path, "{device}", strconv.FormatUint(uint64(f.targetDeviceID), 10))
	path = strings.ReplaceAll(path, "{ownDevice}", strconv.FormatUint(uint64(f.ownDeviceID), 10))
	path = strings.ReplaceAll(path, "{waitlist}", strconv.FormatUint(uint64(f.waitlistID), 10))
	return path
}

// TestRBACRouteMatrix percorre os quatro papéis × todos os endpoints
// autenticados e confirma que a checagem de rota (auth.RequireRole em
// server.go) bate exatamente com a tabela de PLAN.md §6.7: member nunca
// acessa telas de admin (nem leitura), viewer só lê, admin/super_admin têm
// escrita completa. Não valida o resultado *funcional* de cada chamada
// (isso já é coberto nos testes específicos de cada handler) — só que o
// papel errado nunca passa da porta (403) e o papel certo nunca é barrado
// por engano nessa camada.
func TestRBACRouteMatrix(t *testing.T) {
	roles := []store.Role{store.RoleSuperAdmin, store.RoleAdmin, store.RoleViewer, store.RoleMember}

	for _, role := range roles {
		role := role
		t.Run(string(role), func(t *testing.T) {
			for _, rc := range rbacRouteCases {
				rc := rc
				t.Run(rc.name, func(t *testing.T) {
					f := setupRBACFixtures(t, role)
					path := resolveRoutePath(rc.path, f)

					rec := doJSON(t, f.router, rc.method, path, rc.body, f.token)

					wantAllowed := tierAllows(rc.tier, role)
					gotForbidden := rec.Code == http.StatusForbidden
					if wantAllowed && gotForbidden {
						t.Fatalf("papel %q deveria poder chamar %s %s, mas levou 403: %s", role, rc.method, path, rec.Body.String())
					}
					if !wantAllowed && !gotForbidden {
						t.Fatalf("papel %q não deveria poder chamar %s %s, mas não levou 403 (obtido %d): %s", role, rc.method, path, rec.Code, rec.Body.String())
					}
				})
			}
		})
	}
}

// TestRBACRouteMatrix_UnauthenticatedAlwaysRejected garante que nenhum dos
// endpoints da matriz aceita requisição sem token — 401, nunca 403 (403
// significa "autenticado mas sem papel suficiente"; sem token o correto é
// sempre 401, ver auth.RequireAuth).
func TestRBACRouteMatrix_UnauthenticatedAlwaysRejected(t *testing.T) {
	f := setupRBACFixtures(t, store.RoleSuperAdmin)
	for _, rc := range rbacRouteCases {
		rc := rc
		t.Run(rc.name, func(t *testing.T) {
			path := resolveRoutePath(rc.path, f)
			rec := doJSON(t, f.router, rc.method, path, rc.body, "")
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("esperado 401 sem token em %s %s, obtido %d: %s", rc.method, path, rec.Code, rec.Body.String())
			}
		})
	}
}
