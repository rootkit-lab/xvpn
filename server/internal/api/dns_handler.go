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
	"github.com/rootkit-lab/xvpn/server/internal/corpdns"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type dnsRecordJSON struct {
	ID       uint   `json:"id"`
	Hostname string `json:"hostname"`
	IPv4     string `json:"ipv4"`
	System   bool   `json:"system"`
	Enabled  bool   `json:"enabled"`
	Comment  string `json:"comment"`
}

type dnsHostJSON struct {
	Hostname string `json:"hostname"`
	IPv4     string `json:"ipv4"`
}

type dnsResponse struct {
	Listen      string          `json:"listen"`
	Listening   bool            `json:"listening"`
	QueryOK     bool            `json:"query_ok"`
	QueryDetail string          `json:"query_detail,omitempty"`
	Forwarders  []string        `json:"forwarders"`
	CacheSize   int             `json:"cache_size"`
	CatchAll    bool            `json:"catch_all"`
	LastApplied *time.Time      `json:"last_applied_at,omitempty"`
	LastError   string          `json:"last_apply_error,omitempty"`
	Records     []dnsRecordJSON `json:"records"`
}

type updateDNSSettingsRequest struct {
	Forwarders *string `json:"forwarders"`
	CacheSize  *int    `json:"cache_size"`
	CatchAll   *bool   `json:"catch_all"`
}

type upsertDNSRecordRequest struct {
	Hostname string `json:"hostname"`
	IPv4     string `json:"ipv4"`
	Enabled  *bool  `json:"enabled"`
	Comment  string `json:"comment"`
}

func (a *App) handleGetDNS(c *gin.Context) {
	c.JSON(http.StatusOK, a.dnsResponse())
}

func (a *App) handleUpdateDNSSettings(c *gin.Context) {
	var req updateDNSSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	settings, err := a.loadOrInitDNSSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if req.Forwarders != nil {
		fwd, err := corpdns.ParseForwarders(*req.Forwarders)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		settings.Forwarders = strings.Join(fwd, ",")
	}
	if req.CacheSize != nil {
		if err := corpdns.ValidateCacheSize(*req.CacheSize); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		settings.CacheSize = *req.CacheSize
	}
	if req.CatchAll != nil {
		settings.CatchAll = *req.CatchAll
	}
	if err := a.Store.DB.Save(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao persistir"})
		return
	}
	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "dns.settings", "")
	c.JSON(http.StatusOK, a.dnsResponse())
}

func (a *App) handleCreateDNSRecord(c *gin.Context) {
	var req upsertDNSRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hostname e ipv4 são obrigatórios"})
		return
	}
	rec, err := a.parseDNSRecord(req, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Store.DB.Create(&rec).Error; err != nil {
		if isUniqueErr(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "já existe um registro com esse hostname"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "dns.record.create", rec.Hostname)
	c.JSON(http.StatusCreated, a.dnsResponse())
}

func (a *App) handleUpdateDNSRecord(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var rec store.DNSRecord
	if err := a.Store.DB.First(&rec, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "registro não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var req upsertDNSRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	next, err := a.parseDNSRecord(req, rec.System)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if rec.System && next.Hostname != rec.Hostname {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hostname de registro do sistema não pode mudar"})
		return
	}
	rec.Hostname = next.Hostname
	rec.IPv4 = next.IPv4
	rec.Enabled = next.Enabled
	rec.Comment = next.Comment
	if err := a.Store.DB.Save(&rec).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "dns.record.update", rec.Hostname)
	c.JSON(http.StatusOK, a.dnsResponse())
}

func (a *App) handleDeleteDNSRecord(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var rec store.DNSRecord
	if err := a.Store.DB.First(&rec, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "registro não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if rec.System {
		c.JSON(http.StatusForbidden, gin.H{"error": "registro do sistema não pode ser apagado"})
		return
	}
	if err := a.Store.DB.Delete(&rec).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "dns.record.delete", rec.Hostname)
	c.JSON(http.StatusOK, a.dnsResponse())
}

