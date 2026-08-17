package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/bitlaunch"
	"github.com/rootkit-lab/xvpn/server/internal/corpdns"
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
)

const (
	controlPlaneIPv4 = "206.189.224.72"
	controlPlaneWgIP = "10.66.66.1"
	controlHostname  = "control"
)

type BitLaunchAPI interface {
	List() ([]bitlaunch.Server, error)
	Create(bitlaunch.CreateOpts) (bitlaunch.Server, error)
	Destroy(id string) error
	Rebuild(id string, opts bitlaunch.RebuildOpts) error
}

type meshServerResponse struct {
	ID            uint      `json:"id"`
	BitLaunchID   string    `json:"bitlaunch_id"`
	Name          string    `json:"name"`
	Hostname      string    `json:"hostname"`
	Role          string    `json:"role"`
	IPv4          string    `json:"ipv4"`
	WgIP          string    `json:"wg_ip"`
	Region        string    `json:"region"`
	Size          string    `json:"size"`
	Status        string    `json:"status"`
	Labels        []string  `json:"labels"`
	GroupID       *uint     `json:"group_id,omitempty"`
	DeviceID      *uint     `json:"device_id,omitempty"`
	AccessUserIDs []uint    `json:"access_user_ids,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	EnrollToken   string    `json:"enroll_token,omitempty"`
}

type createMeshServerRequest struct {
	Name        string   `json:"name"`
	Hostname    string   `json:"hostname"`
	HostID      int      `json:"host_id"`
	HostImageID string   `json:"host_image_id"`
	SizeID      string   `json:"size_id"`
	RegionID    string   `json:"region_id"`
	SSHKeys     []string `json:"ssh_keys"`
	Labels      []string `json:"labels"`
	Role        string   `json:"role"`
}

type updateMeshServerRequest struct {
	Labels  *[]string `json:"labels"`
	Role    *string   `json:"role"`
	Name    *string   `json:"name"`
	GroupID *uint     `json:"group_id"`
}

type createServerGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type serverGroupResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type setServerAccessRequest struct {
	UserIDs []uint `json:"user_ids"`
}

type meshEnrollRequest struct {
	EnrollToken string `json:"enroll_token"`
	PublicKey   string `json:"public_key"`
	Hostname    string `json:"hostname"`
}

func (a *App) meshServerJSON(s store.MeshServer, includeToken bool) meshServerResponse {
	labels := s.Labels
	if labels == nil {
		labels = []string{}
	}
	out := meshServerResponse{
		ID: s.ID, BitLaunchID: s.BitLaunchID, Name: s.Name, Hostname: s.Hostname,
		Role: s.Role, IPv4: s.IPv4, WgIP: s.WgIP, Region: s.Region, Size: s.Size,
		Status: s.Status, Labels: labels, GroupID: s.GroupID, DeviceID: s.DeviceID,
		CreatedAt: s.CreatedAt,
	}
	if includeToken && s.EnrollToken != "" {
		out.EnrollToken = s.EnrollToken
	}
	return out
}

func (a *App) handleListMeshServers(c *gin.Context) {
	var rows []store.MeshServer
	if err := a.Store.DB.Order("id").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]meshServerResponse, 0, len(rows))
	for _, s := range rows {
		items = append(items, a.meshServerJSON(s, false))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "bitlaunch": a.BitLaunch != nil})
}

func (a *App) handleGetMeshServer(c *gin.Context) {
	s, ok := a.loadMeshServer(c)
	if !ok {
		return
	}
	out := a.meshServerJSON(s, false)
	out.AccessUserIDs = a.serverAccessUserIDs(s.ID, nil)
	c.JSON(http.StatusOK, out)
}

func (a *App) handleImportMeshServers(c *gin.Context) {
	if err := a.upsertControlPlaneServer(callerUserID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	imported := 1
	if a.BitLaunch != nil {
		remote, err := a.BitLaunch.List()
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "falha ao listar BitLaunch"})
			return
		}
		for _, r := range remote {
			if err := a.upsertBitLaunchServer(r, callerUserID(c)); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
				return
			}
			imported++
		}
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.import", strconv.Itoa(imported))
	_ = a.pushIntranetDNS(c.Request.Context())
	a.handleListMeshServers(c)
}

func (a *App) handleCreateMeshServer(c *gin.Context) {
	if a.BitLaunch == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "XVPN_BITLAUNCH_TOKEN não configurado"})
		return
	}
	var req createMeshServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	host := strings.ToLower(strings.TrimSpace(req.Hostname))
	if host == "" {
		host = strings.ToLower(strings.TrimSpace(req.Name))
	}
	if !store.ValidProjectSlug(host) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hostname inválido (2–20, a-z 0-9 hífen)"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = host
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = store.ServerRoleMesh
	}
	if role == store.ServerRoleControl {
		c.JSON(http.StatusBadRequest, gin.H{"error": "o node de controle não se cria por aqui"})
		return
	}
	if req.HostID == 0 || req.HostImageID == "" || req.SizeID == "" || req.RegionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_id, host_image_id, size_id e region_id são obrigatórios"})
		return
	}
	token, err := generateInviteToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	exp := time.Now().Add(2 * time.Hour)
	script := meshCloudInit(token, host)
	created, err := a.BitLaunch.Create(bitlaunch.CreateOpts{
		Name: name, HostID: req.HostID, HostImageID: req.HostImageID,
		SizeID: req.SizeID, RegionID: req.RegionID, SSHKeys: req.SSHKeys, InitScript: script,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "falha ao criar no BitLaunch"})
		return
	}
	row := store.MeshServer{
		BitLaunchID: created.ID, Name: name, Hostname: host, Role: role,
		IPv4: created.IPv4, Region: created.Region, Size: created.Size,
		Image: created.Image, Status: created.Status, Labels: req.Labels,
		CreatedByUserID: callerUserID(c), EnrollToken: token, EnrollExpiresAt: &exp,
	}
	if row.Status == "" {
		row.Status = "launching"
	}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.create", host)
	c.JSON(http.StatusCreated, a.meshServerJSON(row, true))
}

func (a *App) handleUpdateMeshServer(c *gin.Context) {
	s, ok := a.loadMeshServer(c)
	if !ok {
		return
	}
	var req updateMeshServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.Labels != nil {
		s.Labels = *req.Labels
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name inválido"})
			return
		}
		s.Name = name
	}
	if req.Role != nil {
		role := strings.TrimSpace(*req.Role)
		if role != store.ServerRoleMesh && role != store.ServerRoleRunner {
			c.JSON(http.StatusBadRequest, gin.H{"error": "role inválida"})
			return
		}
		if s.Role == store.ServerRoleControl {
			c.JSON(http.StatusBadRequest, gin.H{"error": "não altera o papel do node de controle"})
			return
		}
		s.Role = role
	}
	if req.GroupID != nil {
		if *req.GroupID == 0 {
			s.GroupID = nil
		} else {
			var g store.ServerGroup
			if err := a.Store.DB.First(&g, *req.GroupID).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "grupo não encontrado"})
				return
			}
			s.GroupID = req.GroupID
		}
	}
	if err := a.Store.DB.Save(&s).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.update", s.Hostname)
	c.JSON(http.StatusOK, a.meshServerJSON(s, false))
}

func (a *App) handleDestroyMeshServer(c *gin.Context) {
	s, ok := a.loadMeshServer(c)
	if !ok {
		return
	}
	if s.Role == store.ServerRoleControl || s.WgIP == controlPlaneWgIP {
		c.JSON(http.StatusConflict, gin.H{"error": "o node de controle não se destrói por aqui"})
		return
	}
	if a.BitLaunch != nil && s.BitLaunchID != "" && !strings.HasPrefix(s.BitLaunchID, "local-") {
		if err := a.BitLaunch.Destroy(s.BitLaunchID); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "falha ao destruir no BitLaunch"})
			return
		}
	}
	if s.DeviceID != nil {
		var dev store.Device
		if err := a.Store.DB.First(&dev, *s.DeviceID).Error; err == nil {
			_ = a.WG.RemovePeer(dev.PublicKey)
			_ = a.Store.DB.Delete(&dev).Error
		}
	}
	_ = a.Store.DB.Where("hostname = ?", s.Hostname+".corp.ihuull.com").Delete(&store.DNSRecord{}).Error
	_ = a.pushIntranetDNS(c.Request.Context())
	if err := a.Store.DB.Delete(&s).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.destroy", s.Hostname)
	c.Status(http.StatusNoContent)
}

func (a *App) handleRebuildMeshServer(c *gin.Context) {
	if a.BitLaunch == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "XVPN_BITLAUNCH_TOKEN não configurado"})
		return
	}
	s, ok := a.loadMeshServer(c)
	if !ok {
		return
	}
	if s.Role == store.ServerRoleControl {
		c.JSON(http.StatusConflict, gin.H{"error": "o node de controle não se reconstrói por aqui"})
		return
	}
	var req struct {
		HostImageID      string `json:"host_image_id"`
		ImageDescription string `json:"image_description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.HostImageID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_image_id é obrigatório"})
		return
	}
	if err := a.BitLaunch.Rebuild(s.BitLaunchID, bitlaunch.RebuildOpts{
		HostImageID: req.HostImageID, ImageDescription: req.ImageDescription,
	}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "falha ao rebuild no BitLaunch"})
		return
	}
	token, err := generateInviteToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	exp := time.Now().Add(2 * time.Hour)
	s.EnrollToken = token
	s.EnrollExpiresAt = &exp
	s.Status = "rebuilding"
	s.WgIP = ""
	if err := a.Store.DB.Save(&s).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.rebuild", s.Hostname)
	c.JSON(http.StatusOK, a.meshServerJSON(s, true))
}

