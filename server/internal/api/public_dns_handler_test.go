package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/dnsprovider"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type fakeCloudflare struct {
	zones   []dnsprovider.Zone
	records []dnsprovider.Record
	created dnsprovider.Zone
	rec     dnsprovider.Record
	nuked   []string
	err     error
}

func (f *fakeCloudflare) Accounts() ([]dnsprovider.Account, error) {
	return []dnsprovider.Account{{ID: "acc-1", Email: "ihuull"}}, nil
}
func (f *fakeCloudflare) ListZones() ([]dnsprovider.Zone, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.zones, nil
}
func (f *fakeCloudflare) CreateZone(name, _ string) (dnsprovider.Zone, error) {
	if f.err != nil {
		return dnsprovider.Zone{}, f.err
	}
	if f.created.ID == "" {
		return dnsprovider.Zone{
			ID: "zid-1", Name: name, Status: "pending",
			NameServers: []string{"ns1.stack.example", "ns2.stack.example"},
		}, nil
	}
	return f.created, nil
}
func (f *fakeCloudflare) ListRecords(string) ([]dnsprovider.Record, error) { return f.records, f.err }
func (f *fakeCloudflare) CreateRecord(_ string, rec dnsprovider.Record) (dnsprovider.Record, error) {
	if f.err != nil {
		return dnsprovider.Record{}, f.err
	}
	rec.ID = "rr-1"
	f.rec = rec
	return rec, nil
}
func (f *fakeCloudflare) UpdateRecord(string, string, dnsprovider.Record) (dnsprovider.Record, error) {
	return f.rec, f.err
}
func (f *fakeCloudflare) DeleteRecord(_, id string) error {
	f.nuked = append(f.nuked, id)
	return f.err
}

func TestPublicZoneShowsStackNameservers(t *testing.T) {
	app, _ := newTestApp(t)
	app.Cloudflare = &fakeCloudflare{}
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/dns/public/settings/accounts", upsertCFAccountRequest{
		Name: "Ops", Email: "dns@ihuull.com", Token: "token-cloudflare-16ok",
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("account: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "token-cloudflare-16ok") {
		t.Fatal("token não pode voltar")
	}

	rec = doJSON(t, router, http.MethodPost, "/api/dns/public/zones", createPublicZoneRequest{
		Name: "lab.ihuull.com",
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("zone: %d %s", rec.Code, rec.Body.String())
	}
	var z publicZoneJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &z); err != nil {
		t.Fatal(err)
	}
	if len(z.NameServers) != 2 || z.Name != "lab.ihuull.com" {
		t.Fatalf("NS do stack: %+v", z)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/dns/public/zones", createPublicZoneRequest{Name: "ldpops.appapisip.com"}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ldpops deveria 400, veio %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/dns/public/zones/"+strconv.FormatUint(uint64(z.ID), 10)+"/records", upsertPublicRecordRequest{
		Type: "A", Name: "www", Content: "206.189.224.72", IntranetIPv4: "10.66.66.1",
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("record: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/dns/public/zones/"+strconv.FormatUint(uint64(z.ID), 10)+"/records", upsertPublicRecordRequest{
		Type: "A", Name: "bad", Content: "10.66.66.9",
	}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("A RFC1918 deveria 400, veio %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/me/dns-suffixes", nil, tok)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "lab.ihuull.com") {
		t.Fatalf("suffixes: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/dns/recursor", nil, tok)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "server=/lab.ihuull.com/10.66.66.1") {
		t.Fatalf("recursor: %s", rec.Body.String())
	}
}

func TestAdminWithoutDNSCannotWritePublicZone(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductCore})
	rec := doJSON(t, f.router, http.MethodPost, "/api/dns/public/zones", createPublicZoneRequest{Name: "x.ihuull.com"}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin só-core não deveria criar zona, obtido %d", rec.Code)
	}
}
