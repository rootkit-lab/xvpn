package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func seedLabWithAlice(t *testing.T, app *App, router http.Handler, aliceRole store.ProjectRole) (adminTok, aliceTok string, alice store.User) {
	t.Helper()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	alice = createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	adminTok = loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	aliceTok = loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/members", setProjectMembersRequest{
		Members: []projectMemberIn{
			{UserID: created.Members[0].UserID, Role: store.ProjectRoleOwner},
			{UserID: alice.ID, Role: aliceRole},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}
	return adminTok, aliceTok, alice
}

func uploadProjectPackage(t *testing.T, router http.Handler, token, slug, name, version, kind, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("name", name)
	_ = w.WriteField("version", version)
	if kind != "" {
		_ = w.WriteField("kind", kind)
	}
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("form: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+slug+"/packages", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestForgePackages_UploadListDownloadACL(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_, aliceTok, _ := seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)
	createTestUserWithRole(t, app, "eve", "senha-eve-okkk", store.RoleMember)
	eveTok := loginAndGetToken(t, app, router, "eve", "senha-eve-okkk")

	rec := uploadProjectPackage(t, router, aliceTok, "lab", "sdk", "1.0.0", "generic", "sdk-1.0.0.zip", []byte("hello-pkg"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/packages", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Items      []forgePackageJSON `json:"items"`
		CanPublish bool               `json:"can_publish"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if !listed.CanPublish || len(listed.Items) != 1 || listed.Items[0].Name != "sdk" || listed.Items[0].Latest != "1.0.0" {
		t.Fatalf("list: %+v", listed)
	}
	if len(listed.Items[0].Versions) != 1 {
		t.Fatalf("versions: %+v", listed.Items[0].Versions)
	}
	vid := listed.Items[0].Versions[0].ID

	req := httptest.NewRequest(http.MethodGet, "/api/projects/lab/packages/"+strconv.FormatUint(uint64(vid), 10)+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+aliceTok)
	dl := httptest.NewRecorder()
	router.ServeHTTP(dl, req)
	if dl.Code != http.StatusOK {
		t.Fatalf("download: %d %s", dl.Code, dl.Body.String())
	}
	if dl.Body.String() != "hello-pkg" {
		t.Fatalf("download body: %q", dl.Body.String())
	}

	rec = uploadProjectPackage(t, router, aliceTok, "lab", "sdk", "1.0.0", "generic", "sdk-1.0.0.zip", []byte("other"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("reupload diferente deveria 409, veio %d: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/xgit/packages", nil, eveTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("home eve: %d %s", rec.Code, rec.Body.String())
	}
	var home struct {
		Items []forgePackageJSON `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &home); err != nil {
		t.Fatal(err)
	}
	if len(home.Items) != 0 {
		t.Fatalf("eve não deveria ver packages do lab: %+v", home.Items)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/packages", nil, eveTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("eve list projeto deveria 404, veio %d: %s", rec.Code, rec.Body.String())
	}

	rec = uploadProjectPackage(t, router, eveTok, "lab", "evil", "1.0.0", "generic", "x.bin", []byte("x"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("eve upload deveria 404, veio %d: %s", rec.Code, rec.Body.String())
	}
}

func TestForgePackages_ReporterCannotPublish(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_, aliceTok, _ := seedLabWithAlice(t, app, router, store.ProjectRoleReporter)
	rec := uploadProjectPackage(t, router, aliceTok, "lab", "sdk", "1.0.0", "generic", "sdk.zip", []byte("x"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reporter deveria 403, veio %d: %s", rec.Code, rec.Body.String())
	}
}

func TestForgePackages_NpmPublishAndPackument(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_, aliceTok, _ := seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)
	tarball := []byte("tarball-bytes")
	body := map[string]any{
		"name": "hello",
		"versions": map[string]any{
			"1.2.3": map[string]any{"name": "hello", "version": "1.2.3", "description": "oi"},
		},
		"dist-tags": map[string]string{"latest": "1.2.3"},
		"_attachments": map[string]any{
			"hello-1.2.3.tgz": map[string]any{
				"content_type": "application/octet-stream",
				"data":         base64.StdEncoding.EncodeToString(tarball),
			},
		},
	}
	rec := doJSON(t, router, http.MethodPut, "/api/packages/lab/npm/hello", body, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("npm publish: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/packages/lab/npm/hello", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("packument: %d %s", rec.Code, rec.Body.String())
	}
	var doc struct {
		Name     string            `json:"name"`
		Versions map[string]any    `json:"versions"`
		DistTags map[string]string `json:"dist-tags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Name != "hello" || doc.DistTags["latest"] != "1.2.3" {
		t.Fatalf("packument: %+v", doc)
	}
	ver, ok := doc.Versions["1.2.3"].(map[string]any)
	if !ok {
		t.Fatalf("version: %#v", doc.Versions["1.2.3"])
	}
	dist, _ := ver["dist"].(map[string]any)
	tarballURL, _ := dist["tarball"].(string)
	if tarballURL == "" || dist["integrity"] == "" || dist["shasum"] == "" {
		t.Fatalf("dist: %+v", dist)
	}

	scoped := map[string]any{
		"name": "@ihuull/hello",
		"versions": map[string]any{
			"0.1.0": map[string]any{"name": "@ihuull/hello", "version": "0.1.0"},
		},
		"_attachments": map[string]any{
			"ihuull-hello-0.1.0.tgz": map[string]any{
				"data": base64.StdEncoding.EncodeToString([]byte("scoped")),
			},
		},
	}
	rec = doJSON(t, router, http.MethodPut, "/api/packages/lab/npm/@ihuull/hello", scoped, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("scoped publish: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/packages/lab/npm/@ihuull/hello", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("scoped packument: %d %s", rec.Code, rec.Body.String())
	}
}
