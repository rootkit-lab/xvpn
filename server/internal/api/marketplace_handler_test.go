package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/marketplace"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func seedMarketplaceApp(t *testing.T, app *App, visibility store.AppVisibility, withAsset bool) (appID, versionID, assetID uint, content []byte) {
	t.Helper()
	slug := "app-" + strings.Map(func(r rune) rune {
		if r == '/' || r == ' ' {
			return '-'
		}
		return r
	}, t.Name())
	if len(slug) > 60 {
		slug = slug[:60]
	}
	row := store.App{
		Slug:       slug,
		Name:       "App de teste",
		Visibility: visibility,
		Network:    store.AppNetworkPublic,
		Source:     store.AppSourceBuild,
		SourcePath: "apps/" + slug,
	}
	if err := app.Store.DB.Create(&row).Error; err != nil {
		t.Fatalf("erro criando app: %v", err)
	}
	ver := store.AppVersion{AppID: row.ID, Version: "1.0.0", Channel: store.ChannelStable}
	if err := app.Store.DB.Create(&ver).Error; err != nil {
		t.Fatalf("erro criando versão: %v", err)
	}
	if !withAsset {
		return row.ID, ver.ID, 0, nil
	}
	content = []byte("conteudo-asset-" + t.Name())
	result, err := app.Marketplace.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("erro gravando blob: %v", err)
	}
	asset := store.AppAsset{
		AppVersionID: ver.ID,
		Platform:     store.PlatformLinux,
		Arch:         "amd64",
		Filename:     "app.deb",
		SHA256:       result.SHA256,
		SizeBytes:    result.Size,
		StoragePath:  result.RelPath,
	}
	if err := app.Store.DB.Create(&asset).Error; err != nil {
		t.Fatalf("erro criando asset: %v", err)
	}
	return row.ID, ver.ID, asset.ID, content
}

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

func TestHandleListMarketplaceApps_ACL(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_ = createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleAdmin)
	member := createTestUserWithRole(t, app, "member", "senha-member-ok", store.RoleMember)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	memberTok := loginAndGetToken(t, app, router, "member", "senha-member-ok")

	_, _, _, _ = seedMarketplaceApp(t, app, store.AppVisibilityGlobal, false)
	restricted := store.App{
		Slug: "restricted-app", Name: "Restrito", Visibility: store.AppVisibilityRestricted,
		Source: store.AppSourceExternal, SourcePath: "apps/restricted-app",
	}
	if err := app.Store.DB.Create(&restricted).Error; err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, memberTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("member list: %d %s", rec.Code, rec.Body.String())
	}
	var apps []marketplaceAppResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &apps); err != nil {
		t.Fatal(err)
	}
	for _, a := range apps {
		if a.ID == restricted.ID {
			t.Fatal("member não deveria ver app restrito sem ACL")
		}
	}

	rec = doJSON(t, router, http.MethodPut, "/api/marketplace/apps/"+strconv.FormatUint(uint64(restricted.ID), 10)+"/access",
		setMarketplaceAppAccessRequest{UserIDs: []uint{member.ID}}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("set access: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, memberTok)
	if err := json.Unmarshal(rec.Body.Bytes(), &apps); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range apps {
		if a.ID == restricted.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("member deveria ver app restrito após ACL")
	}
}

func TestHandleDownloadMarketplaceAsset_ACL(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_ = createTestUserWithRole(t, app, "member", "senha-member-ok", store.RoleMember)
	memberTok := loginAndGetToken(t, app, router, "member", "senha-member-ok")

	_, _, assetID, content := seedMarketplaceApp(t, app, store.AppVisibilityGlobal, true)
	rec := doJSON(t, router, http.MethodGet, "/api/marketplace/assets/"+strconv.FormatUint(uint64(assetID), 10)+"/download", nil, memberTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("download global: %d", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), content) {
		t.Fatal("conteúdo do download diverge")
	}

	row := store.App{
		Slug: "r2", Name: "R2", Visibility: store.AppVisibilityRestricted,
		Source: store.AppSourceBuild, SourcePath: "apps/r2",
	}
	if err := app.Store.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	ver := store.AppVersion{AppID: row.ID, Version: "1.0.0", Channel: store.ChannelStable}
	if err := app.Store.DB.Create(&ver).Error; err != nil {
		t.Fatal(err)
	}
	result, err := app.Marketplace.Put(bytes.NewReader([]byte("secret")))
	if err != nil {
		t.Fatal(err)
	}
	asset := store.AppAsset{
		AppVersionID: ver.ID, Platform: store.PlatformLinux, Arch: "amd64",
		Filename: "r.deb", SHA256: result.SHA256, SizeBytes: result.Size, StoragePath: result.RelPath,
	}
	if err := app.Store.DB.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodGet, "/api/marketplace/assets/"+strconv.FormatUint(uint64(asset.ID), 10)+"/download", nil, memberTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403, got %d", rec.Code)
	}
}

