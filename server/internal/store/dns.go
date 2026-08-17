package store

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// DNSSettings é a linha singleton (ID=1) do resolvedor da intranet
// (dnsmasq em 10.66.66.1:53). Bind e zona não são editáveis — PLAN.md §5.
type DNSSettings struct {
	ID             uint `gorm:"primaryKey"`
	Forwarders     string
	CacheSize      int
	CatchAll       bool
	LastAppliedAt  *time.Time
	LastApplyError string
}

// DNSRecord é um A em *.corp.ihuull.com. System=true não pode ser apagado
// (apex e apps oficiais). IPv4 só na sub-rede da VPN.
type DNSRecord struct {
	ID        uint   `gorm:"primaryKey"`
	Hostname  string `gorm:"uniqueIndex;not null"`
	IPv4      string `gorm:"not null"`
	System    bool   `gorm:"not null;default:false"`
	Enabled   bool   `gorm:"not null;default:true"`
	Comment   string `gorm:"not null;default:''"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DefaultIntranetHosts são os A oficiais da zona corp (PLAN.md §5.2).
var DefaultIntranetHosts = []DNSRecord{
	{Hostname: "corp.ihuull.com", IPv4: "10.66.66.1", System: true, Enabled: true, Comment: "apex da intranet"},
	{Hostname: "xchat.corp.ihuull.com", IPv4: "10.66.66.1", System: true, Enabled: true, Comment: "messenger"},
	{Hostname: "xgroup.corp.ihuull.com", IPv4: "10.66.66.1", System: true, Enabled: true, Comment: "rede social"},
	{Hostname: "xdriver.corp.ihuull.com", IPv4: "10.66.66.1", System: true, Enabled: true, Comment: "drive nativo"},
	{Hostname: "xadmin.corp.ihuull.com", IPv4: "10.66.66.1", System: true, Enabled: true, Comment: "console"},
	{Hostname: "xgit.corp.ihuull.com", IPv4: "10.66.66.1", System: true, Enabled: true, Comment: "forge git"},
	{Hostname: "xcodespaces.corp.ihuull.com", IPv4: "10.66.66.1", System: true, Enabled: true, Comment: "ide monaco"},
}

// SeedIntranetDNS cria settings + records oficiais se a tabela estiver vazia.
func SeedIntranetDNS(db *gorm.DB) error {
	var settings DNSSettings
	err := db.First(&settings, 1).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("lendo dns_settings: %w", err)
		}
		settings = DNSSettings{ID: 1, Forwarders: "8.8.8.8,1.1.1.1", CacheSize: 1000, CatchAll: true}
		if err := db.Create(&settings).Error; err != nil {
			return fmt.Errorf("criando dns_settings: %w", err)
		}
	}
	for _, rec := range DefaultIntranetHosts {
		var existing DNSRecord
		err := db.Where("hostname = ?", rec.Hostname).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("lendo dns_record %s: %w", rec.Hostname, err)
		}
		if err := db.Create(&rec).Error; err != nil {
			return fmt.Errorf("criando dns_record %s: %w", rec.Hostname, err)
		}
	}
	return nil
}
