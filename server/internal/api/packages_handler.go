package api

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/marketplace"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// maxPackageBytes limita um artefato do registry (Fase 45.1). npm.com
// aceita ~100 MiB; 64 MiB cabe no VPS sem um publish só encher o disco.
const maxPackageBytes = 64 << 20

var (
	genericPackageName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,213}$`)
	mavenPackageName   = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,213}$`)
	npmPackageName     = regexp.MustCompile(`^(?:@[a-z0-9-~][a-z0-9-._~]{0,213}/)?[a-z0-9-~][a-z0-9-._~]{0,213}$`)
	packageVersion     = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9._-]+)?$`)
	pep503Sep          = regexp.MustCompile(`[-_.]+`)
)

type forgePackageVersionJSON struct {
	ID            uint      `json:"id"`
	Version       string    `json:"version"`
	Filename      string    `json:"filename"`
	SHA256        string    `json:"sha256"`
	Size          int64     `json:"size"`
	Description   string    `json:"description,omitempty"`
	PublishedBy   string    `json:"published_by,omitempty"`
	DownloadCount int64     `json:"download_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type forgePackageJSON struct {
	ID          uint                      `json:"id"`
	ProjectSlug string                    `json:"project_slug"`
	Kind        store.ForgePackageKind    `json:"kind"`
	Name        string                    `json:"name"`
	Latest      string                    `json:"latest,omitempty"`
	Versions    []forgePackageVersionJSON `json:"versions"`
	RegistryURL string                    `json:"registry_url,omitempty"`
	CanPublish  bool                      `json:"can_publish"`
}

func packagesHostOK(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	h = strings.ToLower(h)
	switch h {
	case "xgit.corp.ihuull.com", "xgit.corp.localhost",
		"xadmin.corp.ihuull.com", "xadmin.corp.localhost":
		return true
	default:
		return false
	}
}

// RequirePackagesHost impede que o registry (PLAN §5: só VPN) responda em
// hosts públicos (xvpn.ihuull.com etc.) mesmo com JWE válido. UI: xgit + xadmin.
func (a *App) RequirePackagesHost() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !packagesHostOK(c.Request.Host) || !packagesForwardedFromMesh(c) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "não encontrado"})
			return
		}
		c.Next()
	}
}

// packagesForwardedFromMesh: se o Nginx mandou X-Forwarded-For, o último
// hop tem de ser wg0 ou loopback. Impede Host: xgit.corp num vhost público
// (proxy_set_header Host $host) de abrir o registry na internet.
func packagesForwardedFromMesh(c *gin.Context) bool {
	xff := strings.TrimSpace(c.GetHeader("X-Forwarded-For"))
	if xff == "" {
		return true
	}
	parts := strings.Split(xff, ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	ip := net.ParseIP(last)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	_, wg, err := net.ParseCIDR("10.66.66.0/24")
	if err != nil {
		return false
	}
	return wg.Contains(ip)
}

func (a *App) handleListForgePackages(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	var rows []store.ForgePackage
	if err := a.Store.DB.Order("updated_at desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]forgePackageJSON, 0, len(rows))
	for _, pkg := range rows {
		var proj store.Project
		if err := a.Store.DB.First(&proj, pkg.ProjectID).Error; err != nil || proj.ArchivedAt != nil {
			continue
		}
		if !a.canSeeProject(user, proj) {
			continue
		}
		items = append(items, a.forgePackageJSON(user, proj, pkg, true))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleListProjectPackages(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	var rows []store.ForgePackage
	if err := a.Store.DB.Where("project_id = ?", proj.ID).Order("updated_at desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]forgePackageJSON, 0, len(rows))
	for _, pkg := range rows {
		items = append(items, a.forgePackageJSON(user, proj, pkg, true))
	}
	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"can_publish": a.canAccessProjectFiles(user, proj, true),
	})
}

func (a *App) handleUploadProjectPackage(c *gin.Context) {
	if a.Packages == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "registry indisponível"})
		return
	}
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canAccessProjectFiles(user, proj, true) {
		c.JSON(http.StatusForbidden, gin.H{"error": "só developer+ publica packages"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPackageBytes+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo obrigatório"})
		return
	}
	if fh.Size > maxPackageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 64 MiB"})
		return
	}
	kind := store.ForgePackageKind(strings.TrimSpace(c.PostForm("kind")))
	if kind == "" {
		kind = store.ForgePackageKindGeneric
	}
	name, err := normalizePackageName(kind, c.PostForm("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	version := strings.TrimSpace(c.PostForm("version"))
	if !packageVersion.MatchString(version) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "versão inválida (semver)"})
		return
	}
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo inválido"})
		return
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxPackageBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "falha lendo arquivo"})
		return
	}
	if int64(len(data)) > maxPackageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 64 MiB"})
		return
	}
	filename := sanitizePackageFilename(fh.Filename)
	out, err := a.publishPackageBytes(user, proj, kind, name, version, filename, strings.TrimSpace(c.PostForm("description")), data)
	if err != nil {
		writePackageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (a *App) handleDownloadPackageVersion(c *gin.Context) {
	if a.Packages == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "registry indisponível"})
		return
	}
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var ver store.ForgePackageVersion
	if err := a.Store.DB.First(&ver, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "versão não encontrada"})
		return
	}
	var pkg store.ForgePackage
	if err := a.Store.DB.First(&pkg, ver.PackageID).Error; err != nil || pkg.ProjectID != proj.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "versão não encontrada"})
		return
	}
	if !a.canSeeProject(user, proj) {
		c.JSON(http.StatusNotFound, gin.H{"error": "versão não encontrada"})
		return
	}
	abs, err := a.Packages.AbsPath(ver.StoragePath)
	if err != nil {
		slog.Error("caminho de package inválido", "id", ver.ID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.Store.DB.Model(&store.ForgePackageVersion{}).Where("id = ?", ver.ID).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error; err != nil {
		slog.Error("falha ao incrementar download de package", "id", ver.ID, "err", err)
	}
	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "forge.package_download",
		"slug="+proj.Slug+" name="+pkg.Name+" version="+ver.Version)
	c.FileAttachment(abs, ver.Filename)
}

