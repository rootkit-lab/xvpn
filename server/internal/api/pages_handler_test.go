package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestPagesPublishAndStaticHost(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	app.Config.PagesDir = t.TempDir()
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if err := forge.InitBare(app.Config.GitDir, "xcorp/lab"); err != nil {
		t.Fatal(err)
	}
	if _, err := forge.CommitFiles(app.Config.GitDir, "xcorp/lab", forge.CommitFilesOpts{
		Files:   []forge.FileContent{{Path: "docs/index.html", Content: "<!doctype html><p>docs</p>"}},
		Message: "docs",
	}); err != nil {
		t.Fatal(err)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/pages?source=docs", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish docs: %d %s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/xcorp/lab/index.html", nil)
	req.Host = "pages.corp.ihuull.com"
	got := httptest.NewRecorder()
	router.ServeHTTP(got, req)
	if got.Code != http.StatusOK || !bytes.Contains(got.Body.Bytes(), []byte("docs")) {
		t.Fatalf("static: %d %s", got.Code, got.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/xcorp/lab/index.html", nil)
	req.Host = "xvpn.ihuull.com"
	got = httptest.NewRecorder()
	router.ServeHTTP(got, req)
	if got.Code == http.StatusOK && bytes.Contains(got.Body.Bytes(), []byte("docs")) {
		t.Fatal("host público não deve servir pages")
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("<!doctype html><p>ci</p>")
	_ = tw.WriteHeader(&tar.Header{Name: "index.html", Mode: 0o644, Size: int64(len(body))})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()

	var mp bytes.Buffer
	w := multipart.NewWriter(&mp)
	fw, err := w.CreateFormFile("file", "pages.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	req = httptest.NewRequest(http.MethodPost, "/api/projects/xcorp/lab/pages", &mp)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+adminTok)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	index := filepath.Join(app.Config.PagesDir, "xcorp", "lab", "index.html")
	raw, err := os.ReadFile(index)
	if err != nil || !bytes.Contains(raw, []byte("ci")) {
		t.Fatalf("disco: %v %s", err, raw)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/pages", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var st struct {
		Published bool `json:"published"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil || !st.Published {
		t.Fatalf("status body: %s", rec.Body.String())
	}

	pkgTok, err := app.Tokens.IssuePackages(1, "admin", store.RoleSuperAdmin, "xcorp/other")
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/projects/xcorp/lab/pages?source=docs", nil)
	req.Header.Set("Authorization", "Bearer "+pkgTok)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("token outro repo: %d", rec.Code)
	}
}
