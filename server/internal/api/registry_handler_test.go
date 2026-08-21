package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func doRegistry(t *testing.T, router http.Handler, method, path, token, host string, extra map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Host = host
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestRegistryTokenAndAuthACL(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_, aliceTok, _ := seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)
	createTestUserWithRole(t, app, "eve", "senha-eve-okkk", store.RoleMember)
	eveTok := loginAndGetToken(t, app, router, "eve", "senha-eve-okkk")

	rec := doRegistry(t, router, http.MethodGet, "/api/registry/token?scope=repository:xcorp/lab:pull,push", aliceTok, "registry.corp.ihuull.com", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("token: %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Token == "" {
		t.Fatalf("token body: %s", rec.Body.String())
	}

	rec = doRegistry(t, router, http.MethodGet, "/api/registry/auth", body.Token, "registry.corp.ihuull.com", map[string]string{
		"X-Original-URI":    "/v2/xcorp/lab/manifests/latest",
		"X-Original-Method": "GET",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("pull: %d %s", rec.Code, rec.Body.String())
	}

	rec = doRegistry(t, router, http.MethodGet, "/api/registry/auth", body.Token, "registry.corp.ihuull.com", map[string]string{
		"X-Original-URI":    "/v2/xcorp/lab/manifests/latest",
		"X-Original-Method": "PUT",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("push developer: %d %s", rec.Code, rec.Body.String())
	}

	rec = doRegistry(t, router, http.MethodGet, "/api/registry/token?scope=repository:xcorp/lab:pull", eveTok, "registry.corp.ihuull.com", nil)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("eve token: %d %s", rec.Code, rec.Body.String())
	}

	rec = doRegistry(t, router, http.MethodGet, "/api/registry/token?scope=repository:xcorp/lab:pull", aliceTok, "xvpn.ihuull.com", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("host público: %d", rec.Code)
	}

	rec = doRegistry(t, router, http.MethodGet, "/api/registry/auth", "", "registry.corp.ihuull.com", map[string]string{
		"X-Original-URI": "/v2/xcorp/lab/manifests/latest",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem JWE: %d", rec.Code)
	}
}

func TestParseRegistryScopeAndPath(t *testing.T) {
	repo, acts := parseRegistryScope("repository:xcorp/hello-js:pull,push")
	if repo != "xcorp/hello-js" || acts != "pull,push" {
		t.Fatalf("scope: %q %q", repo, acts)
	}
	if parseRegistryV2Repo("/v2/xcorp/hello-js/manifests/latest") != "xcorp/hello-js" {
		t.Fatal("v2 path")
	}
	if parseRegistryMountFrom("/v2/xcorp/lab/blobs/uploads/?mount=sha256:abc&from=xcorp/secret") != "xcorp/secret" {
		t.Fatal("mount from")
	}
}

func TestRegistryScopedTokenCannotMintOtherRepo(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	adminTok, _, alice := seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "other", Name: "Other"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("other: %d %s", rec.Code, rec.Body.String())
	}
	pkgTok, err := app.Tokens.IssuePackages(alice.ID, alice.Username, alice.Role, "xcorp/lab")
	if err != nil {
		t.Fatal(err)
	}
	rec = doRegistry(t, router, http.MethodGet, "/api/registry/token?scope=repository:xcorp/other:pull", pkgTok, "registry.corp.ihuull.com", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("mint other: %d %s", rec.Code, rec.Body.String())
	}
	rec = doRegistry(t, router, http.MethodGet, "/api/registry/auth", pkgTok, "registry.corp.ihuull.com", map[string]string{
		"X-Original-URI":    "/v2/xcorp/lab/blobs/uploads/?mount=sha256:dead&from=xcorp/other",
		"X-Original-Method": "POST",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("mount other: %d %s", rec.Code, rec.Body.String())
	}
}
