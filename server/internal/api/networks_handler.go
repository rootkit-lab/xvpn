package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/provision"
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
)

type overlayNetworkJSON struct {
	ID     uint   `json:"id"`
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	CIDR   string `json:"cidr"`
	System bool   `json:"system"`
	Exit   bool   `json:"exit"`
}

type networkMemberJSON struct {
	ID          uint   `json:"id"`
	NetworkID   uint   `json:"network_id"`
	SubjectKind string `json:"subject_kind"`
	SubjectID   uint   `json:"subject_id"`
	Role        string `json:"role"`
}

type networkRuleJSON struct {
	ID           uint   `json:"id"`
	Slug         string `json:"slug"`
	SrcNetworkID uint   `json:"src_network_id"`
	DstNetworkID uint   `json:"dst_network_id"`
	Action       string `json:"action"`
	Proto        string `json:"proto"`
	Ports        string `json:"ports"`
	System       bool   `json:"system"`
}

type createNetworkRequest struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	CIDR       string `json:"cidr"`
	Exit       bool   `json:"exit"`
	CorpAccess bool   `json:"corp_access"`
}

type createMemberRequest struct {
	SubjectKind string `json:"subject_kind"`
	SubjectID   uint   `json:"subject_id"`
	Role        string `json:"role"`
}

type createRuleRequest struct {
	Slug         string `json:"slug"`
	SrcNetworkID uint   `json:"src_network_id"`
	DstNetworkID uint   `json:"dst_network_id"`
	Action       string `json:"action"`
	Proto        string `json:"proto"`
	Ports        string `json:"ports"`
}

func (a *App) handleListNetworks(c *gin.Context) {
	var nets []store.OverlayNetwork
	if err := a.Store.DB.Order("id").Find(&nets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var members []store.NetworkMember
	if err := a.Store.DB.Order("id").Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var rules []store.NetworkRule
	if err := a.Store.DB.Order("id").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":   overlayNetsJSON(nets),
		"members": overlayMembersJSON(members),
		"rules":   overlayRulesJSON(rules),
		"pool":    store.UsersPoolCIDR,
	})
}

func (a *App) handleCreateNetwork(c *gin.Context) {
	var req createNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if !store.ValidProjectSlug(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug inválido"})
		return
	}
	if slug == "infra" || slug == "users" {
		c.JSON(http.StatusConflict, gin.H{"error": "slug reservado"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = slug
	}
	var existing []store.OverlayNetwork
	if err := a.Store.DB.Find(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	cidr := strings.TrimSpace(req.CIDR)
	if cidr == "" {
		next, err := store.NextCustomCIDR(existing)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		cidr = next
	}
	if err := store.ValidateOverlayCIDR(store.NetworkKindCustom, cidr, existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row := store.OverlayNetwork{
		Slug: slug, Name: name, Kind: store.NetworkKindCustom,
		CIDR: cidr, System: false, Exit: req.Exit,
	}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "rede já existe"})
		return
	}
	if req.CorpAccess {
		if err := a.addCorpRulesFrom(row.ID); err != nil {
			_ = a.Store.DB.Delete(&row).Error
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
	}
	if err := a.applyOverlayOrRollback(c.Request.Context(), func() {
		_ = a.Store.DB.Where("src_network_id = ?", row.ID).Delete(&store.NetworkRule{}).Error
		_ = a.Store.DB.Delete(&row).Error
	}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "network.create", slug)
	c.JSON(http.StatusCreated, overlayNetJSON(row))
}

func (a *App) handleDeleteNetwork(c *gin.Context) {
	n, ok := a.loadOverlayNetwork(c)
	if !ok {
		return
	}
	if n.System {
		c.JSON(http.StatusConflict, gin.H{"error": "rede de sistema não apaga"})
		return
	}
	var peers int64
	if err := a.Store.DB.Model(&store.Device{}).Where("network_id = ?", n.ID).Count(&peers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if peers > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "rede ainda tem peers"})
		return
	}
	var members []store.NetworkMember
	_ = a.Store.DB.Where("network_id = ?", n.ID).Find(&members).Error
	var rules []store.NetworkRule
	_ = a.Store.DB.Where("src_network_id = ? OR dst_network_id = ?", n.ID, n.ID).Find(&rules).Error
	if err := a.Store.DB.Where("network_id = ?", n.ID).Delete(&store.NetworkMember{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.Store.DB.Where("src_network_id = ? OR dst_network_id = ?", n.ID, n.ID).Delete(&store.NetworkRule{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.Store.DB.Delete(&n).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.applyOverlayOrRollback(c.Request.Context(), func() {
		saved := n
		saved.ID = 0
		if a.Store.DB.Create(&saved).Error != nil {
			return
		}
		for i := range members {
			members[i].ID = 0
			members[i].NetworkID = saved.ID
			_ = a.Store.DB.Create(&members[i]).Error
		}
		for i := range rules {
			rules[i].ID = 0
			if rules[i].SrcNetworkID == n.ID {
				rules[i].SrcNetworkID = saved.ID
			}
			if rules[i].DstNetworkID == n.ID {
				rules[i].DstNetworkID = saved.ID
			}
			_ = a.Store.DB.Create(&rules[i]).Error
		}
	}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "network.delete", n.Slug)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) handleAddNetworkMember(c *gin.Context) {
	n, ok := a.loadOverlayNetwork(c)
	if !ok {
		return
	}
	var req createMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if !store.ValidNetworkSubject(req.SubjectKind) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sujeito inválido"})
		return
	}
	if req.SubjectID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subject_id obrigatório"})
		return
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = store.NetworkMemberRole
	}
	if role != store.NetworkMemberRole && role != store.NetworkOperator {
		c.JSON(http.StatusBadRequest, gin.H{"error": "papel inválido"})
		return
	}
	if err := a.assertMemberSubject(req.SubjectKind, req.SubjectID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row := store.NetworkMember{
		NetworkID: n.ID, SubjectKind: req.SubjectKind, SubjectID: req.SubjectID, Role: role,
	}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "já é membro"})
		return
	}
	if err := a.applyOverlayOrRollback(c.Request.Context(), func() {
		_ = a.Store.DB.Delete(&row).Error
	}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "network.member", n.Slug)
	c.JSON(http.StatusCreated, overlayMemberJSON(row))
}

