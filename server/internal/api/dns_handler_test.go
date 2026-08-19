package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestDNSDefaultsAndCRUD(t *testing.T) {
	app, _ := withProvisioner(t, &fakeUserProvisioner{})
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodGet, "/api/dns", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /dns: %d %s", rec.Code, rec.Body.String())
	}
	var got dnsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Listen != "10.66.66.1:53" {
		t.Fatalf("listen: %q", got.Listen)
	}
	if len(got.Records) < 4 {
		t.Fatalf("esperava records oficiais, obtido %d", len(got.Records))
	}

	rec = doJSON(t, router, http.MethodPost, "/api/dns/records",
		upsertDNSRecordRequest{Hostname: "lab.corp.ihuull.com", IPv4: "10.66.66.9", Comment: "lab"}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST record: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var labID uint
	for _, r := range got.Records {
		if r.Hostname == "lab.corp.ihuull.com" {
			labID = r.ID
		}
	}
	if labID == 0 {
		t.Fatal("lab não apareceu")
	}

	rec = doJSON(t, router, http.MethodPost, "/api/dns/records",
		upsertDNSRecordRequest{Hostname: "evil.example.com", IPv4: "10.66.66.9"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("hostname público deveria 400, obtido %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/dns/records",
		upsertDNSRecordRequest{Hostname: "lab2.corp.ihuull.com", IPv4: "8.8.8.8"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("IPv4 público deveria 400, obtido %d", rec.Code)
	}

	var systemID uint
	for _, r := range got.Records {
		if r.System && r.Hostname == "xchat.corp.ihuull.com" {
			systemID = r.ID
		}
	}
	rec = doJSON(t, router, http.MethodDelete, "/api/dns/records/"+strconv.FormatUint(uint64(systemID), 10), nil, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("apagar system deveria 403, obtido %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/dns/records/"+strconv.FormatUint(uint64(labID), 10), nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE lab: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/dns/apply", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body.String())
	}
}

func TestDNSMemberForbidden(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "member", "senha-member-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "member", "senha-member-123")
	rec := doJSON(t, router, http.MethodGet, "/api/dns", nil, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member GET /dns: %d", rec.Code)
	}
}
