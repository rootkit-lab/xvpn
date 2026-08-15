package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestHandleUpdateConfig_TTLs(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	invite := 42
	jwt := 120
	rec := doJSON(t, router, http.MethodPatch, "/api/config", updateConfigRequest{
		InviteTokenTTLMinutes: &invite,
		JWTTokenTTLMinutes:    &jwt,
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var resp configResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.InviteTokenTTLMinutes != 42 || resp.JWTTokenTTLMinutes != 120 {
		t.Fatalf("resposta=%+v", resp)
	}
	if app.Config.InviteTokenTTLMinutes != 42 || app.Config.JWTTokenTTLMinutes != 120 {
		t.Fatalf("config em memória não atualizou: %+v", app.Config)
	}

	// Sobrevive a ApplyPanelSettingsOverrides (simula reboot parcial).
	app.Config.InviteTokenTTLMinutes = 15
	app.Config.JWTTokenTTLMinutes = 720
	if err := app.ApplyPanelSettingsOverrides(); err != nil {
		t.Fatal(err)
	}
	if app.Config.InviteTokenTTLMinutes != 42 || app.Config.JWTTokenTTLMinutes != 120 {
		t.Fatalf("overrides do DB não aplicados: invite=%d jwt=%d",
			app.Config.InviteTokenTTLMinutes, app.Config.JWTTokenTTLMinutes)
	}
}

func TestHandleUpdateConfig_MemberForbidden(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "member1", "senha-membro-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "member1", "senha-membro-123")
	invite := 10
	rec := doJSON(t, router, http.MethodPatch, "/api/config", updateConfigRequest{
		InviteTokenTTLMinutes: &invite,
	}, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403, obtido %d", rec.Code)
	}
}
