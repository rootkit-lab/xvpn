package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/corpdns"
	"github.com/rootkit-lab/xvpn/server/internal/dnsprovider"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type CloudflareAPI interface {
	Accounts() ([]dnsprovider.Account, error)
	ListZones() ([]dnsprovider.Zone, error)
	CreateZone(name, accountID string) (dnsprovider.Zone, error)
	ListRecords(zoneID string) ([]dnsprovider.Record, error)
	CreateRecord(zoneID string, rec dnsprovider.Record) (dnsprovider.Record, error)
	UpdateRecord(zoneID, id string, rec dnsprovider.Record) (dnsprovider.Record, error)
	DeleteRecord(zoneID, id string) error
}

type cfAccountJSON struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	TokenHint string `json:"token_hint"`
}

type publicZoneJSON struct {
	ID          uint     `json:"id"`
	AccountID   uint     `json:"account_id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	NameServers []string `json:"name_servers"`
	Intranet    bool     `json:"intranet"`
}

type publicRecordJSON struct {
	ID           uint   `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	Content      string `json:"content"`
	TTL          int    `json:"ttl"`
	Proxied      bool   `json:"proxied"`
	IntranetIPv4 string `json:"intranet_ipv4,omitempty"`
	Comment      string `json:"comment,omitempty"`
}

type upsertCFAccountRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Token string `json:"token"`
}

type createPublicZoneRequest struct {
	Name      string `json:"name"`
	AccountID uint   `json:"account_id"`
	Intranet  *bool  `json:"intranet"`
}

type upsertPublicRecordRequest struct {
	Type         string `json:"type"`
	Name         string `json:"name"`
	Content      string `json:"content"`
	TTL          int    `json:"ttl"`
	Proxied      bool   `json:"proxied"`
	IntranetIPv4 string `json:"intranet_ipv4"`
	Comment      string `json:"comment"`
}

func cfAccountJSONOf(a store.CloudflareAccount) cfAccountJSON {
	return cfAccountJSON{ID: a.ID, Name: a.Name, Email: a.Email, TokenHint: tokenHint(a.Token)}
}

func zoneJSON(z store.PublicZone) publicZoneJSON {
	ns := z.NameServers
	if ns == nil {
		ns = []string{}
	}
	return publicZoneJSON{ID: z.ID, AccountID: z.AccountID, Name: z.Name, Status: z.Status, NameServers: ns, Intranet: z.Intranet}
}

func (a *App) handleGetPublicDNSSettings(c *gin.Context) {
	var rows []store.CloudflareAccount
	if err := a.Store.DB.Order("id").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]cfAccountJSON, 0, len(rows))
	for _, r := range rows {
		items = append(items, cfAccountJSONOf(r))
	}
	c.JSON(http.StatusOK, gin.H{"accounts": items, "cloudflare": a.cloudflareReady()})
}

func (a *App) handleCreateCloudflareAccount(c *gin.Context) {
	var req upsertCFAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	name, email, token, err := normalizeBitLaunchAccount(req.Name, req.Email, req.Token, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row := store.CloudflareAccount{Name: name, Email: email, Token: token}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "já existe uma conta com este e-mail"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "dns.cf.account.create", email)
	c.JSON(http.StatusCreated, cfAccountJSONOf(row))
}

func (a *App) handleDeleteCloudflareAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	res := a.Store.DB.Delete(&store.CloudflareAccount{}, id)
	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "conta não encontrada"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "dns.cf.account.delete", strconv.FormatUint(id, 10))
	c.Status(http.StatusNoContent)
}

func (a *App) handleListPublicZones(c *gin.Context) {
	var rows []store.PublicZone
	if err := a.Store.DB.Order("name").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]publicZoneJSON, 0, len(rows))
	for _, z := range rows {
		items = append(items, zoneJSON(z))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "cloudflare": a.cloudflareReady()})
}

func (a *App) handleCreatePublicZone(c *gin.Context) {
	var req createPublicZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	name, err := dnsprovider.NormalizeZone(req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cli, accID, err := a.resolveCloudflare(req.AccountID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cadastre uma conta Cloudflare em DNS → Configurações"})
		return
	}
	var acc store.CloudflareAccount
	cfAccountID := ""
	if accID != 0 {
		if err := a.Store.DB.First(&acc, accID).Error; err == nil {
			cfAccountID = acc.AccountCF
		}
	}
	if cfAccountID == "" {
		if list, err := cli.Accounts(); err == nil && len(list) > 0 {
			cfAccountID = list[0].ID
			if acc.ID != 0 {
				acc.AccountCF = cfAccountID
				_ = a.Store.DB.Save(&acc).Error
			}
		}
	}
	remote, err := cli.CreateZone(name, cfAccountID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "falha ao criar zona no Cloudflare"})
		return
	}
	intranet := true
	if req.Intranet != nil {
		intranet = *req.Intranet
	}
	row := store.PublicZone{
		AccountID: accID, Name: name, CloudflareID: remote.ID,
		NameServers: remote.NameServers, Status: remote.Status, Intranet: intranet,
	}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "zona já cadastrada"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "dns.public.zone.create", name)
	c.JSON(http.StatusCreated, zoneJSON(row))
}

