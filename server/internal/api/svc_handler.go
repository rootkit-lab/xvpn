package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/provision"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const svcAgentURLHint = "http://10.66.66.1:8080"

type serviceJSON struct {
	ID           uint                `json:"id"`
	Slug         string              `json:"slug"`
	Kind         store.ServiceKind   `json:"kind"`
	ProjectSlug  string              `json:"project_slug,omitempty"`
	Host         store.ServiceHost   `json:"host"`
	MeshServerID *uint               `json:"mesh_server_id,omitempty"`
	MeshHostname string              `json:"mesh_hostname,omitempty"`
	Bind         store.ServiceBind   `json:"bind"`
	Listen       string              `json:"listen"`
	Port         uint16              `json:"port"`
	Hostname     string              `json:"hostname,omitempty"`
	Endpoint     string              `json:"endpoint"`
	Status       store.ServiceStatus `json:"status"`
	Error        string              `json:"error,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	Password     string              `json:"password,omitempty"`
}

type createServiceRequest struct {
	Slug         string   `json:"slug"`
	Kind         string   `json:"kind"`
	ProjectSlug  string   `json:"project_slug"`
	Host         string   `json:"host"`
	MeshServerID *uint    `json:"mesh_server_id"`
	Bind         string   `json:"bind"`
	Port         uint16   `json:"port"`
	Backends     []string `json:"backends"`
}

type svcAgentStatusRequest struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type svcDesiredJSON struct {
	ID       uint     `json:"id"`
	Slug     string   `json:"slug"`
	Kind     string   `json:"kind"`
	Bind     string   `json:"bind"`
	Port     uint16   `json:"port"`
	Password string   `json:"password"`
	Backends []string `json:"backends,omitempty"`
	Action   string   `json:"action"`
}

func (a *App) serviceJSON(row store.ServiceInstance, password string) serviceJSON {
	out := serviceJSON{
		ID: row.ID, Slug: row.Slug, Kind: row.Kind, Host: row.Host,
		MeshServerID: row.MeshServerID, Bind: row.Bind, Port: row.Port,
		Status: row.Status, Error: row.Error, CreatedAt: row.CreatedAt,
		Password: password,
	}
	listen, _ := a.serviceListenIP(row)
	out.Listen = listen
	if row.Bind == store.ServiceBindWG0 && listen != "" {
		out.Hostname = store.ServiceHostname(row.Slug)
	}
	out.Endpoint = serviceEndpoint(row.Kind, out.Hostname, listen, row.Port)
	if row.ProjectID != nil {
		var p store.Project
		if err := a.Store.DB.First(&p, *row.ProjectID).Error; err == nil {
			out.ProjectSlug = a.projectRepo(p)
		}
	}
	if row.MeshServerID != nil {
		var s store.MeshServer
		if err := a.Store.DB.First(&s, *row.MeshServerID).Error; err == nil {
			out.MeshHostname = s.Hostname
		}
	}
	return out
}

func serviceEndpoint(kind store.ServiceKind, hostname, listen string, port uint16) string {
	host := hostname
	if host == "" {
		host = listen
	}
	if host == "" {
		return ""
	}
	switch kind {
	case store.ServiceKindRedis:
		return "redis://" + net.JoinHostPort(host, strconv.Itoa(int(port)))
	case store.ServiceKindMongo:
		return "mongodb://" + net.JoinHostPort(host, strconv.Itoa(int(port)))
	case store.ServiceKindRabbit:
		return "amqp://" + net.JoinHostPort(host, strconv.Itoa(int(port)))
	default:
		return "http://" + net.JoinHostPort(host, strconv.Itoa(int(port)))
	}
}

func (a *App) serviceListenIP(row store.ServiceInstance) (string, error) {
	if row.Bind == store.ServiceBindLoopback {
		return "127.0.0.1", nil
	}
	if row.Host == store.ServiceHostLocal {
		return "10.66.66.1", nil
	}
	if row.MeshServerID == nil {
		return "", errors.New("peer da malha obrigatório")
	}
	var s store.MeshServer
	if err := a.Store.DB.First(&s, *row.MeshServerID).Error; err != nil {
		return "", errors.New("servidor não encontrado")
	}
	ip := strings.TrimSpace(s.WgIP)
	if ip == "" {
		return "", errors.New("peer sem IP da malha")
	}
	return ip, nil
}

func (a *App) handleListServices(c *gin.Context) {
	var rows []store.ServiceInstance
	q := a.Store.DB.Order("id DESC")
	if slug := strings.TrimSpace(c.Query("project")); slug != "" {
		var p store.Project
		if err := a.Store.DB.Where("slug = ?", slug).First(&p).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"items": []serviceJSON{}})
			return
		}
		q = q.Where("project_id = ?", p.ID)
	}
	if err := q.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]serviceJSON, 0, len(rows))
	for _, row := range rows {
		items = append(items, a.serviceJSON(row, ""))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleListProjectServices(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	var rows []store.ServiceInstance
	if err := a.Store.DB.Where("project_id = ?", proj.ID).Order("id DESC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]serviceJSON, 0, len(rows))
	for _, row := range rows {
		items = append(items, a.serviceJSON(row, ""))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) loadService(c *gin.Context) (store.ServiceInstance, bool) {
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	var row store.ServiceInstance
	if err := a.Store.DB.Where("slug = ?", slug).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "serviço não encontrado"})
			return store.ServiceInstance{}, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return store.ServiceInstance{}, false
	}
	return row, true
}

func (a *App) handleGetService(c *gin.Context) {
	row, ok := a.loadService(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, a.serviceJSON(row, ""))
}

func (a *App) handleCreateService(c *gin.Context) {
	var req createServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if !store.ValidServiceSlug(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug inválido (2–20, [a-z0-9-])"})
		return
	}
	kind := store.ServiceKind(strings.ToLower(strings.TrimSpace(req.Kind)))
	if !kind.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind deve ser mongo, redis, rabbitmq ou lb"})
		return
	}
	host := store.ServiceHost(strings.ToLower(strings.TrimSpace(req.Host)))
	if host == "" {
		host = store.ServiceHostLocal
	}
	if !host.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host deve ser local ou mesh"})
		return
	}
	bind := store.ServiceBind(strings.ToLower(strings.TrimSpace(req.Bind)))
	if bind == "" {
		bind = store.ServiceBindWG0
	}
	if !bind.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bind deve ser wg0 ou loopback"})
		return
	}
	port := req.Port
	if port == 0 {
		port = kind.DefaultPort()
	}
	if kind == store.ServiceKindMongo && port == 27017 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mongo gerenciado não usa 27017 (control-plane intocável)"})
		return
	}
	if reason, ok := reservedServicePort(port); ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "porta " + strconv.Itoa(int(port)) + " reservada (" + reason + ")"})
		return
	}
	row := store.ServiceInstance{
		Slug: slug, Kind: kind, Host: host, Bind: bind, Port: port,
		Status: store.SvcPending, Backends: req.Backends,
	}
	if host == store.ServiceHostMesh {
		if req.MeshServerID == nil || *req.MeshServerID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "mesh_server_id obrigatório"})
			return
		}
		var srv store.MeshServer
		if err := a.Store.DB.First(&srv, *req.MeshServerID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "servidor não encontrado"})
			return
		}
		if srv.Role == store.ServerRoleControl || srv.Role == store.ServerRoleExternal {
			c.JSON(http.StatusBadRequest, gin.H{"error": "use host=local no control-plane; hosts external não orquestram"})
			return
		}
		if bind == store.ServiceBindWG0 && strings.TrimSpace(srv.WgIP) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "peer sem IP da malha"})
			return
		}
		row.MeshServerID = req.MeshServerID
	}
	if projSlug := strings.ToLower(strings.TrimSpace(req.ProjectSlug)); projSlug != "" {
		org, slug, ok := strings.Cut(projSlug, "/")
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "projeto deve ser org/slug"})
			return
		}
		p, found := a.findProject(org, slug)
		if !found {
			c.JSON(http.StatusBadRequest, gin.H{"error": "projeto não encontrado"})
			return
		}
		row.ProjectID = &p.ID
	}
	if kind == store.ServiceKindLB {
		if len(req.Backends) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "lb exige backends"})
			return
		}
	}
	if err := a.assertPortFree(row); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	plain, err := auth.GenerateRandomPassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	row.AuthSecret = plain
	if err := a.Store.DB.Create(&row).Error; err != nil {
		if isUniqueErr(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "já existe um serviço com esse slug"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.upsertServiceDNS(c.Request.Context(), row)
	if row.Host == store.ServiceHostLocal {
		a.applyLocalService(c.Request.Context(), &row, "apply")
	}
	_ = a.Store.LogAudit(callerUsername(c), "svc.create", row.Slug)
	c.JSON(http.StatusCreated, a.serviceJSON(row, plain))
}

func reservedServicePort(port uint16) (string, bool) {
	reason, ok := map[uint16]string{
		22: "ssh", 53: "dns", 80: "http", 139: "nmbd", 443: "https",
		445: "samba", 8080: "xvpn-api", 27017: "control-plane-mongo", 51820: "wireguard",
	}[port]
	return reason, ok
}

func (a *App) assertPortFree(row store.ServiceInstance) error {
	q := a.Store.DB.Model(&store.ServiceInstance{}).
		Where("port = ? AND status <> ? AND host = ?", row.Port, store.SvcStopped, row.Host)
	if row.Host == store.ServiceHostMesh && row.MeshServerID != nil {
		q = q.Where("mesh_server_id = ?", *row.MeshServerID)
	}
	if row.ID != 0 {
		q = q.Where("id <> ?", row.ID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return errors.New("erro interno")
	}
	if n > 0 {
		return errors.New("porta já em uso neste host")
	}
	return nil
}

func (a *App) handleApplyService(c *gin.Context) {
	row, ok := a.loadService(c)
	if !ok {
		return
	}
	if row.Host == store.ServiceHostLocal {
		a.applyLocalService(c.Request.Context(), &row, "apply")
	} else {
		row.Status = store.SvcPending
		row.Error = ""
		_ = a.Store.DB.Save(&row).Error
	}
	_ = a.upsertServiceDNS(c.Request.Context(), row)
	_ = a.Store.LogAudit(callerUsername(c), "svc.apply", row.Slug)
	c.JSON(http.StatusOK, a.serviceJSON(row, ""))
}

func (a *App) handleStopService(c *gin.Context) {
	row, ok := a.loadService(c)
	if !ok {
		return
	}
	if row.Host == store.ServiceHostLocal {
		a.applyLocalService(c.Request.Context(), &row, "stop")
	} else {
		row.Status = store.SvcStopped
		_ = a.Store.DB.Save(&row).Error
	}
	_ = a.removeServiceDNS(c.Request.Context(), row)
	_ = a.Store.LogAudit(callerUsername(c), "svc.stop", row.Slug)
	c.JSON(http.StatusOK, a.serviceJSON(row, ""))
}

func (a *App) handleRotateServiceSecret(c *gin.Context) {
	row, ok := a.loadService(c)
	if !ok {
		return
	}
	plain, err := auth.GenerateRandomPassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	row.AuthSecret = plain
	if err := a.Store.DB.Save(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if row.Host == store.ServiceHostLocal {
		a.applyLocalService(c.Request.Context(), &row, "apply")
	} else {
		row.Status = store.SvcPending
		row.Error = ""
		_ = a.Store.DB.Save(&row).Error
	}
	_ = a.Store.LogAudit(callerUsername(c), "svc.rotate", row.Slug)
	c.JSON(http.StatusOK, a.serviceJSON(row, plain))
}

func (a *App) handleDeleteService(c *gin.Context) {
	row, ok := a.loadService(c)
	if !ok {
		return
	}
	if row.Host == store.ServiceHostLocal {
		a.applyLocalService(c.Request.Context(), &row, "stop")
	}
	_ = a.removeServiceDNS(c.Request.Context(), row)
	if err := a.Store.DB.Delete(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "svc.delete", row.Slug)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) applyLocalService(ctx context.Context, row *store.ServiceInstance, action string) {
	if a.UserProvisioner == nil {
		row.Status = store.SvcError
		row.Error = "provisionamento privilegiado não configurado"
		_ = a.Store.DB.Save(row).Error
		return
	}
	listen, err := a.serviceListenIP(*row)
	if err != nil {
		row.Status = store.SvcError
		row.Error = err.Error()
		_ = a.Store.DB.Save(row).Error
		return
	}
	spec := provision.SvcSpec{
		Action: action, Slug: row.Slug, Kind: string(row.Kind),
		Bind: listen, Port: row.Port, Password: row.AuthSecret, Backends: row.Backends,
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		row.Status = store.SvcError
		row.Error = "erro interno"
		_ = a.Store.DB.Save(row).Error
		return
	}
	if err := a.UserProvisioner.ApplySvc(ctx, string(raw)); err != nil {
		row.Status = store.SvcError
		row.Error = provisionerErrMsg(err)
		_ = a.Store.DB.Save(row).Error
		return
	}
	if action == "stop" {
		row.Status = store.SvcStopped
	} else {
		row.Status = store.SvcReady
	}
	row.Error = ""
	_ = a.Store.DB.Save(row).Error
}

func (a *App) upsertServiceDNS(ctx context.Context, row store.ServiceInstance) error {
	if row.Bind != store.ServiceBindWG0 {
		return nil
	}
	ip, err := a.serviceListenIP(row)
	if err != nil {
		return err
	}
	host := store.ServiceHostname(row.Slug)
	var rec store.DNSRecord
	err = a.Store.DB.Where("hostname = ?", host).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		rec = store.DNSRecord{
			Hostname: host, IPv4: ip, Enabled: true,
			Comment: "serviço gerenciado " + string(row.Kind),
		}
		if err := a.Store.DB.Create(&rec).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		rec.IPv4 = ip
		rec.Enabled = true
		if err := a.Store.DB.Save(&rec).Error; err != nil {
			return err
		}
	}
	return a.pushIntranetDNS(ctx)
}

func (a *App) removeServiceDNS(ctx context.Context, row store.ServiceInstance) error {
	host := store.ServiceHostname(row.Slug)
	var rec store.DNSRecord
	if err := a.Store.DB.Where("hostname = ? AND system = ?", host, false).First(&rec).Error; err != nil {
		return nil
	}
	if err := a.Store.DB.Delete(&rec).Error; err != nil {
		return err
	}
	return a.pushIntranetDNS(ctx)
}

func (a *App) handleIssueAgentToken(c *gin.Context) {
	s, ok := a.loadMeshServer(c)
	if !ok {
		return
	}
	if s.Role != store.ServerRoleMesh && s.Role != store.ServerRoleRunner {
		c.JSON(http.StatusBadRequest, gin.H{"error": "só host mesh ou runner"})
		return
	}
	plain, err := auth.GenerateRandomPassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	hash, err := auth.HashPassword(plain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	s.AgentTokenHash = hash
	if err := a.Store.DB.Save(&s).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.agent-token", s.Hostname)
	c.JSON(http.StatusOK, gin.H{"agent_token": plain, "svc_url": svcAgentURLHint})
}

func (a *App) agentByToken(token string) (store.MeshServer, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return store.MeshServer{}, false
	}
	var rows []store.MeshServer
	if err := a.Store.DB.Where("agent_token_hash <> '' AND role IN ?", []string{store.ServerRoleMesh, store.ServerRoleRunner}).Find(&rows).Error; err != nil {
		return store.MeshServer{}, false
	}
	for _, s := range rows {
		ok, err := auth.VerifyPassword(s.AgentTokenHash, token)
		if err == nil && ok {
			return s, true
		}
	}
	return store.MeshServer{}, false
}

func (a *App) authenticateSvcAgent(c *gin.Context) (store.MeshServer, bool) {
	raw := strings.TrimSpace(c.GetHeader("Authorization"))
	token := ""
	if strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		token = strings.TrimSpace(raw[7:])
	}
	srv, ok := a.agentByToken(token)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
		return store.MeshServer{}, false
	}
	ip := net.ParseIP(c.RemoteIP())
	if ip == nil || !a.runnerIPOK(srv, ip) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "agent fora do peer esperado"})
		return store.MeshServer{}, false
	}
	return srv, true
}

func (a *App) handleSvcDesired(c *gin.Context) {
	srv, ok := a.authenticateSvcAgent(c)
	if !ok {
		return
	}
	var rows []store.ServiceInstance
	if err := a.Store.DB.Where("host = ? AND mesh_server_id = ?", store.ServiceHostMesh, srv.ID).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]svcDesiredJSON, 0, len(rows))
	for _, row := range rows {
		listen, err := a.serviceListenIP(row)
		if err != nil {
			continue
		}
		action := "apply"
		if row.Status == store.SvcStopped {
			action = "stop"
		}
		items = append(items, svcDesiredJSON{
			ID: row.ID, Slug: row.Slug, Kind: string(row.Kind),
			Bind: listen, Port: row.Port, Password: row.AuthSecret,
			Backends: row.Backends, Action: action,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleSvcAgentStatus(c *gin.Context) {
	srv, ok := a.authenticateSvcAgent(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var row store.ServiceInstance
	if err := a.Store.DB.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "serviço não encontrado"})
		return
	}
	if row.Host != store.ServiceHostMesh || row.MeshServerID == nil || *row.MeshServerID != srv.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "serviço de outro host"})
		return
	}
	var req svcAgentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	st := store.ServiceStatus(strings.TrimSpace(req.Status))
	if !st.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status inválido"})
		return
	}
	row.Status = st
	row.Error = strings.TrimSpace(req.Error)
	if err := a.Store.DB.Save(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