func (a *App) handleSetServerAccess(c *gin.Context) {
	s, ok := a.loadMeshServer(c)
	if !ok {
		return
	}
	var req setServerAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if err := a.Store.DB.Where("server_id = ?", s.ID).Delete(&store.ServerAccess{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	for _, uid := range req.UserIDs {
		var u store.User
		if err := a.Store.DB.First(&u, uid).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "usuário não encontrado"})
			return
		}
		row := store.ServerAccess{UserID: uid, ServerID: &s.ID, Role: "operator"}
		if err := a.Store.DB.Create(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.access", s.Hostname)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) handleMeshServerEnroll(c *gin.Context) {
	var req meshEnrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enroll_token e public_key são obrigatórios"})
		return
	}
	if _, err := wgtypes.ParseKey(req.PublicKey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "public_key inválida"})
		return
	}
	a.enrollMu.Lock()
	defer a.enrollMu.Unlock()

	var rows []store.MeshServer
	if err := a.Store.DB.Where("enroll_token <> ''").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var match *store.MeshServer
	for i := range rows {
		if rows[i].EnrollExpiresAt != nil && time.Now().After(*rows[i].EnrollExpiresAt) {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(rows[i].EnrollToken), []byte(req.EnrollToken)) == 1 {
			match = &rows[i]
			break
		}
	}
	if match == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "enroll_token inválido"})
		return
	}
	if match.Role == store.ServerRoleControl {
		c.JSON(http.StatusConflict, gin.H{"error": "node de controle já está na malha"})
		return
	}

	var existing store.Device
	if err := a.Store.DB.Where("public_key = ?", req.PublicKey).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "esta chave pública já está registrada"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	var devices []store.Device
	if err := a.Store.DB.Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	used := make([]string, 0, len(devices)+1)
	for _, d := range devices {
		used = append(used, d.AllowedIP)
	}
	used = append(used, controlPlaneWgIP+"/32")
	assigned, err := wireguard.AllocateIP(a.Config.WireGuardAllowedSubnet, used)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "nenhum IP disponível na sub-rede da VPN"})
		return
	}
	if strings.HasPrefix(assigned, "10.10.") || strings.HasPrefix(assigned, "10.136.") {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	ownerID := match.CreatedByUserID
	if ownerID == 0 {
		var admin store.User
		if err := a.Store.DB.Where("role = ?", store.RoleSuperAdmin).Order("id").First(&admin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		ownerID = admin.ID
	}
	dev := store.Device{
		UserID: ownerID, Name: "mesh-" + match.Hostname,
		PublicKey: req.PublicKey, AllowedIP: assigned,
	}
	if err := a.Store.DB.Create(&dev).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.WG.AddPeer(wireguard.PeerSpec{PublicKey: dev.PublicKey, AllowedIP: dev.AllowedIP}); err != nil {
		_ = a.Store.DB.Delete(&dev).Error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	wgIP := strings.TrimSuffix(assigned, "/32")
	match.DeviceID = &dev.ID
	match.WgIP = wgIP
	match.Status = "ok"
	match.EnrollToken = ""
	match.EnrollExpiresAt = nil
	if err := a.Store.DB.Save(match).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.ensureMeshDNS(match.Hostname, wgIP)
	_ = a.pushIntranetDNS(context.Background())
	_ = a.Store.LogAudit("mesh-enroll", "compute.enroll", match.Hostname)

	c.JSON(http.StatusCreated, gin.H{
		"assigned_ip":       assigned,
		"server_public_key": a.ServerPublicKey,
		"endpoint":          a.Config.WireGuardEndpoint,
		"dns":               []string{controlPlaneWgIP},
		"hostname":          match.Hostname + ".corp.ihuull.com",
	})
}

func (a *App) handleListServerGroups(c *gin.Context) {
	var rows []store.ServerGroup
	if err := a.Store.DB.Order("id").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]serverGroupResponse, 0, len(rows))
	for _, g := range rows {
		items = append(items, serverGroupResponse{
			ID: g.ID, Name: g.Name, Description: g.Description, CreatedAt: g.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleCreateServerGroup(c *gin.Context) {
	var req createServerGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name é obrigatório"})
		return
	}
	row := store.ServerGroup{Name: name, Description: strings.TrimSpace(req.Description)}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "grupo já existe"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.group.create", name)
	c.JSON(http.StatusCreated, serverGroupResponse{
		ID: row.ID, Name: row.Name, Description: row.Description, CreatedAt: row.CreatedAt,
	})
}

func (a *App) handleSetGroupAccess(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var g store.ServerGroup
	if err := a.Store.DB.First(&g, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "grupo não encontrado"})
		return
	}
	var req setServerAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if err := a.Store.DB.Where("group_id = ?", g.ID).Delete(&store.ServerAccess{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	gid := g.ID
	for _, uid := range req.UserIDs {
		var u store.User
		if err := a.Store.DB.First(&u, uid).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "usuário não encontrado"})
			return
		}
		row := store.ServerAccess{UserID: uid, GroupID: &gid, Role: "operator"}
		if err := a.Store.DB.Create(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.group.access", g.Name)
	c.JSON(http.StatusOK, gin.H{"ok": true, "access_user_ids": a.serverAccessUserIDs(0, &g.ID)})
}

func (a *App) serverAccessUserIDs(serverID uint, groupID *uint) []uint {
	var rows []store.ServerAccess
	q := a.Store.DB
	if groupID != nil {
		q = q.Where("group_id = ?", *groupID)
	} else {
		q = q.Where("server_id = ?", serverID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return []uint{}
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.UserID)
	}
	return ids
}

func (a *App) loadMeshServer(c *gin.Context) (store.MeshServer, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return store.MeshServer{}, false
	}
	var s store.MeshServer
	if err := a.Store.DB.First(&s, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "servidor não encontrado"})
		return store.MeshServer{}, false
	}
	return s, true
}