func TestHandleSetMarketplaceAppAccess_RejectsUnknownUser(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_ = createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleAdmin)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	appID, _, _, _ := seedMarketplaceApp(t, app, store.AppVisibilityRestricted, false)

	rec := doJSON(t, router, http.MethodPut, "/api/marketplace/apps/"+strconv.FormatUint(uint64(appID), 10)+"/access",
		setMarketplaceAppAccessRequest{UserIDs: []uint{99999}}, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestHandleMarketplaceStats(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_ = createTestUserWithRole(t, app, "viewer", "senha-viewer-ok", store.RoleViewer)
	tok := loginAndGetToken(t, app, router, "viewer", "senha-viewer-ok")
	_, _, assetID, _ := seedMarketplaceApp(t, app, store.AppVisibilityGlobal, true)
	_ = app.Store.DB.Model(&store.AppAsset{}).Where("id = ?", assetID).Update("download_count", 3)

	rec := doJSON(t, router, http.MethodGet, "/api/marketplace/stats", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: %d %s", rec.Code, rec.Body.String())
	}
	var stats marketplaceStatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.TotalApps < 1 || stats.TotalAssets < 1 {
		t.Fatalf("stats vazias: %+v", stats)
	}
}

func TestMarketplaceBlobRetainedAcrossVersions(t *testing.T) {
	app, _ := newTestApp(t)
	content := []byte("mesmo-conteudo")
	result, err := app.Marketplace.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	row := store.App{Slug: "dup", Name: "Dup", Visibility: store.AppVisibilityGlobal, Source: store.AppSourceBuild, SourcePath: "apps/dup"}
	if err := app.Store.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	v1 := store.AppVersion{AppID: row.ID, Version: "1.0.0", Channel: store.ChannelStable}
	v2 := store.AppVersion{AppID: row.ID, Version: "1.0.1", Channel: store.ChannelStable}
	if err := app.Store.DB.Create(&v1).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.Store.DB.Create(&v2).Error; err != nil {
		t.Fatal(err)
	}
	a1 := store.AppAsset{AppVersionID: v1.ID, Platform: store.PlatformLinux, Arch: "amd64", Filename: "a.deb", SHA256: result.SHA256, SizeBytes: result.Size, StoragePath: result.RelPath}
	a2 := store.AppAsset{AppVersionID: v2.ID, Platform: store.PlatformLinux, Arch: "amd64", Filename: "a.deb", SHA256: result.SHA256, SizeBytes: result.Size, StoragePath: result.RelPath}
	if err := app.Store.DB.Create(&a1).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.Store.DB.Create(&a2).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.Store.DB.Delete(&store.AppAsset{}, a1.ID).Error; err != nil {
		t.Fatal(err)
	}
	app.removeOrphanBlobs([]store.AppAsset{a1})
	abs := marketplaceBlobAbsPath(t, app, content)
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("blob deveria permanecer (ainda referenciado por a2): %v", err)
	}
}

