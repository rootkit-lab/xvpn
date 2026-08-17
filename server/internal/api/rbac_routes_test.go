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
	app             *App
	router          http.Handler
	token           string
	targetUserID    uint
	targetDeviceID  uint
	ownDeviceID     uint
	waitlistID      uint
	marketAppID     uint
	marketVersionID uint
	marketAssetID   uint
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

	// Fixture do marketplace (Fase 11): um app global com uma versão e um
	// asset cujo blob existe de verdade em disco — assim o caso
	// "download-marketplace-asset" da matriz exercita o caminho completo
	// (não só "não é 403"), já que Visibility global libera qualquer
	// papel autenticado (ver PLAN.md §6.7).
	marketApp := store.App{
		Slug: "rbac-app", Name: "App de teste RBAC", Visibility: store.AppVisibilityGlobal,
		Source: store.AppSourceBuild, SourcePath: "apps/rbac-app",
	}
	if err := app.Store.DB.Create(&marketApp).Error; err != nil {
		t.Fatalf("erro criando app de marketplace de teste: %v", err)
	}
	marketVersion := store.AppVersion{AppID: marketApp.ID, Version: "1.0.0", Channel: store.ChannelStable}
	if err := app.Store.DB.Create(&marketVersion).Error; err != nil {
		t.Fatalf("erro criando versão de marketplace de teste: %v", err)
	}
	putRes, err := app.Marketplace.Put(strings.NewReader("conteudo de teste rbac"))
	if err != nil {
		t.Fatalf("erro gravando blob de teste: %v", err)
	}
	marketAsset := store.AppAsset{
		AppVersionID: marketVersion.ID,
		Platform:     store.PlatformLinux,
		Arch:         "amd64",
		Filename:     "teste.deb",
		SHA256:       putRes.SHA256,
		SizeBytes:    putRes.Size,
		StoragePath:  putRes.RelPath,
	}
	if err := app.Store.DB.Create(&marketAsset).Error; err != nil {
		t.Fatalf("erro criando asset de marketplace de teste: %v", err)
	}

	return rbacFixtures{
		app:             app,
		router:          router,
		token:           token,
		targetUserID:    target.ID,
		targetDeviceID:  targetDevice.ID,
		ownDeviceID:     ownDevice.ID,
		waitlistID:      entry.ID,
		marketAppID:     marketApp.ID,
		marketVersionID: marketVersion.ID,
		marketAssetID:   marketAsset.ID,
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
	{"update-my-ssh-public-key", http.MethodPut, "/api/me/ssh-public-key", updateMySSHPublicKeyRequest{SSHPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE5x portal@test"}, "any"},
	{"change-my-password", http.MethodPatch, "/api/me/password", changeMyPasswordRequest{CurrentPassword: "senha-caller-123", NewPassword: "senha-nova-456"}, "any"},

	{"list-users", http.MethodGet, "/api/users", nil, "viewerUp"},
	{"get-user", http.MethodGet, "/api/users/{user}", nil, "viewerUp"},
	{"list-devices", http.MethodGet, "/api/devices", nil, "viewerUp"},
	{"list-waitlist", http.MethodGet, "/api/waitlist", nil, "viewerUp"},
	{"list-audit", http.MethodGet, "/api/audit", nil, "viewerUp"},
	{"get-config", http.MethodGet, "/api/config", nil, "viewerUp"},
	{"get-dns", http.MethodGet, "/api/dns", nil, "viewerUp"},
	{"marketplace-stats", http.MethodGet, "/api/marketplace/stats", nil, "viewerUp"},

	{"create-user", http.MethodPost, "/api/users", createUserRequest{Username: "gerado-pelo-teste", Password: "senha-valida-123", Role: store.RoleMember}, "adminOnly"},
	{"update-user", http.MethodPatch, "/api/users/{user}", updateUserRequest{Username: strPtr("renomeado-pelo-teste")}, "adminOnly"},
	{"delete-user", http.MethodDelete, "/api/users/{user}", nil, "adminOnly"},
	{"create-invite", http.MethodPost, "/api/users/{user}/invite", nil, "adminOnly"},
	{"reset-password", http.MethodPost, "/api/users/{user}/reset-password", nil, "adminOnly"},
	{"set-file-access", http.MethodPut, "/api/users/{user}/file-access", fileAccessRequest{}, "adminOnly"},
	{"delete-device", http.MethodDelete, "/api/devices/{device}", nil, "adminOnly"},
	{"approve-waitlist", http.MethodPost, "/api/waitlist/{waitlist}/approve", nil, "adminOnly"},
	{"reject-waitlist", http.MethodPost, "/api/waitlist/{waitlist}/reject", nil, "adminOnly"},
	{"provision-waitlist", http.MethodPost, "/api/waitlist/{waitlist}/provision", provisionWaitlistRequest{Username: "gerado-pelo-teste-2"}, "adminOnly"},
	{"update-config", http.MethodPatch, "/api/config", updateConfigRequest{InviteTokenTTLMinutes: intPtr(30)}, "adminOnly"},
	{"update-dns", http.MethodPatch, "/api/dns", updateDNSSettingsRequest{CacheSize: intPtr(200)}, "adminOnly"},
	{"create-dns-record", http.MethodPost, "/api/dns/records", upsertDNSRecordRequest{Hostname: "lab.corp.ihuull.com", IPv4: "10.66.66.9"}, "adminOnly"},
	{"apply-dns", http.MethodPost, "/api/dns/apply", nil, "adminOnly"},

	{"list-marketplace-apps", http.MethodGet, "/api/marketplace/apps", nil, "any"},
	{"download-marketplace-asset", http.MethodGet, "/api/marketplace/assets/{marketAsset}/download", nil, "any"},

	{"set-marketplace-app-access", http.MethodPut, "/api/marketplace/apps/{marketApp}/access", setMarketplaceAppAccessRequest{}, "adminOnly"},

	{"social-people", http.MethodGet, "/api/social/people", nil, "any"},
	{"social-profile-me", http.MethodGet, "/api/social/profile", nil, "any"},
	{"social-profile-patch", http.MethodPatch, "/api/social/profile", patchSocialProfileRequest{DisplayName: strPtr("Caller")}, "any"},
	{"social-profile-get", http.MethodGet, "/api/social/u/target", nil, "any"},
	{"social-follow", http.MethodPost, "/api/social/follow/target", nil, "any"},
	{"social-unfollow", http.MethodDelete, "/api/social/follow/target", nil, "any"},
	{"social-list-groups", http.MethodGet, "/api/social/groups", nil, "any"},
	{"social-create-group", http.MethodPost, "/api/social/groups", createGroupRequest{Name: "grupo-rbac"}, "any"},
	{"social-list-threads", http.MethodGet, "/api/social/threads", nil, "any"},
	{"social-open-thread", http.MethodPost, "/api/social/threads", openThreadRequest{Username: "target"}, "any"},
	{"social-list-stories", http.MethodGet, "/api/social/stories", nil, "any"},
	{"social-feed", http.MethodGet, "/api/social/feed", nil, "any"},
	{"social-create-post", http.MethodPost, "/api/social/posts", createPostRequest{Body: "olá xgroup"}, "any"},
	{"social-star-post", http.MethodPost, "/api/social/posts/1/star", nil, "any"},
	{"social-list-comments", http.MethodGet, "/api/social/posts/1/comments", nil, "any"},
	{"social-create-comment", http.MethodPost, "/api/social/posts/1/comments", createCommentRequest{Body: "ok"}, "any"},
	{"social-repost", http.MethodPost, "/api/social/posts/1/repost", nil, "any"},
}

func strPtr(s string) *string { return &s }
func intPtr(n int) *int       { return &n }

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
	path = strings.ReplaceAll(path, "{marketApp}", strconv.FormatUint(uint64(f.marketAppID), 10))
	path = strings.ReplaceAll(path, "{marketVersion}", strconv.FormatUint(uint64(f.marketVersionID), 10))
	path = strings.ReplaceAll(path, "{marketAsset}", strconv.FormatUint(uint64(f.marketAssetID), 10))
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
			// Um fixture por papel — não por rota. Recriar SQLite+login
			// em cada case (4×N) é o que deixava a matriz lenta.
			f := setupRBACFixtures(t, role)
			for _, rc := range rbacRouteCases {
				rc := rc
				t.Run(rc.name, func(t *testing.T) {
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