func (a *App) handleApplyDNS(c *gin.Context) {
	if a.UserProvisioner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provisionamento privilegiado não configurado neste servidor"})
		return
	}
	settings, err := a.loadOrInitDNSSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var records []store.DNSRecord
	if err := a.Store.DB.Where("enabled = ?", true).Order("hostname ASC").Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	fwd, err := corpdns.ParseForwarders(settings.Forwarders)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload := corpdns.ApplyPayload{
		Forwarders: fwd,
		CacheSize:  settings.CacheSize,
		CatchAll:   settings.CatchAll,
		Records:    make([]corpdns.Record, 0, len(records)),
	}
	for _, r := range records {
		payload.Records = append(payload.Records, corpdns.Record{Hostname: r.Hostname, IPv4: r.IPv4})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	applyErr := a.UserProvisioner.ApplyDNS(c.Request.Context(), string(raw))
	now := time.Now()
	settings.LastAppliedAt = &now
	if applyErr != nil {
		settings.LastApplyError = provisionerErrMsg(applyErr)
		_ = a.Store.DB.Save(&settings).Error
		c.JSON(http.StatusInternalServerError, gin.H{"error": settings.LastApplyError})
		return
	}
	settings.LastApplyError = ""
	if err := a.Store.DB.Save(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "aplicado, mas falha ao persistir estado"})
		return
	}
	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "dns.apply", "")
	c.JSON(http.StatusOK, a.dnsResponse())
}

func (a *App) parseDNSRecord(req upsertDNSRecordRequest, system bool) (store.DNSRecord, error) {
	h, err := corpdns.NormalizeHostname(req.Hostname)
	if err != nil {
		return store.DNSRecord{}, err
	}
	ip, err := corpdns.ValidateIPv4(req.IPv4)
	if err != nil {
		return store.DNSRecord{}, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return store.DNSRecord{
		Hostname: h,
		IPv4:     ip,
		System:   system,
		Enabled:  enabled,
		Comment:  strings.TrimSpace(req.Comment),
	}, nil
}

func (a *App) loadOrInitDNSSettings() (store.DNSSettings, error) {
	if err := store.SeedIntranetDNS(a.Store.DB); err != nil {
		return store.DNSSettings{}, err
	}
	var s store.DNSSettings
	if err := a.Store.DB.First(&s, 1).Error; err != nil {
		return s, err
	}
	return s, nil
}

func (a *App) enabledIntranetHosts() []dnsHostJSON {
	var records []store.DNSRecord
	if err := a.Store.DB.Where("enabled = ?", true).Order("hostname ASC").Find(&records).Error; err != nil {
		out := make([]dnsHostJSON, 0, len(store.DefaultIntranetHosts))
		for _, r := range store.DefaultIntranetHosts {
			out = append(out, dnsHostJSON{Hostname: r.Hostname, IPv4: r.IPv4})
		}
		return out
	}
	out := make([]dnsHostJSON, 0, len(records))
	for _, r := range records {
		out = append(out, dnsHostJSON{Hostname: r.Hostname, IPv4: r.IPv4})
	}
	return out
}

func (a *App) dnsResponse() dnsResponse {
	settings, err := a.loadOrInitDNSSettings()
	if err != nil {
		return dnsResponse{Listen: corpdns.ListenIP + ":53"}
	}
	fwd, _ := corpdns.ParseForwarders(settings.Forwarders)
	var records []store.DNSRecord
	_ = a.Store.DB.Order("system DESC, hostname ASC").Find(&records).Error
	items := make([]dnsRecordJSON, 0, len(records))
	for _, r := range records {
		items = append(items, dnsRecordJSON{
			ID: r.ID, Hostname: r.Hostname, IPv4: r.IPv4,
			System: r.System, Enabled: r.Enabled, Comment: r.Comment,
		})
	}
	listening, queryOK, detail := probeIntranetDNS()
	return dnsResponse{
		Listen:      corpdns.ListenIP + ":53",
		Listening:   listening,
		QueryOK:     queryOK,
		QueryDetail: detail,
		Forwarders:  fwd,
		CacheSize:   settings.CacheSize,
		CatchAll:    settings.CatchAll,
		LastApplied: settings.LastAppliedAt,
		LastError:   settings.LastApplyError,
		Records:     items,
	}
}

func probeIntranetDNS() (listening, queryOK bool, detail string) {
	conn, err := net.DialTimeout("udp", net.JoinHostPort(corpdns.ListenIP, "53"), 400*time.Millisecond)
	if err != nil {
		return false, false, err.Error()
	}
	_ = conn.Close()
	listening = true
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 800 * time.Millisecond}
			return d.DialContext(ctx, "udp", net.JoinHostPort(corpdns.ListenIP, "53"))
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ips, err := r.LookupIP(ctx, "ip4", "corp.ihuull.com")
	if err != nil {
		return true, false, err.Error()
	}
	for _, ip := range ips {
		if ip.String() == corpdns.ListenIP {
			return true, true, corpdns.ListenIP
		}
	}
	return true, false, "corp.ihuull.com não apontou para " + corpdns.ListenIP
}

func isUniqueErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique") || strings.Contains(s, "duplicate")
}
