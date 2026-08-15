package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestHandleJoinWaitlist_Success(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodPost, "/api/waitlist", joinWaitlistRequest{
		Name:  "Alice",
		Email: "alice@example.com",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}

	var resp waitlistResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if resp.Status != waitlistStatusPending {
		t.Fatalf("esperava status pending, obtido %q", resp.Status)
	}
}

func TestHandleJoinWaitlist_InvalidEmailRejected(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodPost, "/api/waitlist", joinWaitlistRequest{
		Name:  "Bob",
		Email: "não-é-um-email",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para e-mail inválido, obtido %d", rec.Code)
	}
}

func TestHandleJoinWaitlist_MissingNameRejected(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodPost, "/api/waitlist", joinWaitlistRequest{
		Email: "carol@example.com",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para nome ausente, obtido %d", rec.Code)
	}
}

func TestHandleJoinWaitlist_DuplicateEmailDoesNotCreateSecondEntry(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	body := joinWaitlistRequest{Name: "Dave", Email: "dave@example.com"}
	rec := doJSON(t, router, http.MethodPost, "/api/waitlist", body, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("primeiro cadastro deveria funcionar, obtido %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/waitlist", body, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("segundo cadastro (mesmo e-mail) deveria devolver 200, obtido %d", rec.Code)
	}

	admin := createTestUser(t, app, "admin", "senha-admin-123")
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	_ = admin
	rec = doJSON(t, router, http.MethodGet, "/api/waitlist", nil, token)
	entries := pageItems[waitlistResponse](t, decodePage[waitlistResponse](t, rec.Body.Bytes()))
	if len(entries) != 1 {
		t.Fatalf("esperava 1 entrada (sem duplicata), obtido %d", len(entries))
	}
}

func TestHandleJoinWaitlist_RequiresAuth(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodGet, "/api/waitlist", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 sem token, obtido %d", rec.Code)
	}
}

func TestHandleJoinWaitlist_RateLimited(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	var lastCode int
	for i := 0; i < 6; i++ {
		rec := doJSON(t, router, http.MethodPost, "/api/waitlist", joinWaitlistRequest{
			Name:  "Spammer",
			Email: "spam" + strconv.Itoa(i) + "@example.com",
		}, "")
		lastCode = rec.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("esperava a 6ª tentativa do mesmo IP em 429, obtido %d", lastCode)
	}
}

func TestHandleApproveAndRejectWaitlist(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUser(t, app, "admin", "senha-admin-123")
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/waitlist", joinWaitlistRequest{
		Name:  "Erin",
		Email: "erin@example.com",
	}, "")
	var created waitlistResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	id := strconv.FormatUint(uint64(created.ID), 10)
	rec = doJSON(t, router, http.MethodPost, "/api/waitlist/"+id+"/approve", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 ao aprovar, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var approved waitlistResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &approved); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if approved.Status != waitlistStatusApproved {
		t.Fatalf("esperava status approved, obtido %q", approved.Status)
	}
	if approved.ReviewedAt == nil {
		t.Fatalf("esperava reviewed_at preenchido após aprovar")
	}

	rec = doJSON(t, router, http.MethodPost, "/api/waitlist/"+id+"/reject", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 ao rejeitar, obtido %d", rec.Code)
	}
}