func (a *App) handleDeleteNetworkMember(c *gin.Context) {
	n, ok := a.loadOverlayNetwork(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("mid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var row store.NetworkMember
	if err := a.Store.DB.Where("id = ? AND network_id = ?", id, n.ID).First(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "membro não encontrado"})
		return
	}
	if err := a.Store.DB.Delete(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.applyOverlayOrRollback(c.Request.Context(), func() {
		row.ID = 0
		_ = a.Store.DB.Create(&row).Error
	}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) handleCreateNetworkRule(c *gin.Context) {
	var req createRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if !store.ValidProjectSlug(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug inválido"})
		return
	}
	if req.SrcNetworkID == 0 || req.DstNetworkID == 0 || req.SrcNetworkID == req.DstNetworkID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "redes de origem e destino inválidas"})
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = store.NetworkRuleAllow
	}
	if action != store.NetworkRuleAllow {
		c.JSON(http.StatusBadRequest, gin.H{"error": "só action allow"})
		return
	}
	proto := strings.ToLower(strings.TrimSpace(req.Proto))
	if proto == "" {
		proto = "any"
	}
	if !store.ValidRuleProto(proto) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "proto inválido"})
		return
	}
	if _, err := store.ParseRulePorts(req.Ports); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Store.DB.First(&store.OverlayNetwork{}, req.SrcNetworkID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rede origem inexistente"})
		return
	}
	if err := a.Store.DB.First(&store.OverlayNetwork{}, req.DstNetworkID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rede destino inexistente"})
		return
	}
	row := store.NetworkRule{
		Slug: slug, SrcNetworkID: req.SrcNetworkID, DstNetworkID: req.DstNetworkID,
		Action: action, Proto: proto, Ports: strings.TrimSpace(req.Ports),
	}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "regra já existe"})
		return
	}
	if err := a.applyOverlayOrRollback(c.Request.Context(), func() {
		_ = a.Store.DB.Delete(&row).Error
	}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "network.rule", slug)
	c.JSON(http.StatusCreated, overlayRuleJSON(row))
}

func (a *App) handleDeleteNetworkRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("rid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var row store.NetworkRule
	if err := a.Store.DB.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "regra não encontrada"})
		return
	}
	if row.System {
		c.JSON(http.StatusConflict, gin.H{"error": "regra de sistema não apaga"})
		return
	}
	if err := a.Store.DB.Delete(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.applyOverlayOrRollback(c.Request.Context(), func() {
		row.ID = 0
		_ = a.Store.DB.Create(&row).Error
	}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) loadOverlayNetwork(c *gin.Context) (store.OverlayNetwork, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return store.OverlayNetwork{}, false
	}
	var n store.OverlayNetwork
	if err := a.Store.DB.First(&n, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rede não encontrada"})
		return store.OverlayNetwork{}, false
	}
	return n, true
}

func (a *App) assertMemberSubject(kind string, id uint) error {
	switch kind {
	case store.NetworkSubjectUser:
		return a.Store.DB.First(&store.User{}, id).Error
	case store.NetworkSubjectDevice:
		return a.Store.DB.First(&store.Device{}, id).Error
	case store.NetworkSubjectMeshServer:
		return a.Store.DB.First(&store.MeshServer{}, id).Error
	}
	return errors.New("sujeito inválido")
}

