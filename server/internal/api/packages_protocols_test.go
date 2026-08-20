package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func doRawPkg(t *testing.T, router http.Handler, method, path, token string, body []byte, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Host = "xgit.corp.ihuull.com"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestForgePackages_MavenPutGetAndAuth(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_, aliceTok, _ := seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)

	jar := []byte("hello-mvn-jar")
	rec := doRawPkg(t, router, http.MethodPut,
		"/api/packages/xcorp/lab/maven/com/ihuull/hello-mvn/0.1.0/hello-mvn-0.1.0.jar",
		aliceTok, jar, "application/octet-stream")
	if rec.Code != http.StatusCreated {
		t.Fatalf("maven put: %d %s", rec.Code, rec.Body.String())
	}

	rec = doRawPkg(t, router, http.MethodPut,
		"/api/packages/xcorp/lab/maven/com/ihuull/hello-mvn/0.1.0/hello-mvn-0.1.0.pom",
		aliceTok, []byte("<project/>"), "application/xml")
	if rec.Code != http.StatusCreated {
		t.Fatalf("maven pom: %d %s", rec.Code, rec.Body.String())
	}

	rec = doRawPkg(t, router, http.MethodPut,
		"/api/packages/xcorp/lab/maven/com/ihuull/hello-mvn/0.1.0/hello-mvn-0.1.0.jar.sha1",
		aliceTok, []byte("deadbeef"), "text/plain")
	if rec.Code != http.StatusCreated {
		t.Fatalf("maven sha1 put: %d %s", rec.Code, rec.Body.String())
	}

	rec = doRawPkg(t, router, http.MethodGet,
		"/api/packages/xcorp/lab/maven/com/ihuull/hello-mvn/0.1.0/hello-mvn-0.1.0.jar",
		aliceTok, nil, "")
	if rec.Code != http.StatusOK || rec.Body.String() != "hello-mvn-jar" {
		t.Fatalf("maven get: %d %q", rec.Code, rec.Body.String())
	}

	rec = doRawPkg(t, router, http.MethodGet,
		"/api/packages/xcorp/lab/maven/com/ihuull/hello-mvn/maven-metadata.xml",
		aliceTok, nil, "")
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("<version>0.1.0</version>")) {
		t.Fatalf("metadata: %d %s", rec.Code, rec.Body.String())
	}

	rec = doRawPkg(t, router, http.MethodGet,
		"/api/packages/xcorp/lab/maven/com/ihuull/hello-mvn/0.1.0/hello-mvn-0.1.0.jar.sha1",
		aliceTok, nil, "")
	if rec.Code != http.StatusOK || rec.Body.Len() != 40 {
		t.Fatalf("sha1 get: %d %q", rec.Code, rec.Body.String())
	}

	snap := []byte("snap-jar")
	rec = doRawPkg(t, router, http.MethodPut,
		"/api/packages/xcorp/lab/maven/com/ihuull/hello-mvn/0.1.0-SNAPSHOT/hello-mvn-0.1.0-SNAPSHOT.jar",
		aliceTok, snap, "application/octet-stream")
	if rec.Code != http.StatusCreated {
		t.Fatalf("snapshot: %d %s", rec.Code, rec.Body.String())
	}

	rec = doRawPkg(t, router, http.MethodGet,
		"/api/packages/xcorp/lab/maven/com/ihuull/hello-mvn/0.1.0/hello-mvn-0.1.0.jar",
		"", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem JWE deveria 401, veio %d", rec.Code)
	}
}

