package api

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const mavenVersionSep = "::"

func packageStoredVersion(kind store.ForgePackageKind, version, filename string) string {
	if kind == store.ForgePackageKindMaven && filename != "" {
		return version + mavenVersionSep + filename
	}
	return version
}

func packageDisplayVersion(kind store.ForgePackageKind, stored string) string {
	if kind == store.ForgePackageKindMaven {
		if i := strings.Index(stored, mavenVersionSep); i >= 0 {
			return stored[:i]
		}
	}
	return stored
}

type mavenPath struct {
	Group    string
	Artifact string
	Version  string
	Filename string
	Meta     bool
	Checksum string // md5|sha1|sha256 of the sibling file
}

func parseMavenPath(raw string) (mavenPath, bool) {
	s := strings.Trim(strings.TrimPrefix(raw, "/"), "/")
	if s == "" {
		return mavenPath{}, false
	}
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return mavenPath{}, false
	}
	last := parts[len(parts)-1]
	check := ""
	base := last
	for _, suf := range []string{".md5", ".sha1", ".sha256"} {
		if strings.HasSuffix(strings.ToLower(last), suf) {
			check = strings.TrimPrefix(suf, ".")
			base = last[:len(last)-len(suf)]
			break
		}
	}
	if strings.HasPrefix(base, "maven-metadata") {
		if len(parts) < 2 {
			return mavenPath{}, false
		}
		artifact := parts[len(parts)-2]
		groupParts := parts[:len(parts)-2]
		if artifact == "" || len(groupParts) == 0 {
			return mavenPath{}, false
		}
		return mavenPath{
			Group:    strings.Join(groupParts, "."),
			Artifact: artifact,
			Meta:     true,
			Checksum: check,
		}, true
	}
	if len(parts) < 3 {
		return mavenPath{}, false
	}
	version := parts[len(parts)-2]
	artifact := parts[len(parts)-3]
	groupParts := parts[:len(parts)-3]
	if artifact == "" || version == "" || len(groupParts) == 0 || base == "" {
		return mavenPath{}, false
	}
	return mavenPath{
		Group:    strings.Join(groupParts, "."),
		Artifact: artifact,
		Version:  version,
		Filename: base,
		Checksum: check,
	}, true
}

func mavenPackageNameOf(p mavenPath) string {
	return strings.ToLower(p.Group + ":" + p.Artifact)
}

func (a *App) handleMavenPut(c *gin.Context) {
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
	p, ok := parseMavenPath(c.Param("filepath"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "caminho Maven inválido"})
		return
	}
	if p.Meta || p.Checksum != "" {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		_, _ = io.Copy(io.Discard, c.Request.Body)
		c.Status(http.StatusCreated)
		return
	}
	if !packageVersion.MatchString(p.Version) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "versão inválida (semver)"})
		return
	}
	name, err := normalizePackageName(store.ForgePackageKindMaven, mavenPackageNameOf(p))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPackageBytes+1)
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, maxPackageBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "falha lendo arquivo"})
		return
	}
	if int64(len(data)) > maxPackageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 64 MiB"})
		return
	}
	filename := sanitizePackageFilename(p.Filename)
	_, err = a.publishPackageBytes(user, proj, store.ForgePackageKindMaven, name, p.Version, filename, "", data)
	if err != nil {
		writePackageError(c, err)
		return
	}
	c.Status(http.StatusCreated)
}

func (a *App) handleMavenGet(c *gin.Context) {
	if a.Packages == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "registry indisponível"})
		return
	}
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	p, ok := parseMavenPath(c.Param("filepath"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "não encontrado"})
		return
	}
	name, err := normalizePackageName(store.ForgePackageKindMaven, mavenPackageNameOf(p))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "não encontrado"})
		return
	}
	var pkg store.ForgePackage
	if err := a.Store.DB.Where("project_id = ? AND kind = ? AND name = ?", proj.ID, store.ForgePackageKindMaven, name).
		First(&pkg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "package não encontrado"})
		return
	}
	if p.Meta {
		a.writeMavenMetadata(c, pkg, p)
		return
	}
	var vers []store.ForgePackageVersion
	if err := a.Store.DB.Where("package_id = ?", pkg.ID).Find(&vers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var ver *store.ForgePackageVersion
	want := packageStoredVersion(store.ForgePackageKindMaven, p.Version, p.Filename)
	for i := range vers {
		if vers[i].Filename == p.Filename && (vers[i].Version == want || vers[i].Version == p.Version) {
			ver = &vers[i]
			break
		}
	}
	if ver == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "versão não encontrada"})
		return
	}
	abs, err := a.Packages.AbsPath(ver.StoragePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if p.Checksum != "" {
		raw, err := readFileCap(abs, maxPackageBytes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(fileChecksum(raw, p.Checksum)))
		return
	}
	c.FileAttachment(abs, ver.Filename)
}