func (a *App) handleImportPublicZones(c *gin.Context) {
	imported := 0
	for _, acc := range a.listCloudflareAccounts() {
		cli, _, err := a.resolveCloudflare(acc.ID)
		if err != nil {
			continue
		}
		remote, err := cli.ListZones()
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "falha ao listar zonas Cloudflare (" + acc.Email + ")"})
			return
		}
		for _, z := range remote {
			if _, err := dnsprovider.NormalizeZone(z.Name); err != nil {
				continue
			}
			var existing store.PublicZone
			err := a.Store.DB.Where("name = ?", z.Name).First(&existing).Error
			if err == nil {
				existing.CloudflareID = z.ID
				existing.NameServers = z.NameServers
				existing.Status = z.Status
				existing.AccountID = acc.ID
				_ = a.Store.DB.Save(&existing).Error
				imported++
				continue
			}
			row := store.PublicZone{
				AccountID: acc.ID, Name: z.Name, CloudflareID: z.ID,
				NameServers: z.NameServers, Status: z.Status, Intranet: true,
			}
			if err := a.Store.DB.Create(&row).Error; err == nil {
				imported++
			}
		}
	}
	_ = a.Store.LogAudit(callerUsername(c), "dns.public.import", strconv.Itoa(imported))
	a.handleListPublicZones(c)
}

func (a *App) handleGetPublicZone(c *gin.Context) {
	z, ok := a.loadPublicZone(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, zoneJSON(z))
}

func (a *App) handleListPublicRecords(c *gin.Context) {
	z, ok := a.loadPublicZone(c)
	if !ok {
		return
	}
	var rows []store.PublicRecord
	_ = a.Store.DB.Where("zone_id = ?", z.ID).Order("name").Find(&rows).Error
	items := make([]publicRecordJSON, 0, len(rows))
	for _, r := range rows {
		items = append(items, publicRecordJSON{
			ID: r.ID, Type: r.Type, Name: r.Name, Content: r.Content,
			TTL: r.TTL, Proxied: r.Proxied, IntranetIPv4: r.IntranetIPv4, Comment: r.Comment,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "zone": zoneJSON(z)})
}

func (a *App) handleCreatePublicRecord(c *gin.Context) {
	z, ok := a.loadPublicZone(c)
	if !ok {
		return
	}
	var req upsertPublicRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	rec, err := a.parsePublicRecord(z, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cli, _, err := a.resolveCloudflare(z.AccountID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cadastre uma conta Cloudflare em DNS → Configurações"})
		return
	}
	created, err := cli.CreateRecord(z.CloudflareID, dnsprovider.Record{
		Type: rec.Type, Name: rec.Name, Content: rec.Content, TTL: rec.TTL, Proxied: rec.Proxied,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "falha ao criar registro no Cloudflare"})
		return
	}
	rec.CloudflareID = created.ID
	if created.Name != "" {
		rec.Name = created.Name
	}
	if err := a.Store.DB.Create(&rec).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "registro já existe"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "dns.public.record.create", rec.Name)
	_ = a.pushIntranetDNS(c.Request.Context())
	c.JSON(http.StatusCreated, publicRecordJSON{
		ID: rec.ID, Type: rec.Type, Name: rec.Name, Content: rec.Content,
		TTL: rec.TTL, Proxied: rec.Proxied, IntranetIPv4: rec.IntranetIPv4, Comment: rec.Comment,
	})
}

func (a *App) handleDeletePublicRecord(c *gin.Context) {
	z, ok := a.loadPublicZone(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("rid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var rec store.PublicRecord
	if err := a.Store.DB.Where("id = ? AND zone_id = ?", id, z.ID).First(&rec).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "registro não encontrado"})
		return
	}
	if cli, _, err := a.resolveCloudflare(z.AccountID); err == nil && rec.CloudflareID != "" {
		_ = cli.DeleteRecord(z.CloudflareID, rec.CloudflareID)
	}
	_ = a.Store.DB.Delete(&rec).Error
	_ = a.Store.LogAudit(callerUsername(c), "dns.public.record.delete", rec.Name)
	_ = a.pushIntranetDNS(c.Request.Context())
	c.Status(http.StatusNoContent)
}