func (a *App) handleNpmPublish(c *gin.Context) {
	if a.Packages == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "registry indisponível"})
		return
	}
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canAccessProjectFiles(user, proj, true) {
		c.JSON(http.StatusForbidden, gin.H{"error": "só developer+ publica packages"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPackageBytes*2)
	var doc npmPublishDocument
	if err := json.NewDecoder(c.Request.Body).Decode(&doc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "manifest npm inválido"})
		return
	}
	name, err := normalizePackageName(store.ForgePackageKindNPM, firstNonEmpty(npmPackageNameParam(c), doc.Name))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	version, meta, attName, att, err := doc.pickVersion()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(att.Data))
	if err != nil || len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "anexo npm inválido"})
		return
	}
	if int64(len(raw)) > maxPackageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 64 MiB"})
		return
	}
	filename := sanitizePackageFilename(attName)
	if filename == "package.bin" {
		filename = strings.ReplaceAll(name, "/", "-") + "-" + version + ".tgz"
	}
	out, err := a.publishPackageBytes(user, proj, store.ForgePackageKindNPM, name, version, filename, meta.Description, raw)
	if err != nil {
		writePackageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (a *App) handleNpmPackument(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	name, err := normalizePackageName(store.ForgePackageKindNPM, npmPackageNameParam(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var pkg store.ForgePackage
	if err := a.Store.DB.Where("project_id = ? AND kind = ? AND name = ?", proj.ID, store.ForgePackageKindNPM, name).
		First(&pkg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "package não encontrado"})
		return
	}
	_ = user
	var vers []store.ForgePackageVersion
	if err := a.Store.DB.Where("package_id = ?", pkg.ID).Order("created_at asc").Find(&vers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if len(vers) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "package não encontrado"})
		return
	}
	versions := make(map[string]any, len(vers))
	latest := vers[len(vers)-1].Version
	for _, ver := range vers {
		tarball := gitCloneHost + "/api/projects/" + a.projectRepo(proj) + "/packages/" + strconv.FormatUint(uint64(ver.ID), 10) + "/download"
		integrity, shasum := ver.Integrity, ver.Shasum
		if integrity == "" || shasum == "" {
			if a.Packages != nil {
				if abs, err := a.Packages.AbsPath(ver.StoragePath); err == nil {
					if raw, err := readFileCap(abs, maxPackageBytes); err == nil {
						integrity, shasum = npmDigest(raw)
					}
				}
			}
		}
		versions[ver.Version] = gin.H{
			"name":        pkg.Name,
			"version":     ver.Version,
			"description": ver.Description,
			"dist": gin.H{
				"tarball":   tarball,
				"integrity": integrity,
				"shasum":    shasum,
			},
		}
		latest = ver.Version
	}
	c.JSON(http.StatusOK, gin.H{
		"name":      pkg.Name,
		"versions":  versions,
		"dist-tags": gin.H{"latest": latest},
	})
}

