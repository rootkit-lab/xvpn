package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rootkit-lab/xvpn/server/internal/bitlaunch"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type fakeBitLaunch struct {
	list    []bitlaunch.Server
	created bitlaunch.Server
	opts    bitlaunch.CreateOpts
	nuked   []string
	account bitlaunch.Account
	tx      bitlaunch.Transaction
	topUp   bitlaunch.TopUpOpts
	err     error
}

func (f *fakeBitLaunch) List() ([]bitlaunch.Server, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func (f *fakeBitLaunch) Create(opts bitlaunch.CreateOpts) (bitlaunch.Server, error) {
	if f.err != nil {
		return bitlaunch.Server{}, f.err
	}
	f.opts = opts
	return f.created, nil
}

func (f *fakeBitLaunch) Destroy(id string) error {
	if f.err != nil {
		return f.err
	}
	f.nuked = append(f.nuked, id)
	return nil
}

func (f *fakeBitLaunch) Rebuild(id string, _ bitlaunch.RebuildOpts) error {
	if f.err != nil {
		return f.err
	}
	f.nuked = append(f.nuked, "rebuild:"+id)
	return nil
}

func (f *fakeBitLaunch) Account() (bitlaunch.Account, error) {
	if f.err != nil {
		return bitlaunch.Account{}, f.err
	}
	if f.account.Email == "" && f.account.Balance == 0 {
		return bitlaunch.Account{Email: "fake@bitlaunch.local", Balance: 30000, Used: 1, Limit: 5, CostPerHr: 21}, nil
	}
	return f.account, nil
}

func (f *fakeBitLaunch) CreateTransaction(opts bitlaunch.TopUpOpts) (bitlaunch.Transaction, error) {
	if f.err != nil {
		return bitlaunch.Transaction{}, f.err
	}
	f.topUp = opts
	if f.tx.ID == "" {
		return bitlaunch.Transaction{
			ID: "tx-1", Address: "bc1qtestxxxxxxxx", CryptoSymbol: opts.CryptoSymbol,
			AmountUSD: opts.AmountUSD, AmountCrypto: "0.001", Status: "Pending",
			StatusURL: "https://pay.bitlaunch.io/invoice/tx-1",
		}, nil
	}
	return f.tx, nil
}

func TestImportCreatesControlPlaneWithoutBitLaunch(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/servers/import", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Items     []meshServerResponse `json:"items"`
		BitLaunch bool                 `json:"bitlaunch"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.BitLaunch || len(listed.Items) != 1 {
		t.Fatalf("esperado só o node local: %+v", listed)
	}
	got := listed.Items[0]
	if got.Role != store.ServerRoleControl || got.IPv4 != controlPlaneIPv4 || got.WgIP != controlPlaneWgIP {
		t.Fatalf("control: %+v", got)
	}
	var recDNS store.DNSRecord
	if err := app.Store.DB.Where("hostname = ?", "control.corp.ihuull.com").First(&recDNS).Error; err != nil {
		t.Fatalf("A corp: %v", err)
	}
	if recDNS.IPv4 != controlPlaneWgIP {
		t.Fatalf("A deveria apontar para wg0, veio %s", recDNS.IPv4)
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/servers/"+strconv.FormatUint(uint64(got.ID), 10), nil, tok)
	if rec.Code != http.StatusConflict {
		t.Fatalf("destroy control deveria 409, veio %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateMeshServerAndEnroll(t *testing.T) {
	app, wg := newTestApp(t)
	fake := &fakeBitLaunch{created: bitlaunch.Server{
		ID: "bl-mesh", Name: "lab-a", IPv4: "203.0.113.40", Region: "ams", Size: "1gb", Status: "launching",
	}}
	app.BitLaunch = fake
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/servers", createMeshServerRequest{
		Name: "Lab A", Hostname: "laba", HostID: 4, HostImageID: "img", SizeID: "sz", RegionID: "ams",
		Labels: []string{"edge"}, Role: store.ServerRoleMesh,
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created meshServerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.EnrollToken == "" || created.Hostname != "laba" || fake.opts.InitScript == "" {
		t.Fatalf("create resp: %+v script=%q", created, fake.opts.InitScript)
	}
	if !strings.Contains(fake.opts.InitScript, "wg genkey") || strings.Contains(fake.opts.InitScript, "PrivateKey = "+created.EnrollToken) {
		t.Fatalf("cloud-init deveria gerar chave no host: %s", fake.opts.InitScript)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/servers/enroll", meshEnrollRequest{
		EnrollToken: created.EnrollToken, PublicKey: testPublicKey, Hostname: "laba",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll: %d %s", rec.Code, rec.Body.String())
	}
	var enrolled struct {
		AssignedIP string `json:"assigned_ip"`
		Hostname   string `json:"hostname"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &enrolled); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enrolled.AssignedIP, "10.66.66.") || strings.HasPrefix(enrolled.AssignedIP, "10.10.") || strings.HasPrefix(enrolled.AssignedIP, "10.136.") {
		t.Fatalf("IP fora da malha: %s", enrolled.AssignedIP)
	}
	if enrolled.Hostname != "laba.corp.ihuull.com" {
		t.Fatalf("hostname: %s", enrolled.Hostname)
	}
	if _, ok := wg.peers[testPublicKey]; !ok {
		t.Fatal("peer não entrou no wg")
	}
	var recDNS store.DNSRecord
	if err := app.Store.DB.Where("hostname = ?", "laba.corp.ihuull.com").First(&recDNS).Error; err != nil {
		t.Fatalf("A corp do mesh: %v", err)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/servers/"+strconv.FormatUint(uint64(created.ID), 10), nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	var after meshServerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.EnrollToken != "" || after.WgIP == "" || after.Status != "ok" {
		t.Fatalf("após enroll: %+v", after)
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/servers/"+strconv.FormatUint(uint64(created.ID), 10), nil, tok)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("destroy: %d %s", rec.Code, rec.Body.String())
	}
	if len(fake.nuked) != 1 || fake.nuked[0] != "bl-mesh" {
		t.Fatalf("bitlaunch destroy: %v", fake.nuked)
	}
	if _, ok := wg.peers[testPublicKey]; ok {
		t.Fatal("peer deveria ter saído")
	}
}