func TestHandleMarketplaceSync_CreateUpdateArchive(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.PublishToken = "test-publish-token"
	content := []byte("sync-asset-body")
	sha := sha256Hex(content)
	app.fetchAsset = func(ctx context.Context, s *marketplace.Store, url, expected string) (marketplace.PutResult, string, error) {
		if expected != sha {
			t.Fatalf("sha esperado %s got %s", sha, expected)
		}
		result, err := s.Put(bytes.NewReader(content))
		if err != nil {
			return marketplace.PutResult{}, "", err
		}
		return result, "xvpn.deb", nil
	}
	router := NewRouter(app)

	payload := marketplaceSyncRequest{
		CommitSHA: "abc123",
		Apps: []marketplaceSyncAppInput{{
			Slug: "xvpn-client", Name: "XVPN Client", Source: store.AppSourceBuild,
			SourcePath: "apps/xvpn-client", Version: "0.1.0", Channel: store.ChannelStable,
			Visibility: store.AppVisibilityGlobal,
			Assets: []marketplaceSyncAssetInput{{
				Platform: "linux", Arch: "amd64", URL: "https://example.com/xvpn.deb", SHA256: sha, Filename: "xvpn.deb",
			}},
		}},
	}
	rec := doJSON(t, router, http.MethodPost, "/api/marketplace/sync", payload, "test-publish-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("sync create: %d %s", rec.Code, rec.Body.String())
	}
	var resp marketplaceSyncResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Created) != 1 || resp.Created[0] != "xvpn-client" {
		t.Fatalf("created=%v", resp.Created)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/marketplace/sync", payload, "test-publish-token")
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Unchanged) != 1 {
		t.Fatalf("unchanged=%v skipped=%v", resp.Unchanged, resp.Skipped)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/marketplace/sync", marketplaceSyncRequest{Apps: []marketplaceSyncAppInput{}}, "test-publish-token")
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Archived) != 1 || resp.Archived[0] != "xvpn-client" {
		t.Fatalf("archived=%v", resp.Archived)
	}

	_ = createTestUserWithRole(t, app, "viewer2", "senha-viewer2-ok", store.RoleViewer)
	list := doJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, loginAndGetToken(t, app, router, "viewer2", "senha-viewer2-ok"))
	var apps []marketplaceAppResponse
	_ = json.Unmarshal(list.Body.Bytes(), &apps)
	if len(apps) != 0 {
		t.Fatalf("apps arquivados não deveriam listar: %v", apps)
	}
}

func TestRequireMarketplacePublishAuth_RejectsAdminJWT(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.PublishToken = "test-publish-token"
	router := NewRouter(app)
	_ = createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleAdmin)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	rec := doJSON(t, router, http.MethodPost, "/api/marketplace/sync", marketplaceSyncRequest{Apps: []marketplaceSyncAppInput{}}, tok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin JWT não deve sync: %d", rec.Code)
	}
}