// TestHandleProvisionWaitlist_CreatesUserAndInvite cobre o fluxo de
// "aprovar waitlist e provisionar" (Fase 10, PLAN.md §6.7): uma chamada só
// cria o User de verdade, gera o InviteToken de enrollment e marca o
// cadastro como aprovado.
func TestHandleProvisionWaitlist_CreatesUserAndInvite(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUser(t, app, "admin", "senha-admin-123")
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/waitlist", joinWaitlistRequest{Name: "Erin", Email: "erin@example.com"}, "")
	var entry waitlistResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &entry); err != nil {
		t.Fatalf("erro decodificando cadastro: %v", err)
	}

	id := strconv.FormatUint(uint64(entry.ID), 10)
	rec = doJSON(t, router, http.MethodPost, "/api/waitlist/"+id+"/provision",
		provisionWaitlistRequest{Username: "erin", Role: store.RoleMember}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var resp provisionWaitlistResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if resp.User.Username != "erin" || resp.User.Role != store.RoleMember {
		t.Fatalf("usuário criado inesperado: %+v", resp.User)
	}
	if resp.Password == "" {
		t.Fatalf("esperava uma senha gerada na resposta")
	}
	if resp.Invite.Token == "" {
		t.Fatalf("esperava um invite token na resposta")
	}

	// O cadastro precisa ter sido marcado como aprovado.
	rec = doJSON(t, router, http.MethodGet, "/api/waitlist", nil, token)
	entries := pageItems[waitlistResponse](t, decodePage[waitlistResponse](t, rec.Body.Bytes()))
	if len(entries) != 1 || entries[0].Status != waitlistStatusApproved {
		t.Fatalf("esperava cadastro aprovado após provisionar, obtido %+v", entries)
	}

	// A senha gerada precisa funcionar de fato.
	loginRec := doJSON(t, router, http.MethodPost, "/api/auth/login", loginRequest{Username: "erin", Password: resp.Password}, "")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("esperava conseguir logar com a senha gerada, obtido %d: %s", loginRec.Code, loginRec.Body.String())
	}

	// O invite gerado precisa ser utilizável no enrollment.
	enrollRec := doJSON(t, router, http.MethodPost, "/api/devices/enroll",
		enrollRequest{InviteToken: resp.Invite.Token, PublicKey: testPublicKey, DeviceName: "notebook-da-erin"}, "")
	if enrollRec.Code != http.StatusCreated {
		t.Fatalf("esperava conseguir usar o invite gerado no enrollment, obtido %d: %s", enrollRec.Code, enrollRec.Body.String())
	}
}

// TestHandleProvisionWaitlist_DefaultsToMemberRole confirma que omitir o
// papel no corpo produz o mais restritivo, igual a handleCreateUser.
func TestHandleProvisionWaitlist_DefaultsToMemberRole(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUser(t, app, "admin", "senha-admin-123")
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/waitlist", joinWaitlistRequest{Name: "Frank", Email: "frank@example.com"}, "")
	var entry waitlistResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &entry)

	id := strconv.FormatUint(uint64(entry.ID), 10)
	rec = doJSON(t, router, http.MethodPost, "/api/waitlist/"+id+"/provision", provisionWaitlistRequest{Username: "frank"}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var resp provisionWaitlistResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.User.Role != store.RoleMember {
		t.Fatalf("esperava role padrão member, obtido %q", resp.User.Role)
	}
}

// TestHandleProvisionWaitlist_CannotEscalatePrivilege: mesma regra de
// handleCreateUser — um admin não provisiona um super_admin.
func TestHandleProvisionWaitlist_CannotEscalatePrivilege(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "gerente", "senha-gerente-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "gerente", "senha-gerente-123")

	rec := doJSON(t, router, http.MethodPost, "/api/waitlist", joinWaitlistRequest{Name: "Gina", Email: "gina@example.com"}, "")
	var entry waitlistResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &entry)

	id := strconv.FormatUint(uint64(entry.ID), 10)
	rec = doJSON(t, router, http.MethodPost, "/api/waitlist/"+id+"/provision",
		provisionWaitlistRequest{Username: "gina", Role: store.RoleSuperAdmin}, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 ao tentar provisionar super_admin sendo admin, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleProvisionWaitlist_NotFound(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUser(t, app, "admin", "senha-admin-123")
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/waitlist/999/provision", provisionWaitlistRequest{Username: "ninguem"}, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404, obtido %d", rec.Code)
	}
}

func TestHandleReviewWaitlist_NotFound(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUser(t, app, "admin", "senha-admin-123")
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/waitlist/999/approve", nil, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404, obtido %d", rec.Code)
	}
}