func TestForgePackages_NugetPushIndexAndAuth(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_, aliceTok, _ := seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("package", "hello.1.2.3.nupkg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, "nupkg-bytes"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	rec := doRawPkg(t, router, http.MethodPut, "/api/packages/xcorp/lab/nuget", aliceTok, buf.Bytes(), w.FormDataContentType())
	if rec.Code != http.StatusCreated {
		t.Fatalf("nuget push: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSONPkg(t, router, http.MethodGet, "/api/packages/xcorp/lab/nuget/index.json", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("index: %d %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("PackagePublish/2.0.0")) {
		t.Fatalf("index resources: %s", rec.Body.String())
	}

	rec = doJSONPkg(t, router, http.MethodGet, "/api/packages/xcorp/lab/nuget/flat/hello/index.json", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("versions: %d %s", rec.Code, rec.Body.String())
	}
	var vers struct {
		Versions []string `json:"versions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &vers); err != nil {
		t.Fatal(err)
	}
	if len(vers.Versions) != 1 || vers.Versions[0] != "1.2.3" {
		t.Fatalf("versions: %+v", vers)
	}

	rec = doRawPkg(t, router, http.MethodGet,
		"/api/packages/xcorp/lab/nuget/flat/hello/1.2.3/hello.1.2.3.nupkg",
		aliceTok, nil, "")
	if rec.Code != http.StatusOK || rec.Body.String() != "nupkg-bytes" {
		t.Fatalf("download: %d %q", rec.Code, rec.Body.String())
	}

	rec = doJSONPkg(t, router, http.MethodGet, "/api/packages/xcorp/lab/nuget/index.json", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem JWE deveria 401, veio %d", rec.Code)
	}
}

func TestForgePackages_RubygemsPushAndAuth(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_, aliceTok, _ := seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)

	rec := doRawPkg(t, router, http.MethodPost,
		"/api/packages/xcorp/lab/rubygems/api/v1/gems?filename=hello-ihuull-0.1.0.gem",
		aliceTok, []byte("gem-bytes"), "application/octet-stream")
	if rec.Code != http.StatusCreated {
		t.Fatalf("gem push: %d %s", rec.Code, rec.Body.String())
	}

	rec = doRawPkg(t, router, http.MethodGet,
		"/api/packages/xcorp/lab/rubygems/gems/hello-ihuull-0.1.0.gem",
		aliceTok, nil, "")
	if rec.Code != http.StatusOK || rec.Body.String() != "gem-bytes" {
		t.Fatalf("gem get: %d %q", rec.Code, rec.Body.String())
	}

	rec = doRawPkg(t, router, http.MethodGet,
		"/api/packages/xcorp/lab/rubygems/gems/hello-ihuull-0.1.0.gem",
		"", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem JWE deveria 401, veio %d", rec.Code)
	}
}

func TestParseTrailingSemver_PlatformGem(t *testing.T) {
	name, ver, ok := parseTrailingSemver("hello-0.1.0-x86_64-linux")
	if !ok || name != "hello" || ver != "0.1.0" {
		t.Fatalf("platform gem: %q %q %v", name, ver, ok)
	}
	name, ver, ok = parseTrailingSemver("hello-ihuull-0.1.0")
	if !ok || name != "hello-ihuull" || ver != "0.1.0" {
		t.Fatalf("plain gem: %q %q %v", name, ver, ok)
	}
	name, ver, ok = parseTrailingSemver("hello.1.2.3")
	if !ok || name != "hello" || ver != "1.2.3" {
		t.Fatalf("nuget: %q %q %v", name, ver, ok)
	}
}

func TestRedactCiSecrets(t *testing.T) {
	in := "XVPN_PACKAGES_TOKEN=abc\nAuthorization: Bearer eyJ.a.b.c.d\necho eyJhbGciOi.aaa.bbb.ccc.ddd\n"
	out := redactCiSecrets(in)
	if strings.Contains(out, "abc") || strings.Contains(out, "eyJ") {
		t.Fatalf("não redigiu: %q", out)
	}
}

func TestPackagesToken_RejectedOnGit(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_, _, _ = seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)
	var proj store.Project
	if err := app.Store.DB.Where("slug = ?", "lab").First(&proj).Error; err != nil {
		t.Fatal(err)
	}
	tok := app.issuePackagesTokenForJob(store.CiJob{Actor: "admin"}, proj)
	req := httptest.NewRequest(http.MethodGet, "/xcorp/lab/info/refs?service=git-upload-pack", nil)
	req.Host = "xgit.corp.ihuull.com"
	req.SetBasicAuth("admin", tok)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("git com aud=packages deveria 403, veio %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPackagesToken_ScopedAudience(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_, _, _ = seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)
	var proj store.Project
	if err := app.Store.DB.Where("slug = ?", "lab").First(&proj).Error; err != nil {
		t.Fatal(err)
	}
	tok := app.issuePackagesTokenForJob(store.CiJob{Actor: "admin"}, proj)
	if tok == "" {
		t.Fatal("token vazio")
	}
	rec := doJSONPkg(t, router, http.MethodGet, "/api/projects/xcorp/lab/packages", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("registry deveria aceitar aud=packages: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/projects", nil, tok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("fora do registry deveria 403, veio %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/users", nil, tok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin list deveria 403, veio %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWorkflowPublishTemplatesOmitToken(t *testing.T) {
	for _, tpl := range workflowTemplates {
		if tpl.Category != "publish" {
			continue
		}
		if strings.Contains(tpl.Script, "echo ") && strings.Contains(tpl.Script, "publish") {
			t.Fatalf("%s ainda é echo: %s", tpl.ID, tpl.Script)
		}
		if strings.Contains(tpl.Script, "{{TOKEN}}") || strings.Contains(tpl.Script, "{{JWE}}") {
			t.Fatalf("%s interpola token: %s", tpl.ID, tpl.Script)
		}
		if !strings.Contains(tpl.Script, "XVPN_PACKAGES_TOKEN") {
			t.Fatalf("%s deveria usar XVPN_PACKAGES_TOKEN", tpl.ID)
		}
		if tpl.ID == "generic-xgit" && strings.Contains(tpl.Script, `basename "$PWD"`) {
			t.Fatal("generic-xgit não deve usar PWD (o runner clona em src/)")
		}
	}
}
