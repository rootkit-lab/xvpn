package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestNetworksSeedAndCreateCustom(t *testing.T) {
	app, _ := withProvisioner(t, &fakeUserProvisioner{})
	router := NewRouter(app)
	admin := createTestUserWithRole(t, app, "netadmin", "senha-admin-123", store.RoleSuperAdmin)
	token := loginAndGetToken(t, app, router, admin.Username, "senha-admin-123")

	rec := doJSON(t, router, http.MethodGet, "/api/networks", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Items []overlayNetworkJSON `json:"items"`
		Rules []networkRuleJSON    `json:"rules"`
		Pool  string               `json:"pool"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.Pool != store.UsersPoolCIDR || len(listed.Items) != 2 || len(listed.Rules) != 3 {
		t.Fatalf("seed: %+v", listed)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/networks", createNetworkRequest{
		Slug: "lab", Name: "Lab", CorpAccess: true,
	}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created overlayNetworkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.CIDR != "10.66.81.0/24" || created.Kind != store.NetworkKindCustom {
		t.Fatalf("custom: %+v", created)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/networks", createNetworkRequest{
		Slug: "bad", CIDR: "10.10.1.0/24",
	}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("10.10 deveria 400, obtido %d", rec.Code)
	}
}

func TestNetworksMemberAndRule(t *testing.T) {
	app, _ := withProvisioner(t, &fakeUserProvisioner{})
	router := NewRouter(app)
	admin := createTestUserWithRole(t, app, "netadmin", "senha-admin-123", store.RoleSuperAdmin)
	member := createTestUserWithRole(t, app, "alice", "senha-alice-123", store.RoleMember)
	token := loginAndGetToken(t, app, router, admin.Username, "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/networks", createNetworkRequest{Slug: "lab"}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var net overlayNetworkJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &net)

	path := "/api/networks/" + strconv.FormatUint(uint64(net.ID), 10) + "/members"
	rec = doJSON(t, router, http.MethodPost, path, createMemberRequest{
		SubjectKind: store.NetworkSubjectUser, SubjectID: member.ID,
	}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("member: %d %s", rec.Code, rec.Body.String())
	}

	users, _ := store.NetworkByKind(app.Store.DB, store.NetworkKindUsers)
	rec = doJSON(t, router, http.MethodPost, "/api/networks/rules", createRuleRequest{
		Slug: "lab-in", SrcNetworkID: users.ID, DstNetworkID: net.ID, Action: "allow", Proto: "tcp", Ports: "22",
	}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("rule: %d %s", rec.Code, rec.Body.String())
	}
}

func TestDeviceEnrollUsesUsersNotInfra(t *testing.T) {
	app, _ := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	invite := createTestInvite(t, app, admin.ID)
	router := NewRouter(app)
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", enrollRequest{
		InviteToken: invite, PublicKey: testPublicKey, DeviceName: "note",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll: %d %s", rec.Code, rec.Body.String())
	}
	var resp enrollResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !strings.HasPrefix(resp.AssignedIP, "10.66.80.") {
		t.Fatalf("device deve nascer em users: %s", resp.AssignedIP)
	}
	var d store.Device
	if err := app.Store.DB.First(&d).Error; err != nil {
		t.Fatal(err)
	}
	users, _ := store.NetworkByKind(app.Store.DB, store.NetworkKindUsers)
	if d.NetworkID != users.ID {
		t.Fatalf("network_id=%d want %d", d.NetworkID, users.ID)
	}
}

func TestAdminWithoutCoreCannotCreateNetwork(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductMarketplace})
	rec := doJSON(t, f.router, http.MethodPost, "/api/networks", createNetworkRequest{Slug: "lab"}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403, obtido %d %s", rec.Code, rec.Body.String())
	}
}