func TestHandleMarketplaceSync_NotRegisteredWithoutToken(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.PublishToken = ""
	router := NewRouter(app)
	req := httptest.NewRequest(http.MethodPost, "/api/marketplace/sync", bytes.NewReader([]byte(`{"apps":[]}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("rota sync não deveria existir sem token: %d", rec.Code)
	}
}

func doMarketplaceJSON(t *testing.T, router http.Handler, method, path string, body any, token, host, remoteAddr string, extra map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("erro codificando corpo: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if host != "" {
		req.Host = host
	}
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func seedVpnAndPublicApps(t *testing.T, app *App) (vpnID, publicID, vpnAssetID uint, vpnContent []byte) {
	t.Helper()
	vpn := store.App{
		Slug: "xchat", Name: "xchat", Visibility: store.AppVisibilityGlobal,
		Network: store.AppNetworkVPN, Source: store.AppSourceBuild, SourcePath: "apps/xvpn-chat",
	}
	if err := app.Store.DB.Create(&vpn).Error; err != nil {
		t.Fatal(err)
	}
	pub := store.App{
		Slug: "xvpn-client", Name: "XVPN Client", Visibility: store.AppVisibilityGlobal,
		Network: store.AppNetworkPublic, Source: store.AppSourceBuild, SourcePath: "apps/xvpn-client",
	}
	if err := app.Store.DB.Create(&pub).Error; err != nil {
		t.Fatal(err)
	}
	ver := store.AppVersion{AppID: vpn.ID, Version: "1.0.0", Channel: store.ChannelStable}
	if err := app.Store.DB.Create(&ver).Error; err != nil {
		t.Fatal(err)
	}
	vpnContent = []byte("vpn-asset-" + t.Name())
	result, err := app.Marketplace.Put(bytes.NewReader(vpnContent))
	if err != nil {
		t.Fatal(err)
	}
	asset := store.AppAsset{
		AppVersionID: ver.ID, Platform: store.PlatformLinux, Arch: "amd64",
		Filename: "xchat.deb", SHA256: result.SHA256, SizeBytes: result.Size, StoragePath: result.RelPath,
	}
	if err := app.Store.DB.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	return vpn.ID, pub.ID, asset.ID, vpnContent
}

func listedIDs(t *testing.T, rec *httptest.ResponseRecorder) map[uint]marketplaceAppResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var apps []marketplaceAppResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &apps); err != nil {
		t.Fatal(err)
	}
	out := make(map[uint]marketplaceAppResponse, len(apps))
	for _, a := range apps {
		out[a.ID] = a
	}
	return out
}

func TestHandleMarketplaceSync_PersistsNetwork(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.PublishToken = "test-publish-token"
	content := []byte("sync-network-body")
	sha := sha256Hex(content)
	app.fetchAsset = func(ctx context.Context, s *marketplace.Store, url, expected string) (marketplace.PutResult, string, error) {
		result, err := s.Put(bytes.NewReader(content))
		if err != nil {
			return marketplace.PutResult{}, "", err
		}
		return result, "xchat.deb", nil
	}
	router := NewRouter(app)

	payload := marketplaceSyncRequest{
		CommitSHA: "def456",
		Apps: []marketplaceSyncAppInput{{
			Slug: "xchat", Name: "xchat", Source: store.AppSourceBuild,
			SourcePath: "apps/xvpn-chat", Version: "0.2.0", Channel: store.ChannelStable,
			Visibility: store.AppVisibilityGlobal, Network: store.AppNetworkVPN,
			Assets: []marketplaceSyncAssetInput{{
				Platform: "linux", Arch: "amd64", URL: "https://example.com/xchat.deb", SHA256: sha, Filename: "xchat.deb",
			}},
		}},
	}
	rec := doJSON(t, router, http.MethodPost, "/api/marketplace/sync", payload, "test-publish-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body.String())
	}
	var row store.App
	if err := app.Store.DB.Where("slug = ?", "xchat").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Network != store.AppNetworkVPN {
		t.Fatalf("network após create: %q", row.Network)
	}

	payload.Apps[0].Network = store.AppNetworkPublic
	rec = doJSON(t, router, http.MethodPost, "/api/marketplace/sync", payload, "test-publish-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("sync update: %d %s", rec.Code, rec.Body.String())
	}
	if err := app.Store.DB.Where("slug = ?", "xchat").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Network != store.AppNetworkPublic {
		t.Fatalf("network após update: %q", row.Network)
	}

	payload.Apps[0].Network = "lan"
	rec = doJSON(t, router, http.MethodPost, "/api/marketplace/sync", payload, "test-publish-token")
	var resp marketplaceSyncResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Skipped) != 1 || !strings.Contains(resp.Skipped[0].Reason, "network") {
		t.Fatalf("esperado skip por network inválido: %+v", resp.Skipped)
	}
}

func TestHandleListMarketplaceApps_NetworkFilter(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_ = createTestUserWithRole(t, app, "member", "senha-member-ok", store.RoleMember)
	tok := loginAndGetToken(t, app, router, "member", "senha-member-ok")
	vpnID, publicID, _, _ := seedVpnAndPublicApps(t, app)

	publicStore := listedIDs(t, doMarketplaceJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, tok,
		"marketplace.ihuull.com", "203.0.113.9:443", nil))
	if _, ok := publicStore[vpnID]; ok {
		t.Fatal("app network:vpn não deve aparecer na loja pública sem túnel")
	}
	if _, ok := publicStore[publicID]; !ok {
		t.Fatal("app network:public deve aparecer na loja pública")
	}
	if publicStore[publicID].Network != string(store.AppNetworkPublic) {
		t.Fatalf("network no JSON: %q", publicStore[publicID].Network)
	}

	corp := listedIDs(t, doMarketplaceJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, tok,
		"xchat.corp.ihuull.com", "127.0.0.1:443", nil))
	if _, ok := corp[vpnID]; !ok {
		t.Fatal("app network:vpn deve aparecer em *.corp")
	}

	onTunnel := listedIDs(t, doMarketplaceJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, tok,
		"marketplace.ihuull.com", "10.66.66.5:44444", nil))
	if _, ok := onTunnel[vpnID]; !ok {
		t.Fatal("app network:vpn deve aparecer quando o peer está na VPN")
	}

	viaNginx := listedIDs(t, doMarketplaceJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, tok,
		"marketplace.ihuull.com", "127.0.0.1:54321", map[string]string{"X-Forwarded-For": "10.66.66.8"}))
	if _, ok := viaNginx[vpnID]; !ok {
		t.Fatal("app network:vpn deve aparecer quando o Nginx vê $remote_addr na wg0")
	}

	forged := listedIDs(t, doMarketplaceJSON(t, router, http.MethodGet, "/api/marketplace/apps", nil, tok,
		"marketplace.ihuull.com", "203.0.113.9:443", map[string]string{"X-Forwarded-For": "10.66.66.2"}))
	if _, ok := forged[vpnID]; ok {
		t.Fatal("X-Forwarded-For forjado da internet pública não deve revelar app vpn")
	}
}

func TestHandleDownloadMarketplaceAsset_NetworkFilter(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	_ = createTestUserWithRole(t, app, "member", "senha-member-ok", store.RoleMember)
	tok := loginAndGetToken(t, app, router, "member", "senha-member-ok")
	_, _, assetID, content := seedVpnAndPublicApps(t, app)
	path := "/api/marketplace/assets/" + strconv.FormatUint(uint64(assetID), 10) + "/download"

	denied := doMarketplaceJSON(t, router, http.MethodGet, path, nil, tok,
		"marketplace.ihuull.com", "203.0.113.9:443", nil)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("download público sem túnel: esperado 403, got %d %s", denied.Code, denied.Body.String())
	}

	okCorp := doMarketplaceJSON(t, router, http.MethodGet, path, nil, tok,
		"xdriver.corp.localhost", "127.0.0.1:443", nil)
	if okCorp.Code != http.StatusOK {
		t.Fatalf("download em *.corp: %d %s", okCorp.Code, okCorp.Body.String())
	}
	if !bytes.Equal(okCorp.Body.Bytes(), content) {
		t.Fatal("conteúdo do download em *.corp diverge")
	}

	okVPN := doMarketplaceJSON(t, router, http.MethodGet, path, nil, tok,
		"marketplace.ihuull.com", "10.66.66.4:9", nil)
	if okVPN.Code != http.StatusOK {
		t.Fatalf("download com peer na VPN: %d %s", okVPN.Code, okVPN.Body.String())
	}
}