func (a *App) forgePackageJSON(user store.User, proj store.Project, pkg store.ForgePackage, withVersions bool) forgePackageJSON {
	out := forgePackageJSON{
		ID:          pkg.ID,
		ProjectSlug: a.projectRepo(proj),
		Kind:        pkg.Kind,
		Name:        pkg.Name,
		CanPublish:  a.canAccessProjectFiles(user, proj, true),
		Versions:    []forgePackageVersionJSON{},
	}
	repo := a.projectRepo(proj)
	switch pkg.Kind {
	case store.ForgePackageKindNPM:
		out.RegistryURL = gitCloneHost + "/api/packages/" + repo + "/npm/"
	case store.ForgePackageKindPyPI:
		out.RegistryURL = gitCloneHost + "/api/packages/" + repo + "/pypi/simple/"
	case store.ForgePackageKindMaven:
		out.RegistryURL = gitCloneHost + "/api/packages/" + repo + "/maven"
	case store.ForgePackageKindNuGet:
		out.RegistryURL = gitCloneHost + "/api/packages/" + repo + "/nuget/index.json"
	case store.ForgePackageKindRubyGems:
		out.RegistryURL = gitCloneHost + "/api/packages/" + repo + "/rubygems"
	}
	if !withVersions {
		return out
	}
	var vers []store.ForgePackageVersion
	_ = a.Store.DB.Where("package_id = ?", pkg.ID).Order("created_at desc").Find(&vers).Error
	for i, ver := range vers {
		var publisher store.User
		who := ""
		if ver.PublishedByID != 0 && a.Store.DB.First(&publisher, ver.PublishedByID).Error == nil {
			who = publisher.Username
		}
		display := packageDisplayVersion(pkg.Kind, ver.Version)
		out.Versions = append(out.Versions, forgePackageVersionJSON{
			ID:            ver.ID,
			Version:       display,
			Filename:      ver.Filename,
			SHA256:        ver.SHA256,
			Size:          ver.Size,
			Description:   ver.Description,
			PublishedBy:   who,
			DownloadCount: ver.DownloadCount,
			CreatedAt:     ver.CreatedAt,
		})
		if i == 0 {
			out.Latest = display
		}
	}
	return out
}

func (a *App) publishPackageBytes(user store.User, proj store.Project, kind store.ForgePackageKind, name, version, filename, description string, data []byte) (forgePackageJSON, error) {
	result, err := a.Packages.Put(bytes.NewReader(data))
	if err != nil {
		if err == marketplace.ErrAssetTooLarge {
			return forgePackageJSON{}, errPackageTooLarge
		}
		return forgePackageJSON{}, err
	}
	integrity, shasum := npmDigest(data)
	var pkg store.ForgePackage
	err = a.Store.DB.Where("project_id = ? AND kind = ? AND name = ?", proj.ID, kind, name).First(&pkg).Error
	if err != nil {
		pkg = store.ForgePackage{ProjectID: proj.ID, Kind: kind, Name: name}
		if err := a.Store.DB.Create(&pkg).Error; err != nil {
			a.removeOrphanPackageBlob(result.RelPath)
			return forgePackageJSON{}, err
		}
	}
	storedVersion := packageStoredVersion(kind, version, filename)
	var existing store.ForgePackageVersion
	if err := a.Store.DB.Where("package_id = ? AND version = ?", pkg.ID, storedVersion).First(&existing).Error; err == nil {
		if existing.SHA256 != result.SHA256 {
			a.removeOrphanPackageBlob(result.RelPath)
			return forgePackageJSON{}, errPackageExists
		}
		return a.forgePackageJSON(user, proj, pkg, true), nil
	}
	ver := store.ForgePackageVersion{
		PackageID:     pkg.ID,
		Version:       storedVersion,
		Filename:      filename,
		SHA256:        result.SHA256,
		Integrity:     integrity,
		Shasum:        shasum,
		Size:          result.Size,
		StoragePath:   result.RelPath,
		Description:   description,
		PublishedByID: user.ID,
	}
	if err := a.Store.DB.Create(&ver).Error; err != nil {
		a.removeOrphanPackageBlob(result.RelPath)
		return forgePackageJSON{}, err
	}
	_ = a.Store.DB.Model(&pkg).Update("updated_at", time.Now()).Error
	_ = a.Store.LogAudit(user.Username, "forge.package_publish",
		"slug="+proj.Slug+" kind="+string(kind)+" name="+name+" version="+version)
	return a.forgePackageJSON(user, proj, pkg, true), nil
}

