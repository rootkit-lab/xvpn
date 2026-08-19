package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// setupScopedAdmin monta um fixture por papel+escopo (não por rota): um
// admin com products explícitos, mais os IDs que as escritas de produto
// precisam. A matriz da Fase 10 (rbac_routes_test.go) continua cobrindo
// admin irrestrito (lista vazia).
func setupScopedAdmin(t *testing.T, products []store.Product) rbacFixtures {
	t.Helper()
	f := setupRBACFixtures(t, store.RoleAdmin)
	if err := f.app.Store.DB.Model(&store.User{}).Where("username = ?", "caller").Select("Products").Updates(store.User{Products: products}).Error; err != nil {
		t.Fatalf("erro gravando escopo do caller: %v", err)
	}
	// Relogin não é necessário: refreshCallerFromDB lê products do banco.
	return f
}

func TestAdminWithoutMarketplaceScopeCannotWriteStoreACL(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductCore})
	path := "/api/marketplace/apps/" + strconv.FormatUint(uint64(f.marketAppID), 10) + "/access"
	rec := doJSON(t, f.router, http.MethodPut, path, setMarketplaceAppAccessRequest{}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin só-core não deveria escrever ACL da loja, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithMarketplaceScopeCanWriteStoreACL(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductMarketplace})
	path := "/api/marketplace/apps/" + strconv.FormatUint(uint64(f.marketAppID), 10) + "/access"
	rec := doJSON(t, f.router, http.MethodPut, path, setMarketplaceAppAccessRequest{}, f.token)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("admin com escopo marketplace foi barrado na ACL: %s", rec.Body.String())
	}
}

func TestAdminWithoutCoreScopeCannotDeleteDevice(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductMarketplace})
	path := "/api/devices/" + strconv.FormatUint(uint64(f.targetDeviceID), 10)
	rec := doJSON(t, f.router, http.MethodDelete, path, nil, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin só-loja não deveria revogar peer, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithoutXDriverScopeCannotSetFileAccess(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductMarketplace})
	path := "/api/users/" + strconv.FormatUint(uint64(f.targetUserID), 10) + "/file-access"
	rec := doJSON(t, f.router, http.MethodPut, path, fileAccessRequest{}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin sem xdriver não deveria alterar file-access, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithoutCoreScopeCannotPatchConfig(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductXDriver})
	rec := doJSON(t, f.router, http.MethodPatch, "/api/config", updateConfigRequest{InviteTokenTTLMinutes: intPtr(30)}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin sem core não deveria alterar config, obtido %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, f.router, http.MethodPatch, "/api/config/xcodespaces", patchCodespaceSettingsRequest{Provider: ptr("glm")}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin sem core não deveria alterar o assistente, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithoutManagedScopeCannotCreateService(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductCore})
	rec := doJSON(t, f.router, http.MethodPost, "/api/services", createServiceRequest{Slug: "cache", Kind: "redis"}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin sem managed não deveria criar serviço, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithManagedScopeCanCreateService(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductManaged})
	f.app.UserProvisioner = &fakeUserProvisioner{}
	rec := doJSON(t, f.router, http.MethodPost, "/api/services", createServiceRequest{Slug: "cache", Kind: "redis", Host: "local", Bind: "wg0"}, f.token)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("admin com managed foi barrado: %s", rec.Body.String())
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithoutDNSScopeCannotWriteDNS(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductCore})
	rec := doJSON(t, f.router, http.MethodPatch, "/api/dns", updateDNSSettingsRequest{CacheSize: intPtr(200)}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin só-core não deveria escrever DNS, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithDNSScopeCanWriteDNS(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductDNS})
	rec := doJSON(t, f.router, http.MethodPost, "/api/dns/records",
		upsertDNSRecordRequest{Hostname: "lab.corp.ihuull.com", IPv4: "10.66.66.9"}, f.token)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("admin com escopo dns foi barrado: %s", rec.Body.String())
	}
}

func TestAdminWithoutComputeScopeCannotCreateServerGroup(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductCore})
	rec := doJSON(t, f.router, http.MethodPost, "/api/server-groups", createServerGroupRequest{Name: "edge"}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin só-core não deveria criar grupo compute, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUnrestrictedAdminStillWritesAllProducts(t *testing.T) {
	// Lista vazia = admin da Fase 10. Garante que o middleware novo não
	// quebrou o papel admin da matriz existente.
	f := setupRBACFixtures(t, store.RoleAdmin)
	acl := "/api/marketplace/apps/" + strconv.FormatUint(uint64(f.marketAppID), 10) + "/access"
	rec := doJSON(t, f.router, http.MethodPut, acl, setMarketplaceAppAccessRequest{}, f.token)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("admin irrestrito foi barrado na ACL: %s", rec.Body.String())
	}
	dev := "/api/devices/" + strconv.FormatUint(uint64(f.targetDeviceID), 10)
	rec = doJSON(t, f.router, http.MethodDelete, dev, nil, f.token)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("admin irrestrito foi barrado ao revogar device: %s", rec.Body.String())
	}
}

