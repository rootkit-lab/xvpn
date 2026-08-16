package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func doJSONHost(t *testing.T, router http.Handler, method, path string, body any, token, host string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestDriverListRequiresCorpHost(t *testing.T) {
	app, _ := newTestApp(t)
	shared := t.TempDir()
	app.Config.DriverSharedDir = shared
	app.Config.DriverHomeRoot = t.TempDir()
	if err := os.WriteFile(filepath.Join(shared, "readme.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodGet, "/api/driver/ls?root=shared", nil, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("sem Host corp deveria 404, veio %d %s", rec.Code, rec.Body.String())
	}

	rec2 := doJSONHost(t, router, http.MethodGet, "/api/driver/ls?root=shared", nil, token, "xdriver.corp.ihuull.com")
	if rec2.Code != http.StatusOK {
		t.Fatalf("com Host corp: %d %s", rec2.Code, rec2.Body.String())
	}
	var body struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].Name != "readme.txt" {
		t.Fatalf("lista: %+v", body)
	}
}

func TestDriverRejectsEmptyDelete(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.DriverSharedDir = t.TempDir()
	app.Config.DriverHomeRoot = t.TempDir()
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	rec := doJSONHost(t, router, http.MethodDelete, "/api/driver/rm", map[string]string{"root": "shared", "path": ""}, token, "xdriver.corp.ihuull.com")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("delete vazio deveria 400, veio %d", rec.Code)
	}
}