// removeOrphanPackageBlob só apaga o ficheiro se nenhuma versão ainda
// aponta para o RelPath — o store é content-addressed e dois packages
// podem partilhar o mesmo blob.
func (a *App) removeOrphanPackageBlob(relPath string) {
	if a.Packages == nil || relPath == "" {
		return
	}
	var count int64
	if err := a.Store.DB.Model(&store.ForgePackageVersion{}).Where("storage_path = ?", relPath).Count(&count).Error; err != nil {
		slog.Error("falha ao checar referências de package blob", "err", err)
		return
	}
	if count > 0 {
		return
	}
	if err := a.Packages.Remove(relPath); err != nil {
		slog.Error("falha ao remover package blob órfão", "path", relPath, "err", err)
	}
}

type packageError string

func (e packageError) Error() string { return string(e) }

const (
	errPackageExists   = packageError("versão já publicada")
	errPackageTooLarge = packageError("arquivo excede 64 MiB")
	errPackageName     = packageError("nome de package inválido")
	errPackageKind     = packageError("kind deve ser generic, npm, pypi, maven, nuget ou rubygems")
	errNpmManifest     = packageError("manifest npm sem versão ou anexo")
)

func writePackageError(c *gin.Context, err error) {
	switch err {
	case errPackageExists:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errPackageTooLarge, marketplace.ErrAssetTooLarge:
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": errPackageTooLarge.Error()})
	case errPackageName, errPackageKind, errNpmManifest:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		slog.Error("falha no registry XGIT", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
	}
}

func normalizePackageName(kind store.ForgePackageKind, raw string) (string, error) {
	if !kind.Valid() {
		return "", errPackageKind
	}
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return "", errPackageName
	}
	if kind == store.ForgePackageKindNPM {
		if !npmPackageName.MatchString(name) {
			return "", errPackageName
		}
		return name, nil
	}
	if kind == store.ForgePackageKindPyPI {
		name = pep503Name(name)
	}
	if kind == store.ForgePackageKindMaven {
		if !mavenPackageName.MatchString(name) {
			return "", errPackageName
		}
		return name, nil
	}
	if !genericPackageName.MatchString(name) {
		return "", errPackageName
	}
	return name, nil
}

func pep503Name(s string) string {
	return pep503Sep.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
}

func sanitizePackageFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if name == "" || name == "." || name == ".." {
		return "package.bin"
	}
	return name
}

func npmPackageNameParam(c *gin.Context) string {
	s := strings.TrimPrefix(c.Param("pkg"), "/")
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "/-/"); i >= 0 {
		s = s[:i]
	}
	return s
}

func npmDigest(data []byte) (integrity, shasum string) {
	sum512 := sha512.Sum512(data)
	sum1 := sha1.Sum(data)
	return "sha512-" + base64.StdEncoding.EncodeToString(sum512[:]), hex.EncodeToString(sum1[:])
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func readFileCap(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, max+1))
}

type npmPublishDocument struct {
	Name        string                    `json:"name"`
	Versions    map[string]npmVersionMeta `json:"versions"`
	Attachments map[string]npmAttachment  `json:"_attachments"`
	DistTags    map[string]string         `json:"dist-tags"`
}

type npmVersionMeta struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type npmAttachment struct {
	ContentType string `json:"content_type"`
	Data        string `json:"data"`
}

func (d npmPublishDocument) pickVersion() (version string, meta npmVersionMeta, attName string, att npmAttachment, err error) {
	if len(d.Versions) == 0 || len(d.Attachments) == 0 {
		return "", npmVersionMeta{}, "", npmAttachment{}, errNpmManifest
	}
	version = strings.TrimSpace(d.DistTags["latest"])
	if version == "" {
		for v := range d.Versions {
			version = v
			break
		}
	}
	meta = d.Versions[version]
	if meta.Version == "" {
		meta.Version = version
	}
	if !packageVersion.MatchString(version) {
		return "", npmVersionMeta{}, "", npmAttachment{}, errNpmManifest
	}
	for name, a := range d.Attachments {
		return version, meta, name, a, nil
	}
	return "", npmVersionMeta{}, "", npmAttachment{}, errNpmManifest
}