func TestSuperAdminIgnoresStoredProducts(t *testing.T) {
	f := setupRBACFixtures(t, store.RoleSuperAdmin)
	if err := f.app.Store.DB.Model(&store.User{}).Where("username = ?", "caller").Select("Products").Updates(store.User{Products: []store.Product{store.ProductMarketplace}}).Error; err != nil {
		t.Fatalf("erro gravando products no super_admin: %v", err)
	}
	path := "/api/devices/" + strconv.FormatUint(uint64(f.targetDeviceID), 10)
	rec := doJSON(t, f.router, http.MethodDelete, path, nil, f.token)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("super_admin não deveria ser limitado por products: %s", rec.Body.String())
	}
}

func TestAdminWithoutCoreScopeCannotDeleteUser(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductMarketplace})
	path := "/api/users/" + strconv.FormatUint(uint64(f.targetUserID), 10)
	rec := doJSON(t, f.router, http.MethodDelete, path, nil, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin só-loja não deveria apagar usuário (revoga peers), obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithoutCoverCannotResetUnrestrictedAdmin(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductMarketplace})
	peer := createTestUserWithRole(t, f.app, "admin-irrestrito", "senha-irrestrito-ok", store.RoleAdmin)
	path := "/api/users/" + strconv.FormatUint(uint64(peer.ID), 10) + "/reset-password"
	rec := doJSON(t, f.router, http.MethodPost, path, nil, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin da loja não deveria resetar senha de admin irrestrito, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithCoverCanResetMemberPassword(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductMarketplace})
	path := "/api/users/" + strconv.FormatUint(uint64(f.targetUserID), 10) + "/reset-password"
	rec := doJSON(t, f.router, http.MethodPost, path, nil, f.token)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("admin da loja deveria resetar senha de member (IAM): %s", rec.Body.String())
	}
}

func TestAdminWithoutCoverCannotPatchUnrestrictedAdmin(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductMarketplace})
	peer := createTestUserWithRole(t, f.app, "admin-livre", "senha-livre-ok", store.RoleAdmin)
	path := "/api/users/" + strconv.FormatUint(uint64(peer.ID), 10)
	rec := doJSON(t, f.router, http.MethodPatch, path, updateUserRequest{Username: strPtr("renomeado-ataque")}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin da loja não deveria editar admin irrestrito, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUser_ScopedAdminInheritsProducts(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductMarketplace})
	rec := doJSON(t, f.router, http.MethodPost, "/api/users",
		createUserRequest{Username: "loja-jr", Password: "senha-valida-123", Role: store.RoleAdmin}, f.token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var created userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(created.Products) != 1 || created.Products[0] != store.ProductMarketplace {
		t.Fatalf("admin da loja deveria herdar products, obtido %v", created.Products)
	}
}

func TestCreateUser_ScopedAdminCannotGrantCore(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductMarketplace})
	rec := doJSON(t, f.router, http.MethodPost, "/api/users",
		createUserRequest{Username: "invasor", Password: "senha-valida-123", Role: store.RoleAdmin, Products: []store.Product{store.ProductCore}}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 ao conceder core, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUser_SuperAdminSetsProducts(t *testing.T) {
	f := setupRBACFixtures(t, store.RoleSuperAdmin)
	peer := createTestUserWithRole(t, f.app, "op-loja", "senha-loja-123", store.RoleAdmin)
	products := []store.Product{store.ProductMarketplace}
	rec := doJSON(t, f.router, http.MethodPatch, "/api/users/"+strconv.FormatUint(uint64(peer.ID), 10),
		updateUserRequest{Products: &products}, f.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var updated userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(updated.Products) != 1 || updated.Products[0] != store.ProductMarketplace {
		t.Fatalf("products não persistiu, obtido %v", updated.Products)
	}
}

func TestAuthMeReturnsProducts(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductXGroup, store.ProductXDriver})
	rec := doJSON(t, f.router, http.MethodGet, "/api/auth/me", nil, f.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var me userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !containsProduct(me.Products, store.ProductXGroup) || !containsProduct(me.Products, store.ProductXDriver) {
		t.Fatalf("/auth/me deveria devolver o escopo, obtido %v", me.Products)
	}
}

func containsProduct(list []store.Product, want store.Product) bool {
	for _, p := range list {
		if p == want {
			return true
		}
	}
	return false
}