func TestCreateMeshServerStoresChosenAccount(t *testing.T) {
	app, _ := newTestApp(t)
	fake := &fakeBitLaunch{created: bitlaunch.Server{ID: "bl-acc", Status: "launching"}}
	app.BitLaunch = fake
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	acc := store.BitLaunchAccount{Name: "Trabalho", Email: "ops@ihuull.com", Token: "token-conta-escolhida"}
	if err := app.Store.DB.Create(&acc).Error; err != nil {
		t.Fatal(err)
	}
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	rec := doJSON(t, router, http.MethodPost, "/api/servers", createMeshServerRequest{
		Hostname: "labc", HostID: 4, HostImageID: "img", SizeID: "sz", RegionID: "ams",
		AccountID: acc.ID,
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created meshServerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.AccountID == nil || *created.AccountID != acc.ID {
		t.Fatalf("servidor deveria gravar a conta: %+v", created)
	}
}

func TestCreateMeshServerWithoutTokenIsUnavailable(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	rec := doJSON(t, router, http.MethodPost, "/api/servers", createMeshServerRequest{
		Hostname: "laba", HostID: 4, HostImageID: "img", SizeID: "sz", RegionID: "ams",
	}, tok)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("sem token deveria 503, veio %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithoutComputeScopeCannotImport(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductCore})
	rec := doJSON(t, f.router, http.MethodPost, "/api/servers/import", nil, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin só-core não deveria importar, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithComputeScopeCanImport(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductCompute})
	rec := doJSON(t, f.router, http.MethodPost, "/api/servers/import", nil, f.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin compute deveria importar, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestServerGroupAndAccess(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	member := createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/server-groups", createServerGroupRequest{Name: "edge"}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("group: %d %s", rec.Code, rec.Body.String())
	}
	var g serverGroupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &g); err != nil {
		t.Fatal(err)
	}

	rec = doJSON(t, router, http.MethodPut, "/api/server-groups/"+strconv.FormatUint(uint64(g.ID), 10)+"/access",
		setServerAccessRequest{UserIDs: []uint{member.ID}}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("group access: %d %s", rec.Code, rec.Body.String())
	}

	exp := time.Now().Add(time.Hour)
	row := store.MeshServer{
		BitLaunchID: "local-lab", Name: "lab", Hostname: "labx", Role: store.ServerRoleMesh,
		Status: "ok", EnrollToken: "tok", EnrollExpiresAt: &exp,
	}
	if err := app.Store.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodPatch, "/api/servers/"+strconv.FormatUint(uint64(row.ID), 10),
		updateMeshServerRequest{Labels: &[]string{"runner"}, GroupID: &g.ID}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPut, "/api/servers/"+strconv.FormatUint(uint64(row.ID), 10)+"/access",
		setServerAccessRequest{UserIDs: []uint{member.ID}}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("server access: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/servers/"+strconv.FormatUint(uint64(row.ID), 10), nil, tok)
	var got meshServerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.GroupID == nil || *got.GroupID != g.ID || len(got.AccessUserIDs) != 1 {
		t.Fatalf("get: %+v", got)
	}
}

func TestCreateRejectsExistingCustomDNS(t *testing.T) {
	app, _ := newTestApp(t)
	app.BitLaunch = &fakeBitLaunch{created: bitlaunch.Server{ID: "bl-dns"}}
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	if err := app.Store.DB.Create(&store.DNSRecord{
		Hostname: "labd.corp.ihuull.com", IPv4: "10.66.66.9", Enabled: true, Comment: "lab do dns",
	}).Error; err != nil {
		t.Fatal(err)
	}
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	rec := doJSON(t, router, http.MethodPost, "/api/servers", createMeshServerRequest{
		Hostname: "labd", HostID: 4, HostImageID: "img", SizeID: "sz", RegionID: "ams",
	}, tok)
	if rec.Code != http.StatusConflict {
		t.Fatalf("A customizado deveria 409, veio %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRejectsReservedIntranetHostname(t *testing.T) {
	app, _ := newTestApp(t)
	app.BitLaunch = &fakeBitLaunch{created: bitlaunch.Server{ID: "bl-x"}}
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	rec := doJSON(t, router, http.MethodPost, "/api/servers", createMeshServerRequest{
		Hostname: "xadmin", HostID: 4, HostImageID: "img", SizeID: "sz", RegionID: "ams",
	}, tok)
	if rec.Code != http.StatusConflict {
		t.Fatalf("xadmin deveria 409, veio %d %s", rec.Code, rec.Body.String())
	}
}

func TestRebuildRevokesOldPeer(t *testing.T) {
	app, wg := newTestApp(t)
	fake := &fakeBitLaunch{created: bitlaunch.Server{ID: "bl-r", Status: "ok"}}
	app.BitLaunch = fake
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/servers", createMeshServerRequest{
		Hostname: "labr", HostID: 4, HostImageID: "img", SizeID: "sz", RegionID: "ams",
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created meshServerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodPost, "/api/servers/enroll", meshEnrollRequest{
		EnrollToken: created.EnrollToken, PublicKey: testPublicKey, Hostname: "labr",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := wg.peers[testPublicKey]; !ok {
		t.Fatal("peer deveria existir antes do rebuild")
	}

	rec = doJSON(t, router, http.MethodPost, "/api/servers/"+strconv.FormatUint(uint64(created.ID), 10)+"/rebuild",
		map[string]string{"host_image_id": "img2"}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("rebuild: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := wg.peers[testPublicKey]; ok {
		t.Fatal("rebuild deveria revogar o peer antigo")
	}
	var after meshServerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.EnrollToken == "" || after.WgIP != "" {
		t.Fatalf("rebuild deveria devolver token novo e zerar wg: %+v", after)
	}

	var sys store.DNSRecord
	if err := app.Store.DB.Where("hostname = ?", "xadmin.corp.ihuull.com").First(&sys).Error; err != nil {
		t.Fatal(err)
	}
	if !sys.System || sys.IPv4 != "10.66.66.1" {
		t.Fatalf("A de sistema não deveria mudar: %+v", sys)
	}
}

func TestImportMarksExternalHostsAndBlocksMutations(t *testing.T) {
	app, _ := newTestApp(t)
	app.BitLaunch = &fakeBitLaunch{list: []bitlaunch.Server{
		{ID: "bl-cripto", Name: "server-cripto-prod", IPv4: "203.0.113.80", Status: "ok"},
		{ID: "bl-ip", Name: "outro-nome", IPv4: store.ExternalIPv4Cripto, Status: "ok"},
	}}
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/servers/import", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Items []meshServerResponse `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	var cripto, byIP *meshServerResponse
	for i := range listed.Items {
		it := &listed.Items[i]
		if it.Name == "server-cripto-prod" {
			cripto = it
		}
		if it.IPv4 == store.ExternalIPv4Cripto {
			byIP = it
		}
	}
	if cripto == nil || cripto.Role != store.ServerRoleExternal || !cripto.Protected {
		t.Fatalf("cripto-prod: %+v", cripto)
	}
	if byIP == nil || byIP.Role != store.ServerRoleExternal || !byIP.Protected {
		t.Fatalf("65.38: %+v", byIP)
	}
	var corp store.DNSRecord
	if err := app.Store.DB.Where("hostname = ?", "server-cripto-prod.corp.ihuull.com").First(&corp).Error; err == nil {
		t.Fatal("host externo não deveria ganhar A corp")
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/servers/"+strconv.FormatUint(uint64(cripto.ID), 10), nil, tok)
	if rec.Code != http.StatusConflict {
		t.Fatalf("destroy externo deveria 409, veio %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, "/api/servers/"+strconv.FormatUint(uint64(byIP.ID), 10)+"/rebuild",
		map[string]string{"host_image_id": "img"}, tok)
	if rec.Code != http.StatusConflict {
		t.Fatalf("rebuild externo deveria 409, veio %d %s", rec.Code, rec.Body.String())
	}

	notes := "caixa só observa; app própria"
	rec = doJSON(t, router, http.MethodPatch, "/api/servers/"+strconv.FormatUint(uint64(cripto.ID), 10),
		updateMeshServerRequest{Notes: &notes}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("notes: %d %s", rec.Code, rec.Body.String())
	}
	var after meshServerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Notes != notes {
		t.Fatalf("notes: %+v", after)
	}
}