func (a *App) handlePypiUpload(c *gin.Context) {
	if a.Packages == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "registry indisponível"})
		return
	}
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canAccessProjectFiles(user, proj, true) {
		c.JSON(http.StatusForbidden, gin.H{"error": "só developer+ publica packages"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPackageBytes+1<<20)
	fh, err := c.FormFile("content")
	if err != nil {
		fh, err = c.FormFile("file")
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo obrigatório (content)"})
		return
	}
	if fh.Size > maxPackageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 64 MiB"})
		return
	}
	name, err := normalizePackageName(store.ForgePackageKindPyPI, firstNonEmpty(c.PostForm("name"), fh.Filename))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	version := strings.TrimSpace(c.PostForm("version"))
	if !packageVersion.MatchString(version) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "versão inválida (semver)"})
		return
	}
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo inválido"})
		return
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxPackageBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "falha lendo arquivo"})
		return
	}
	if int64(len(data)) > maxPackageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 64 MiB"})
		return
	}
	filename := sanitizePackageFilename(fh.Filename)
	out, err := a.publishPackageBytes(user, proj, store.ForgePackageKindPyPI, name, version, filename, strings.TrimSpace(c.PostForm("summary")), data)
	if err != nil {
		writePackageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (a *App) handlePypiSimpleIndex(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	var rows []store.ForgePackage
	if err := a.Store.DB.Where("project_id = ? AND kind = ?", proj.ID, store.ForgePackageKindPyPI).
		Order("name asc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if wantsPypiJSON(c) {
		projects := make([]gin.H, 0, len(rows))
		for _, pkg := range rows {
			projects = append(projects, gin.H{"name": pkg.Name})
		}
		c.Header("Content-Type", "application/vnd.pypi.simple.v1+json")
		c.JSON(http.StatusOK, gin.H{"meta": gin.H{"api-version": "1.0"}, "projects": projects})
		return
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><body>\n")
	for _, pkg := range rows {
		b.WriteString(`<a href="` + html.EscapeString(pkg.Name) + `/">` + html.EscapeString(pkg.Name) + "</a>\n")
	}
	b.WriteString("</body></html>\n")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(b.String()))
}

func (a *App) handlePypiSimplePackage(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	name, err := normalizePackageName(store.ForgePackageKindPyPI, c.Param("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var pkg store.ForgePackage
	if err := a.Store.DB.Where("project_id = ? AND kind = ? AND name = ?", proj.ID, store.ForgePackageKindPyPI, name).
		First(&pkg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "package não encontrado"})
		return
	}
	var vers []store.ForgePackageVersion
	if err := a.Store.DB.Where("package_id = ?", pkg.ID).Order("created_at asc").Find(&vers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if wantsPypiJSON(c) {
		files := make([]gin.H, 0, len(vers))
		for _, ver := range vers {
			files = append(files, gin.H{
				"filename": ver.Filename,
				"url":      pypiTarballURL(a.projectRepo(proj), ver),
				"hashes":   gin.H{"sha256": ver.SHA256},
			})
		}
		c.Header("Content-Type", "application/vnd.pypi.simple.v1+json")
		c.JSON(http.StatusOK, gin.H{"meta": gin.H{"api-version": "1.0"}, "name": pkg.Name, "files": files})
		return
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><body>\n")
	for _, ver := range vers {
		url := html.EscapeString(pypiTarballURL(a.projectRepo(proj), ver))
		b.WriteString(`<a href="` + url + `">` + html.EscapeString(ver.Filename) + "</a>\n")
	}
	b.WriteString("</body></html>\n")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(b.String()))
}

func pypiTarballURL(slug string, ver store.ForgePackageVersion) string {
	return gitCloneHost + "/api/projects/" + slug + "/packages/" + strconv.FormatUint(uint64(ver.ID), 10) + "/download#sha256=" + ver.SHA256
}

func wantsPypiJSON(c *gin.Context) bool {
	return strings.Contains(strings.ToLower(c.GetHeader("Accept")), "application/vnd.pypi.simple.v1+json")
}