func (a *App) addCorpRulesFrom(srcID uint) error {
	infra, err := store.NetworkByKind(a.Store.DB, store.NetworkKindInfra)
	if err != nil {
		return err
	}
	var src store.OverlayNetwork
	if err := a.Store.DB.First(&src, srcID).Error; err != nil {
		return err
	}
	rules := []store.NetworkRule{
		{Slug: src.Slug + "-corp", SrcNetworkID: srcID, DstNetworkID: infra.ID, Action: store.NetworkRuleAllow, Proto: "tcp", Ports: "443,53"},
		{Slug: src.Slug + "-dns", SrcNetworkID: srcID, DstNetworkID: infra.ID, Action: store.NetworkRuleAllow, Proto: "udp", Ports: "53"},
	}
	for _, r := range rules {
		if err := a.Store.DB.Create(&r).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *App) allocateInNetwork(net store.OverlayNetwork) (string, error) {
	var devices []store.Device
	if err := a.Store.DB.Find(&devices).Error; err != nil {
		return "", err
	}
	used := make([]string, 0, len(devices)+1)
	for _, d := range devices {
		used = append(used, d.AllowedIP)
	}
	if net.Kind == store.NetworkKindInfra {
		used = append(used, store.ControlPlaneIP+"/32")
	}
	return wireguard.AllocateIP(net.CIDR, used)
}

func (a *App) overlayIPAllowed(ip net.IP) bool {
	return store.OverlayContainsIP(a.Store.DB, ip)
}

func (a *App) ApplyOverlayFirewall(ctx context.Context) error {
	return a.applyOverlayFirewall(ctx)
}

func (a *App) applyOverlayOrRollback(ctx context.Context, rollback func()) error {
	err := a.applyOverlayFirewall(ctx)
	if err != nil && rollback != nil {
		rollback()
	}
	return err
}

func (a *App) applyOverlayFirewall(ctx context.Context) error {
	if a.UserProvisioner == nil {
		return errors.New("provisionador overlay ausente")
	}
	var nets []store.OverlayNetwork
	if err := a.Store.DB.Find(&nets).Error; err != nil {
		return err
	}
	var rules []store.NetworkRule
	if err := a.Store.DB.Find(&rules).Error; err != nil {
		return err
	}
	byID := map[uint]store.OverlayNetwork{}
	spec := provision.OverlaySpec{}
	for _, n := range nets {
		byID[n.ID] = n
		spec.Networks = append(spec.Networks, provision.OverlayNetSpec{ID: n.ID, CIDR: n.CIDR, Exit: n.Exit})
	}
	for _, r := range rules {
		src, okS := byID[r.SrcNetworkID]
		dst, okD := byID[r.DstNetworkID]
		if !okS || !okD {
			continue
		}
		ports, err := store.ParseRulePorts(r.Ports)
		if err != nil {
			return err
		}
		spec.Rules = append(spec.Rules, provision.OverlayRuleSpec{
			SrcCIDR: src.CIDR, DstCIDR: dst.CIDR, Action: r.Action, Proto: r.Proto, Ports: ports,
		})
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	return a.UserProvisioner.ApplyOverlay(ctx, string(raw))
}

func overlayNetsJSON(in []store.OverlayNetwork) []overlayNetworkJSON {
	out := make([]overlayNetworkJSON, 0, len(in))
	for _, n := range in {
		out = append(out, overlayNetJSON(n))
	}
	return out
}

func overlayNetJSON(n store.OverlayNetwork) overlayNetworkJSON {
	return overlayNetworkJSON{
		ID: n.ID, Slug: n.Slug, Name: n.Name, Kind: n.Kind, CIDR: n.CIDR, System: n.System, Exit: n.Exit,
	}
}

func overlayMembersJSON(in []store.NetworkMember) []networkMemberJSON {
	out := make([]networkMemberJSON, 0, len(in))
	for _, m := range in {
		out = append(out, overlayMemberJSON(m))
	}
	return out
}

func overlayMemberJSON(m store.NetworkMember) networkMemberJSON {
	return networkMemberJSON{
		ID: m.ID, NetworkID: m.NetworkID, SubjectKind: m.SubjectKind, SubjectID: m.SubjectID, Role: m.Role,
	}
}

func overlayRulesJSON(in []store.NetworkRule) []networkRuleJSON {
	out := make([]networkRuleJSON, 0, len(in))
	for _, r := range in {
		out = append(out, overlayRuleJSON(r))
	}
	return out
}

func overlayRuleJSON(r store.NetworkRule) networkRuleJSON {
	return networkRuleJSON{
		ID: r.ID, Slug: r.Slug, SrcNetworkID: r.SrcNetworkID, DstNetworkID: r.DstNetworkID,
		Action: r.Action, Proto: r.Proto, Ports: r.Ports, System: r.System,
	}
}
