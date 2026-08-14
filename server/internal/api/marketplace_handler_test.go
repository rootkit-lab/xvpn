package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// doMultipartUpload monta e executa um POST multipart/form-data — só o
// upload de asset do marketplace usa esse content-type no backend inteiro,
// por isso o helper vive aqui em vez de auth_handler_test.go (doJSON).
// Campos vazios ("") são omitidos do corpo, para exercitar os caminhos de
// validação (platform/file ausentes).
func doMultipartUpload(t *testing.T, router http.Handler, path, token, platform, arch, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if platform != "" {
		if err := w.WriteField("platform", platform); err != nil {
			t.Fatalf("erro escrevendo campo platform: %v", err)
		}
	}
	if arch != "" {
		if err := w.WriteField("arch", arch); err != nil {
			t.Fatalf("erro escrevendo campo arch: %v", err)
		}
	}
	if filename != "" {
		part, err := w.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("erro criando parte do arquivo: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("erro escrevendo conteúdo do arquivo: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("erro fechando multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// createMarketplaceAppAndVersion cria um app (com a visibilidade pedida) e
// uma primeira versão "1.0.0" via API, devolvendo os dois IDs — reduz o
// boilerplate repetido em quase todo teste deste arquivo.
func createMarketplaceAppAndVersion(t *testing.T, router http.Handler, adminToken string, visibility store.AppVisibility) (appID, versionID uint) {
	t.Helper()
	rec := doJSON(t, router, http.MethodPost, "/api/marketplace/apps", createMarketplaceAppRequest{
		Name:       "App de teste",
		Visibility: visibility,
	}, adminToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("erro criando app de teste: %d %s", rec.Code, rec.Body.String())
	}
	var app marketplaceAppResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &app); err != nil {
		t.Fatalf("erro decodificando app: %v", err)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/marketplace/apps/"+strconv.FormatUint(uint64(app.ID), 10)+"/versions",
		createMarketplaceVersionRequest{Version: "1.0.0"}, adminToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("erro criando versão de teste: %d %s", rec.Code, rec.Body.String())
	}
	var version marketplaceVersionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &version); err != nil {
		t.Fatalf("erro decodificando versão: %v", err)
	}
	return app.ID, version.ID
}

// marketplaceBlobAbsPath recalcula o caminho físico esperado de um blob a
// partir do próprio conteúdo, do mesmo jeito que internal/marketplace faz
// internamente — usado para verificar (sem expor StoragePath na API) se um
// blob sobrevive ou some do disco após operações de delete.
func marketplaceBlobAbsPath(t *testing.T, app *App, content []byte) string {
	t.Helper()
	sum := sha256.Sum256(content)
	hexSum := hex.EncodeToString(sum[:])
	abs, err := app.Marketplace.AbsPath(filepath.Join("blobs", hexSum[:2], hexSum))
	if err != nil {
		t.Fatalf("erro resolvendo caminho do blob: %v", err)
	}
	return abs
}

func TestHandleUploadMarketplaceAsset_ComputesHashAndPersists(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	_, versionID := createMarketplaceAppAndVersion(t, router, token, store.AppVisibilityGlobal)
	content := []byte("conteudo binario de teste do pacote .deb")

	rec := doMultipartUpload(t, router, "/api/marketplace/versions/"+strconv.FormatUint(uint64(versionID), 10)+"/assets",
		token, "linux", "", "pacote-teste.deb", content)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}

	var asset marketplaceAssetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &asset); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}

	sum := sha256.Sum256(content)
	wantSHA := hex.EncodeToString(sum[:])
	if asset.SHA256 != wantSHA {
		t.Fatalf("sha256 calculado errado: esperado %s, obtido %s", wantSHA, asset.SHA256)
	}
	if asset.SizeBytes != int64(len(content)) {
		t.Fatalf("size_bytes errado: esperado %d, obtido %d", len(content), asset.SizeBytes)
	}
	if asset.Platform != "linux" {
		t.Fatalf("platform errada: %q", asset.Platform)
	}
	if asset.Arch != defaultAssetArch {
		t.Fatalf("esperava arch padrão %q quando omitida, obtido %q", defaultAssetArch, asset.Arch)
	}
	if asset.Filename != "pacote-teste.deb" {
		t.Fatalf("filename errado: %q", asset.Filename)
	}

	blobPath := marketplaceBlobAbsPath(t, app, content)
	got, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("blob não foi gravado em disco no caminho esperado: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("conteúdo do blob em disco não bate com o upload")
	}
}