func (a *App) writeMavenMetadata(c *gin.Context, pkg store.ForgePackage, p mavenPath) {
	var vers []store.ForgePackageVersion
	if err := a.Store.DB.Where("package_id = ?", pkg.ID).Order("created_at asc").Find(&vers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	seen := map[string]struct{}{}
	var versions []string
	for _, ver := range vers {
		v := packageDisplayVersion(store.ForgePackageKindMaven, ver.Version)
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		versions = append(versions, v)
	}
	if len(versions) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "package não encontrado"})
		return
	}
	latest := versions[len(versions)-1]
	release := latest
	for i := len(versions) - 1; i >= 0; i-- {
		if !strings.Contains(strings.ToUpper(versions[i]), "SNAPSHOT") {
			release = versions[i]
			break
		}
	}
	type versioning struct {
		Latest      string   `xml:"latest"`
		Release     string   `xml:"release"`
		Versions    []string `xml:"versions>version"`
		LastUpdated string   `xml:"lastUpdated"`
	}
	type metadata struct {
		XMLName    xml.Name   `xml:"metadata"`
		GroupID    string     `xml:"groupId"`
		ArtifactID string     `xml:"artifactId"`
		Versioning versioning `xml:"versioning"`
	}
	doc := metadata{
		GroupID:    p.Group,
		ArtifactID: p.Artifact,
		Versioning: versioning{
			Latest:      latest,
			Release:     release,
			Versions:    versions,
			LastUpdated: vers[len(vers)-1].CreatedAt.UTC().Format("20060102150405"),
		},
	}
	raw, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	body := append([]byte(xml.Header), raw...)
	if p.Checksum != "" {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(fileChecksum(body, p.Checksum)))
		return
	}
	c.Data(http.StatusOK, "application/xml; charset=utf-8", body)
}

func fileChecksum(data []byte, kind string) string {
	switch strings.ToLower(kind) {
	case "md5":
		sum := md5.Sum(data)
		return hex.EncodeToString(sum[:])
	case "sha1":
		sum := sha1.Sum(data)
		return hex.EncodeToString(sum[:])
	default:
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
}

func (a *App) handleNugetIndex(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	base := gitCloneHost + "/api/packages/" + a.projectRepo(proj) + "/nuget"
	c.JSON(http.StatusOK, gin.H{
		"version": "3.0.0",
		"resources": []gin.H{
			{"@id": base, "@type": "PackagePublish/2.0.0"},
			{"@id": base + "/flat/", "@type": "PackageBaseAddress/3.0.0"},
		},
	})
}

func (a *App) handleNugetPush(c *gin.Context) {
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
	var data []byte
	var filename string
	if fh, err := c.FormFile("package"); err == nil {
		filename = fh.Filename
		src, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo inválido"})
			return
		}
		defer src.Close()
		data, err = io.ReadAll(io.LimitReader(src, maxPackageBytes+1))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "falha lendo arquivo"})
			return
		}
	} else if fh, err := c.FormFile("file"); err == nil {
		filename = fh.Filename
		src, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo inválido"})
			return
		}
		defer src.Close()
		data, err = io.ReadAll(io.LimitReader(src, maxPackageBytes+1))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "falha lendo arquivo"})
			return
		}
	} else {
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxPackageBytes+1))
		if err != nil || len(raw) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo obrigatório"})
			return
		}
		data = raw
		filename = strings.TrimSpace(c.Query("filename"))
		if filename == "" {
			filename = "package.nupkg"
		}
	}
	if int64(len(data)) > maxPackageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 64 MiB"})
		return
	}
	filename = sanitizePackageFilename(filename)
	name := firstNonEmpty(c.Query("id"), c.PostForm("id"))
	version := firstNonEmpty(c.Query("version"), c.PostForm("version"))
	if parsedName, parsedVer, ok := parseTrailingSemver(strings.TrimSuffix(filename, ".nupkg")); ok {
		if name == "" {
			name = parsedName
		}
		if version == "" {
			version = parsedVer
		}
	}
	name, err := normalizePackageName(store.ForgePackageKindNuGet, name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !packageVersion.MatchString(version) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "versão inválida (semver)"})
		return
	}
	out, err := a.publishPackageBytes(user, proj, store.ForgePackageKindNuGet, name, version, filename, "", data)
	if err != nil {
		writePackageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (a *App) handleNugetVersions(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	name, err := normalizePackageName(store.ForgePackageKindNuGet, c.Param("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var pkg store.ForgePackage
	if err := a.Store.DB.Where("project_id = ? AND kind = ? AND name = ?", proj.ID, store.ForgePackageKindNuGet, name).
		First(&pkg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "package não encontrado"})
		return
	}
	var vers []store.ForgePackageVersion
	if err := a.Store.DB.Where("package_id = ?", pkg.ID).Order("created_at asc").Find(&vers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	ids := make([]string, 0, len(vers))
	for _, ver := range vers {
		ids = append(ids, ver.Version)
	}
	c.JSON(http.StatusOK, gin.H{"versions": ids})
}

func (a *App) handleNugetDownload(c *gin.Context) {
	if a.Packages == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "registry indisponível"})
		return
	}
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	name, err := normalizePackageName(store.ForgePackageKindNuGet, c.Param("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	version := strings.TrimSpace(c.Param("version"))
	if !packageVersion.MatchString(version) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "versão inválida (semver)"})
		return
	}
	var pkg store.ForgePackage
	if err := a.Store.DB.Where("project_id = ? AND kind = ? AND name = ?", proj.ID, store.ForgePackageKindNuGet, name).
		First(&pkg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "package não encontrado"})
		return
	}
	var ver store.ForgePackageVersion
	if err := a.Store.DB.Where("package_id = ? AND version = ?", pkg.ID, version).First(&ver).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "versão não encontrada"})
		return
	}
	abs, err := a.Packages.AbsPath(ver.StoragePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.FileAttachment(abs, ver.Filename)
}

