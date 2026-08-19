package api

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// Promoção admin → super_admin sem re-login: o JWT ainda carrega "admin",
// mas refreshCallerFromDB deve ler o DB e permitir CanManage do próprio
// usuário agora super_admin (antes falhava com 403 no file-access).
func TestRefreshCallerFromDB_PromotedRoleSeesDB(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	admin := createTestUserWithRole(t, app, "promoted", "senha-admin-123", store.RoleAdmin)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "promoted", "senha-admin-123")

	if err := app.Store.DB.Model(&admin).Update("role", store.RoleSuperAdmin).Error; err != nil {
		t.Fatalf("promovendo no DB: %v", err)
	}

	path := "/api/users/" + strconv.FormatUint(uint64(admin.ID), 10) + "/file-access"
	rec := doJSON(t, router, http.MethodPut, path, fileAccessRequest{
		SFTPEnabled:  false,
		SambaEnabled: true,
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("após promoção no DB, esperado 200 com JWT antigo; obtido %d: %s", rec.Code, rec.Body.String())
	}
}

// Rebaixamento: JWT de super_admin não pode continuar gerenciando outro
// super_admin depois que o DB já rebaixou o chamador para admin.
func TestRefreshCallerFromDB_DemotedRoleSeesDB(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	boss := createTestUserWithRole(t, app, "boss", "senha-admin-123", store.RoleSuperAdmin)
	target := createTestUserWithRole(t, app, "peer", "senha-peer-123", store.RoleSuperAdmin)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "boss", "senha-admin-123")

	if err := app.Store.DB.Model(&boss).Update("role", store.RoleAdmin).Error; err != nil {
		t.Fatalf("rebaixando no DB: %v", err)
	}

	path := "/api/users/" + strconv.FormatUint(uint64(target.ID), 10) + "/file-access"
	rec := doJSON(t, router, http.MethodPut, path, fileAccessRequest{
		SFTPEnabled: false,
	}, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("após rebaixamento no DB, esperado 403; obtido %d: %s", rec.Code, rec.Body.String())
	}
}