func TestHandleUploadMarketplaceAsset_RejectsInvalidOrMissingPlatform(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	_, versionID := createMarketplaceAppAndVersion(t, router, token, store.AppVisibilityGlobal)
	path := "/api/marketplace/versions/" + strconv.FormatUint(uint64(versionID), 10) + "/assets"

	for _, platform := range []string{"", "macos", "ios"} {
		rec := doMultipartUpload(t, router, path, token, platform, "", "arquivo.bin", []byte("x"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("platform=%q: esperado 400, obtido %d: %s", platform, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleUploadMarketplaceAsset_RequiresFile(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	_, versionID := createMarketplaceAppAndVersion(t, router, token, store.AppVisibilityGlobal)

	rec := doMultipartUpload(t, router, "/api/marketplace/versions/"+strconv.FormatUint(uint64(versionID), 10)+"/assets",
		token, "linux", "", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 sem arquivo, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUploadMarketplaceAsset_VersionNotFound(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doMultipartUpload(t, router, "/api/marketplace/versions/9999/assets", token, "linux", "", "a.deb", []byte("x"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404 para versão inexistente, obtido %d", rec.Code)
	}
}

// uploadTestAsset cria app+versão+asset de uma vez, devolvendo o ID do
// asset — usado pelos testes de download que não se importam com a
// resposta do upload em si.
func uploadTestAsset(t *testing.T, router http.Handler, adminToken string, visibility store.AppVisibility, content []byte) (appID, assetID uint) {
	t.Helper()
	appID, versionID := createMarketplaceAppAndVersion(t, router, adminToken, visibility)
	rec := doMultipartUpload(t, router, "/api/marketplace/versions/"+strconv.FormatUint(uint64(versionID), 10)+"/assets",
		adminToken, "linux", "", "pacote.deb", content)
	if rec.Code != http.StatusCreated {
		t.Fatalf("erro subindo asset de teste: %d %s", rec.Code, rec.Body.String())
	}
	var asset marketplaceAssetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &asset); err != nil {
		t.Fatalf("erro decodificando asset: %v", err)
	}
	return appID, asset.ID
}

func TestHandleDownloadMarketplaceAsset_RequiresAuth(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	_, assetID := uploadTestAsset(t, router, token, store.AppVisibilityGlobal, []byte("conteudo"))

	rec := doJSON(t, router, http.MethodGet, "/api/marketplace/assets/"+strconv.FormatUint(uint64(assetID), 10)+"/download", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 sem token, obtido %d", rec.Code)
	}
}

func TestHandleDownloadMarketplaceAsset_GlobalAllowsAnyAuthenticatedRole(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	adminToken := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	createTestUserWithRole(t, app, "viewer1", "senha-viewer-123", store.RoleViewer)
	viewerToken := loginAndGetToken(t, app, router, "viewer1", "senha-viewer-123")

	content := []byte("conteudo do app global")
	_, assetID := uploadTestAsset(t, router, adminToken, store.AppVisibilityGlobal, content)
	downloadPath := "/api/marketplace/assets/" + strconv.FormatUint(uint64(assetID), 10) + "/download"

	rec := doJSON(t, router, http.MethodGet, downloadPath, nil, viewerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 (app global libera qualquer papel), obtido %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Equal(rec.Body.Bytes(), content) {
		t.Fatalf("corpo do download não bate com o conteúdo original")
	}
	if disp := rec.Header().Get("Content-Disposition"); disp == "" {
		t.Fatalf("esperava Content-Disposition com o nome do arquivo")
	}

	// download_count precisa refletir esse download (só visível via
	// listagem, que não expõe download_count fora do bloco Assets).
	rec = doJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, adminToken)
	var apps []marketplaceAppResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &apps)
	if len(apps) != 1 || len(apps[0].Versions) != 1 || len(apps[0].Versions[0].Assets) != 1 {
		t.Fatalf("catálogo inesperado após download: %+v", apps)
	}
	if got := apps[0].Versions[0].Assets[0].DownloadCount; got != 1 {
		t.Fatalf("esperava download_count=1, obtido %d", got)
	}
}

func TestHandleDownloadMarketplaceAsset_RestrictedRequiresExplicitAccess(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	adminToken := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	member := createTestUserWithRole(t, app, "member1", "senha-member-123", store.RoleMember)
	memberToken := loginAndGetToken(t, app, router, "member1", "senha-member-123")

	appID, assetID := uploadTestAsset(t, router, adminToken, store.AppVisibilityRestricted, []byte("conteudo restrito"))
	downloadPath := "/api/marketplace/assets/" + strconv.FormatUint(uint64(assetID), 10) + "/download"

	rec := doJSON(t, router, http.MethodGet, downloadPath, nil, memberToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 sem AppAccess, obtido %d: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPut, "/api/marketplace/apps/"+strconv.FormatUint(uint64(appID), 10)+"/access",
		setMarketplaceAppAccessRequest{UserIDs: []uint{member.ID}}, adminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("erro concedendo acesso: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, downloadPath, nil, memberToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 após conceder AppAccess, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleDownloadMarketplaceAsset_AdminBypassesACL cobre PLAN.md §6.7
// ("admin/super_admin: Admin + download") com um admin que nunca recebeu
// AppAccess — precisa baixar mesmo assim.
func TestHandleDownloadMarketplaceAsset_AdminBypassesACL(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "publisher", "senha-publisher-123", store.RoleAdmin)
	publisherToken := loginAndGetToken(t, app, router, "publisher", "senha-publisher-123")
	createTestUserWithRole(t, app, "outroadmin", "senha-outroadmin-123", store.RoleAdmin)
	otherAdminToken := loginAndGetToken(t, app, router, "outroadmin", "senha-outroadmin-123")

	_, assetID := uploadTestAsset(t, router, publisherToken, store.AppVisibilityRestricted, []byte("conteudo restrito"))

	rec := doJSON(t, router, http.MethodGet, "/api/marketplace/assets/"+strconv.FormatUint(uint64(assetID), 10)+"/download", nil, otherAdminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200: admin sempre baixa mesmo sem AppAccess, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDownloadMarketplaceAsset_NotFound(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "member1", "senha-member-123", store.RoleMember)
	token := loginAndGetToken(t, app, router, "member1", "senha-member-123")

	rec := doJSON(t, router, http.MethodGet, "/api/marketplace/assets/9999/download", nil, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404, obtido %d", rec.Code)
	}
}

// TestHandleListMarketplaceApps_FiltersRestrictedByACL cobre a matriz de
// visibilidade completa de PLAN.md §6.8: global aparece pra todo mundo,
// restricted só aparece pra quem tem AppAccess (ou é admin+), e
// AccessUserIDs nunca vaza pra quem não administra o marketplace.
func TestHandleListMarketplaceApps_FiltersRestrictedByACL(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	adminToken := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	withAccess := createTestUserWithRole(t, app, "comacesso", "senha-comacesso-123", store.RoleMember)
	withAccessToken := loginAndGetToken(t, app, router, "comacesso", "senha-comacesso-123")
	createTestUserWithRole(t, app, "semacesso", "senha-semacesso-123", store.RoleViewer)
	noAccessToken := loginAndGetToken(t, app, router, "semacesso", "senha-semacesso-123")

	globalAppID, _ := createMarketplaceAppAndVersion(t, router, adminToken, store.AppVisibilityGlobal)
	restrictedAppID, _ := createMarketplaceAppAndVersion(t, router, adminToken, store.AppVisibilityRestricted)

	rec := doJSON(t, router, http.MethodPut, "/api/marketplace/apps/"+strconv.FormatUint(uint64(restrictedAppID), 10)+"/access",
		setMarketplaceAppAccessRequest{UserIDs: []uint{withAccess.ID}}, adminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("erro concedendo acesso: %d %s", rec.Code, rec.Body.String())
	}

	// Sem acesso: só enxerga o app global.
	rec = doJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, noAccessToken)
	var apps []marketplaceAppResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &apps); err != nil {
		t.Fatalf("erro decodificando catálogo: %v", err)
	}
	if len(apps) != 1 || apps[0].ID != globalAppID {
		t.Fatalf("esperava só o app global para quem não tem AppAccess, obtido %+v", apps)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("access_user_ids")) {
		t.Fatalf("access_user_ids nunca deveria aparecer pra quem não administra o marketplace")
	}

	// Com acesso explícito: enxerga os dois.
	rec = doJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, withAccessToken)
	_ = json.Unmarshal(rec.Body.Bytes(), &apps)
	if len(apps) != 2 {
		t.Fatalf("esperava 2 apps para quem tem AppAccess no restrito, obtido %d", len(apps))
	}

	// Admin: enxerga os dois e vê a lista de quem tem acesso ao restrito.
	rec = doJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, adminToken)
	_ = json.Unmarshal(rec.Body.Bytes(), &apps)
	if len(apps) != 2 {
		t.Fatalf("esperava 2 apps para admin, obtido %d", len(apps))
	}
	for _, a := range apps {
		if a.ID == restrictedAppID {
			if len(a.AccessUserIDs) != 1 || a.AccessUserIDs[0] != withAccess.ID {
				t.Fatalf("access_user_ids do app restrito errado: %+v", a.AccessUserIDs)
			}
		}
	}
}

// TestHandleListMarketplaceApps_OrdersAssetsByPlatformThenArch cobre a
// listagem por plataforma: mesmo subindo os assets fora de ordem, a
// resposta os agrupa dentro da versão ordenados por platform/arch (ver
// Preload em handleListMarketplaceApps), o que é o que a tela de
// Marketplace usa para renderizar por plataforma.
func TestHandleListMarketplaceApps_OrdersAssetsByPlatformThenArch(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	adminToken := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	_, versionID := createMarketplaceAppAndVersion(t, router, adminToken, store.AppVisibilityGlobal)
	versionPath := "/api/marketplace/versions/" + strconv.FormatUint(uint64(versionID), 10) + "/assets"

	// De propósito fora de ordem alfabética (windows, android, linux).
	uploads := []struct{ platform, content string }{
		{"windows", "conteudo-windows"},
		{"android", "conteudo-android"},
		{"linux", "conteudo-linux"},
	}
	for _, u := range uploads {
		rec := doMultipartUpload(t, router, versionPath, adminToken, u.platform, "", "pacote", []byte(u.content))
		if rec.Code != http.StatusCreated {
			t.Fatalf("erro subindo asset %s: %d %s", u.platform, rec.Code, rec.Body.String())
		}
	}

	rec := doJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, adminToken)
	var apps []marketplaceAppResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &apps); err != nil {
		t.Fatalf("erro decodificando catálogo: %v", err)
	}
	if len(apps) != 1 || len(apps[0].Versions) != 1 {
		t.Fatalf("catálogo inesperado: %+v", apps)
	}
	assets := apps[0].Versions[0].Assets
	if len(assets) != 3 {
		t.Fatalf("esperava 3 assets, obtido %d", len(assets))
	}
	wantOrder := []string{"android", "linux", "windows"}
	for i, want := range wantOrder {
		if assets[i].Platform != want {
			t.Fatalf("ordem de plataformas inesperada, posição %d: esperado %s, obtido %s (lista completa: %+v)", i, want, assets[i].Platform, assets)
		}
	}
}

func TestHandleSetMarketplaceAppAccess_RejectsUnknownUserID(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	adminToken := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	appID, _ := createMarketplaceAppAndVersion(t, router, adminToken, store.AppVisibilityRestricted)

	rec := doJSON(t, router, http.MethodPut, "/api/marketplace/apps/"+strconv.FormatUint(uint64(appID), 10)+"/access",
		setMarketplaceAppAccessRequest{UserIDs: []uint{99999}}, adminToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para user_id inexistente, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleSetMarketplaceAppAccess_ReplacesPreviousList(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	adminToken := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	userA := createTestUserWithRole(t, app, "usuarioa", "senha-usuarioa-123", store.RoleMember)
	userB := createTestUserWithRole(t, app, "usuariob", "senha-usuariob-123", store.RoleMember)

	appID, _ := createMarketplaceAppAndVersion(t, router, adminToken, store.AppVisibilityRestricted)
	accessPath := "/api/marketplace/apps/" + strconv.FormatUint(uint64(appID), 10) + "/access"

	rec := doJSON(t, router, http.MethodPut, accessPath, setMarketplaceAppAccessRequest{UserIDs: []uint{userA.ID}}, adminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("erro na primeira concessão: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPut, accessPath, setMarketplaceAppAccessRequest{UserIDs: []uint{userB.ID}}, adminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("erro na segunda concessão: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, adminToken)
	var apps []marketplaceAppResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &apps)
	var found *marketplaceAppResponse
	for i := range apps {
		if apps[i].ID == appID {
			found = &apps[i]
		}
	}
	if found == nil {
		t.Fatalf("app não encontrado na listagem")
	}
	if len(found.AccessUserIDs) != 1 || found.AccessUserIDs[0] != userB.ID {
		t.Fatalf("esperava só userB com acesso após a segunda chamada, obtido %+v", found.AccessUserIDs)
	}
}

// TestHandleDeleteMarketplaceAsset_PreservesSharedBlobUntilAllReferencesGone
// exercita o dedupe de ponta a ponta pela API: dois assets (de apps
// diferentes) com o mesmo conteúdo compartilham um único blob em disco; o
// blob só pode sumir quando o último asset que aponta pra ele for
// removido (ver removeOrphanBlobs em marketplace_handler.go).
func TestHandleDeleteMarketplaceAsset_PreservesSharedBlobUntilAllReferencesGone(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	adminToken := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	content := []byte("conteudo identico compartilhado entre dois apps")
	_, versionAID := createMarketplaceAppAndVersion(t, router, adminToken, store.AppVisibilityGlobal)
	_, versionBID := createMarketplaceAppAndVersion(t, router, adminToken, store.AppVisibilityGlobal)

	rec := doMultipartUpload(t, router, "/api/marketplace/versions/"+strconv.FormatUint(uint64(versionAID), 10)+"/assets",
		adminToken, "linux", "", "pacote.deb", content)
	var assetA marketplaceAssetResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &assetA)

	rec = doMultipartUpload(t, router, "/api/marketplace/versions/"+strconv.FormatUint(uint64(versionBID), 10)+"/assets",
		adminToken, "linux", "", "outro-nome.deb", content)
	var assetB marketplaceAssetResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &assetB)

	blobPath := marketplaceBlobAbsPath(t, app, content)
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("blob deveria existir após os dois uploads: %v", err)
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/marketplace/assets/"+strconv.FormatUint(uint64(assetA.ID), 10), nil, adminToken)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperado 204 ao deletar assetA, obtido %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("blob não deveria sumir enquanto assetB ainda existe: %v", err)
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/marketplace/assets/"+strconv.FormatUint(uint64(assetB.ID), 10), nil, adminToken)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperado 204 ao deletar assetB, obtido %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(blobPath); !os.IsNotExist(err) {
		t.Fatalf("blob deveria ter sido removido após o último asset que o referenciava, err=%v", err)
	}
}

func TestHandleCreateMarketplaceApp_ValidationErrors(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/marketplace/apps", createMarketplaceAppRequest{Name: "  "}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para name vazio, obtido %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/marketplace/apps",
		createMarketplaceAppRequest{Name: "App", Visibility: store.AppVisibility("publico")}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para visibility inválida, obtido %d", rec.Code)
	}
}

func TestHandleCreateMarketplaceVersion_ValidationErrors(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	appID, _ := createMarketplaceAppAndVersion(t, router, token, store.AppVisibilityGlobal)
	versionsPath := "/api/marketplace/apps/" + strconv.FormatUint(uint64(appID), 10) + "/versions"

	rec := doJSON(t, router, http.MethodPost, versionsPath, createMarketplaceVersionRequest{Version: " "}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para version vazia, obtido %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPost, versionsPath, createMarketplaceVersionRequest{Version: "2.0.0", Channel: "nightly"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para channel inválido, obtido %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/marketplace/apps/9999/versions", createMarketplaceVersionRequest{Version: "1.0.0"}, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404 para app inexistente, obtido %d", rec.Code)
	}
}

// TestHandleDeleteMarketplaceApp_CascadesAndRemovesBlob confirma que
// apagar um app não deixa nem versão/asset/ACL órfãos no banco nem blob
// órfão em disco (ver handleDeleteMarketplaceApp + removeOrphanBlobs).
func TestHandleDeleteMarketplaceApp_CascadesAndRemovesBlob(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	adminToken := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	member := createTestUserWithRole(t, app, "membro", "senha-membro-123", store.RoleMember)

	content := []byte("conteudo exclusivo deste app")
	appID, versionID := createMarketplaceAppAndVersion(t, router, adminToken, store.AppVisibilityRestricted)
	doJSON(t, router, http.MethodPut, "/api/marketplace/apps/"+strconv.FormatUint(uint64(appID), 10)+"/access",
		setMarketplaceAppAccessRequest{UserIDs: []uint{member.ID}}, adminToken)
	doMultipartUpload(t, router, "/api/marketplace/versions/"+strconv.FormatUint(uint64(versionID), 10)+"/assets",
		adminToken, "linux", "", "pacote.deb", content)

	blobPath := marketplaceBlobAbsPath(t, app, content)
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("blob deveria existir antes da exclusão: %v", err)
	}

	rec := doJSON(t, router, http.MethodDelete, "/api/marketplace/apps/"+strconv.FormatUint(uint64(appID), 10), nil, adminToken)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperado 204, obtido %d: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, adminToken)
	var apps []marketplaceAppResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &apps)
	if len(apps) != 0 {
		t.Fatalf("esperava catálogo vazio após deletar o único app, obtido %+v", apps)
	}

	if _, err := os.Stat(blobPath); !os.IsNotExist(err) {
		t.Fatalf("blob deveria ter sido removido junto do app, err=%v", err)
	}
}