func (a *App) upsertControlPlaneServer(actorID uint) error {
	var existing store.MeshServer
	err := a.Store.DB.Where("role = ?", store.ServerRoleControl).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	row := store.MeshServer{
		BitLaunchID: "local-control", Name: "VPS ihuull", Hostname: controlHostname,
		Role: store.ServerRoleControl, IPv4: controlPlaneIPv4, WgIP: controlPlaneWgIP,
		Status: "ok", Labels: []string{"control"}, CreatedByUserID: actorID,
	}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		return err
	}
	return a.ensureMeshDNS(controlHostname, controlPlaneWgIP)
}

func (a *App) upsertBitLaunchServer(r bitlaunch.Server, actorID uint) error {
	if r.IPv4 == controlPlaneIPv4 {
		var s store.MeshServer
		if err := a.Store.DB.Where("role = ?", store.ServerRoleControl).First(&s).Error; err == nil {
			s.BitLaunchID = r.ID
			s.IPv4 = r.IPv4
			s.Region = r.Region
			s.Size = r.Size
			s.Status = r.Status
			return a.Store.DB.Save(&s).Error
		}
	}
	var s store.MeshServer
	err := a.Store.DB.Where("bit_launch_id = ?", r.ID).First(&s).Error
	if err == nil {
		s.IPv4 = r.IPv4
		s.Status = r.Status
		s.Region = r.Region
		s.Size = r.Size
		return a.Store.DB.Save(&s).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	host := strings.ToLower(strings.TrimSpace(r.Name))
	host = strings.ReplaceAll(host, " ", "-")
	if !store.ValidProjectSlug(host) {
		host = "node-" + strings.ToLower(r.ID)
		if len(host) > 20 {
			host = host[:20]
		}
	}
	s = store.MeshServer{
		BitLaunchID: r.ID, Name: r.Name, Hostname: host, Role: store.ServerRoleMesh,
		IPv4: r.IPv4, Region: r.Region, Size: r.Size, Image: r.Image,
		Status: r.Status, CreatedByUserID: actorID,
	}
	return a.Store.DB.Create(&s).Error
}

func (a *App) ensureMeshDNS(hostname, ipv4 string) error {
	fqdn := hostname + ".corp.ihuull.com"
	h, err := corpdns.NormalizeHostname(fqdn)
	if err != nil {
		return err
	}
	ip, err := corpdns.ValidateIPv4(ipv4)
	if err != nil {
		return err
	}
	var rec store.DNSRecord
	err = a.Store.DB.Where("hostname = ?", h).First(&rec).Error
	if err == nil {
		rec.IPv4 = ip
		rec.Enabled = true
		rec.Comment = "malha compute"
		return a.Store.DB.Save(&rec).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return a.Store.DB.Create(&store.DNSRecord{
		Hostname: h, IPv4: ip, Enabled: true, Comment: "malha compute",
	}).Error
}

func meshCloudInit(enrollToken, hostname string) string {
	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y wireguard curl
umask 077
wg genkey | tee /etc/wireguard/private.key | wg pubkey > /etc/wireguard/public.key
PUB=$(cat /etc/wireguard/public.key)
RESP=$(curl -fsS -X POST https://xvpn.ihuull.com/api/servers/enroll \
  -H 'Content-Type: application/json' \
  -d "{\"enroll_token\":\"%s\",\"public_key\":\"$PUB\",\"hostname\":\"%s\"}")
IP=$(printf '%%s' "$RESP" | sed -n 's/.*"assigned_ip":"\([^"]*\)".*/\1/p')
SPUB=$(printf '%%s' "$RESP" | sed -n 's/.*"server_public_key":"\([^"]*\)".*/\1/p')
EP=$(printf '%%s' "$RESP" | sed -n 's/.*"endpoint":"\([^"]*\)".*/\1/p')
cat >/etc/wireguard/wg0.conf <<EOF
[Interface]
Address = $IP
PrivateKey = $(cat /etc/wireguard/private.key)
DNS = 10.66.66.1

[Peer]
PublicKey = $SPUB
Endpoint = $EP
AllowedIPs = 10.66.66.0/24
PersistentKeepalive = 25
EOF
systemctl enable --now wg-quick@wg0
ufw allow from 10.66.66.0/24 to any port 22 proto tcp || true
`, enrollToken, hostname)
}
