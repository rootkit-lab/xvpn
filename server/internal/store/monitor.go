package store

import "time"

// MonitorCheck guarda o último resultado de um probe (Fase 67.5 — xmonitor).
type MonitorCheck struct {
	ID        uint      `gorm:"primaryKey"`
	Slug      string    `gorm:"uniqueIndex;not null"`
	Name      string    `gorm:"not null"`
	Status    string    `gorm:"not null"` // ok, warn, critical, skipped
	Summary   string    `gorm:"not null;default:''"`
	Detail    string    `gorm:"not null;default:''"`
	CheckedAt time.Time `gorm:"index"`
}

// MonitorNodeMetric é o último relatório de carga/disco de um peer mesh
// (POST /api/xmonitor/report com token do agente).
type MonitorNodeMetric struct {
	ID           uint      `gorm:"primaryKey"`
	MeshServerID uint      `gorm:"uniqueIndex;not null"`
	Hostname     string    `gorm:"not null;default:''"`
	Load1        float64   `gorm:"not null;default:0"`
	DiskUsedPct  float64   `gorm:"not null;default:0"`
	DiskAvailGB  float64   `gorm:"not null;default:0"`
	ReportedAt   time.Time `gorm:"index"`
}