// TestMarketplaceHandlers_AuditLogsKeyActions cobre ROADMAP.md Fase 11
// ("Audit log: upload, publish, delete, download") — cada ação sensível
// precisa deixar rastro em /api/audit.
func TestMarketplaceHandlers_AuditLogsKeyActions(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	adminToken := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	content := []byte("conteudo para auditoria")
	appID, assetID := uploadTestAsset(t, router, adminToken, store.AppVisibilityGlobal, content)
	doJSON(t, router, http.MethodGet, "/api/marketplace/assets/"+strconv.FormatUint(uint64(assetID), 10)+"/download", nil, adminToken)
	doJSON(t, router, http.MethodDelete, "/api/marketplace/apps/"+strconv.FormatUint(uint64(appID), 10), nil, adminToken)

	rec := doJSON(t, router, http.MethodGet, "/api/audit", nil, adminToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("erro listando auditoria: %d %s", rec.Code, rec.Body.String())
	}
	var logs []auditLogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &logs); err != nil {
		t.Fatalf("erro decodificando auditoria: %v", err)
	}

	wantActions := map[string]bool{
		"marketplace.app_create":     false,
		"marketplace.version_create": false,
		"marketplace.asset_upload":   false,
		"marketplace.asset_download": false,
		"marketplace.app_delete":     false,
	}
	for _, l := range logs {
		if _, ok := wantActions[l.Action]; ok {
			wantActions[l.Action] = true
		}
	}
	for action, seen := range wantActions {
		if !seen {
			t.Fatalf("esperava uma entrada de auditoria %q, não encontrada em %+v", action, logs)
		}
	}
}