func (a *App) handleDNSRecursor(c *gin.Context) {
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, corpdns.RenderRecursor(a.publicSplitSuffixes()))
}

func (a *App) handleMeDNSSuffixes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"suffixes": a.publicSplitSuffixes()})
}

func (a *App) parsePublicRecord(z store.PublicZone, req upsertPublicRecordRequest) (store.PublicRecord, error) {
	rr := strings.ToUpper(strings.TrimSpace(req.Type))
	if !dnsprovider.AllowedType(rr) {
		return store.PublicRecord{}, errMsg("tipo inválido")
	}
	name, err := dnsprovider.NormalizeRecordName(z.Name, req.Name)
	if err != nil {
		return store.PublicRecord{}, err
	}
	if err := dnsprovider.ValidatePublicContent(rr, req.Content); err != nil {
		return store.PublicRecord{}, err
	}
	intra := strings.TrimSpace(req.IntranetIPv4)
	if intra != "" {
		ip, err := corpdns.ValidateIPv4(intra)
		if err != nil {
			return store.PublicRecord{}, err
		}
		intra = ip
	}
	ttl := req.TTL
	if ttl <= 0 {
		ttl = 1
	}
	return store.PublicRecord{
		ZoneID: z.ID, Type: rr, Name: name, Content: strings.TrimSpace(req.Content),
		TTL: ttl, Proxied: req.Proxied, IntranetIPv4: intra, Comment: strings.TrimSpace(req.Comment),
	}, nil
}

func (a *App) loadPublicZone(c *gin.Context) (store.PublicZone, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return store.PublicZone{}, false
	}
	var z store.PublicZone
	if err := a.Store.DB.First(&z, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "zona não encontrada"})
		return store.PublicZone{}, false
	}
	return z, true
}

func (a *App) cloudflareReady() bool {
	if a.Cloudflare != nil {
		return true
	}
	var n int64
	_ = a.Store.DB.Model(&store.CloudflareAccount{}).Count(&n).Error
	return n > 0
}

func (a *App) listCloudflareAccounts() []store.CloudflareAccount {
	var rows []store.CloudflareAccount
	_ = a.Store.DB.Order("id").Find(&rows).Error
	return rows
}

func (a *App) resolveCloudflare(accountID uint) (CloudflareAPI, uint, error) {
	if accountID != 0 {
		var acc store.CloudflareAccount
		if err := a.Store.DB.First(&acc, accountID).Error; err != nil {
			return nil, 0, err
		}
		if a.Cloudflare != nil {
			return a.Cloudflare, acc.ID, nil
		}
		return dnsprovider.New(acc.Token), acc.ID, nil
	}
	if a.Cloudflare != nil {
		return a.Cloudflare, 0, nil
	}
	var acc store.CloudflareAccount
	if err := a.Store.DB.Order("id").First(&acc).Error; err != nil {
		return nil, 0, err
	}
	return dnsprovider.New(acc.Token), acc.ID, nil
}

func (a *App) SeedCloudflareEnvAccount() error {
	if a.Config == nil || strings.TrimSpace(a.Config.CloudflareToken) == "" {
		return nil
	}
	var n int64
	if err := a.Store.DB.Model(&store.CloudflareAccount{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return a.Store.DB.Create(&store.CloudflareAccount{
		Name: "env", Email: "env@cloudflare.local", Token: strings.TrimSpace(a.Config.CloudflareToken),
	}).Error
}

func (a *App) publicSplitSuffixes() []string {
	out := []string{"corp.ihuull.com"}
	var zones []store.PublicZone
	if err := a.Store.DB.Where("intranet = ?", true).Find(&zones).Error; err != nil {
		return out
	}
	seen := map[string]bool{"corp.ihuull.com": true}
	for _, z := range zones {
		if seen[z.Name] {
			continue
		}
		seen[z.Name] = true
		out = append(out, z.Name)
	}
	return out
}

func (a *App) stackIntranetRecords() []corpdns.Record {
	var rows []store.PublicRecord
	if err := a.Store.DB.Where("intranet_ipv4 <> ''").Find(&rows).Error; err != nil {
		return nil
	}
	out := make([]corpdns.Record, 0, len(rows))
	for _, r := range rows {
		out = append(out, corpdns.Record{Hostname: r.Name, IPv4: r.IntranetIPv4})
	}
	return out
}