func (a *App) handleRubygemsPush(c *gin.Context) {
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
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPackageBytes+1)
	filename := ""
	var data []byte
	if fh, err := c.FormFile("file"); err == nil {
		filename = fh.Filename
		src, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo inválido"})
			return
		}
		defer src.Close()
		data, err = io.ReadAll(io.LimitReader(src, maxPackageBytes+1))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "falha lendo arquivo"})
			return
		}
	} else if fh, err := c.FormFile("gem"); err == nil {
		filename = fh.Filename
		src, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo inválido"})
			return
		}
		defer src.Close()
		data, err = io.ReadAll(io.LimitReader(src, maxPackageBytes+1))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "falha lendo arquivo"})
			return
		}
	} else {
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxPackageBytes+1))
		if err != nil || len(raw) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo obrigatório"})
			return
		}
		data = raw
		filename = strings.TrimSpace(c.Query("filename"))
		if filename == "" {
			filename = "package.gem"
		}
	}
	if int64(len(data)) > maxPackageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 64 MiB"})
		return
	}
	filename = sanitizePackageFilename(filename)
	name := firstNonEmpty(c.Query("name"), c.PostForm("name"))
	version := firstNonEmpty(c.Query("version"), c.PostForm("version"))
	if parsedName, parsedVer, ok := parseTrailingSemver(strings.TrimSuffix(filename, ".gem")); ok {
		if name == "" {
			name = parsedName
		}
		if version == "" {
			version = parsedVer
		}
	}
	name, err := normalizePackageName(store.ForgePackageKindRubyGems, name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !packageVersion.MatchString(version) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "versão inválida (semver)"})
		return
	}
	out, err := a.publishPackageBytes(user, proj, store.ForgePackageKindRubyGems, name, version, filename, "", data)
	if err != nil {
		writePackageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (a *App) handleRubygemsDownload(c *gin.Context) {
	if a.Packages == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "registry indisponível"})
		return
	}
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	filename := sanitizePackageFilename(c.Param("filename"))
	name, version, ok := parseTrailingSemver(strings.TrimSuffix(filename, ".gem"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ficheiro .gem inválido"})
		return
	}
	name, err := normalizePackageName(store.ForgePackageKindRubyGems, name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var pkg store.ForgePackage
	if err := a.Store.DB.Where("project_id = ? AND kind = ? AND name = ?", proj.ID, store.ForgePackageKindRubyGems, name).
		First(&pkg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "package não encontrado"})
		return
	}
	var ver store.ForgePackageVersion
	if err := a.Store.DB.Where("package_id = ? AND version = ?", pkg.ID, version).First(&ver).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "versão não encontrada"})
		return
	}
	abs, err := a.Packages.AbsPath(ver.StoragePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.FileAttachment(abs, ver.Filename)
}

func parseTrailingSemver(base string) (name, version string, ok bool) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", "", false
	}
	for i := 0; i < len(base); i++ {
		if base[i] != '-' && base[i] != '.' {
			continue
		}
		cand := strings.TrimLeft(base[i+1:], "")
		if packageVersion.MatchString(cand) {
			n := strings.Trim(base[:i], "-.")
			if n == "" {
				continue
			}
			return n, cand, true
		}
	}
	return "", "", false
}
