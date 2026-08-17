package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// BackupKind é o transporte do destino off-site (PLAN.md §6.19).
type BackupKind string

const (
	BackupKindSFTP    BackupKind = "sftp"
	BackupKindB2      BackupKind = "b2"
	BackupKindS3      BackupKind = "s3"
	BackupKindWebDAV  BackupKind = "webdav"
	BackupKindDrive   BackupKind = "drive"
	BackupKindXDriver BackupKind = "xdriver"
	BackupKindLocal   BackupKind = "local"
)

func (k BackupKind) Valid() bool {
	switch k {
	case BackupKindSFTP, BackupKindB2, BackupKindS3, BackupKindWebDAV, BackupKindDrive, BackupKindXDriver, BackupKindLocal:
		return true
	}
	return false
}

func (k BackupKind) Offsite() bool {
	return k != BackupKindXDriver && k != BackupKindLocal
}

// BackupSettings é a linha singleton (ID=1) do motor off-site.
type BackupSettings struct {
	ID                 uint `gorm:"primaryKey"`
	RetentionDays      int  `gorm:"not null;default:7"`
	IncludeMongo       bool `gorm:"not null;default:true"`
	IncludeMarketplace bool `gorm:"not null;default:true"`
	IncludeGit         bool `gorm:"not null;default:true"`
	IncludeSocial      bool `gorm:"not null;default:true"`
	UpdatedAt          time.Time
}

// BackupDestination é um repositório restic/rclone. Secret nunca no GET.
type BackupDestination struct {
	ID        uint       `gorm:"primaryKey"`
	Name      string     `gorm:"not null"`
	Kind      BackupKind `gorm:"not null;index"`
	Endpoint  string     `gorm:"not null;default:''"`
	Path      string     `gorm:"not null;default:''"`
	Enabled   bool       `gorm:"not null;default:true"`
	Secret    string     `gorm:"type:text" json:"-"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BackupJobStatus é o ciclo de um snapshot.
type BackupJobStatus string

const (
	BackupJobPending BackupJobStatus = "pending"
	BackupJobRunning BackupJobStatus = "running"
	BackupJobOK      BackupJobStatus = "ok"
	BackupJobError   BackupJobStatus = "error"
)

func (s BackupJobStatus) Valid() bool {
	switch s {
	case BackupJobPending, BackupJobRunning, BackupJobOK, BackupJobError:
		return true
	}
	return false
}

// BackupJob é uma execução (dry-run ou snapshot real).
type BackupJob struct {
	ID            uint            `gorm:"primaryKey"`
	DestinationID uint            `gorm:"index;not null"`
	DryRun        bool            `gorm:"not null;default:false"`
	Status        BackupJobStatus `gorm:"not null;default:pending;index"`
	SnapshotID    string
	Bytes         int64
	Error         string
	StartedAt     *time.Time
	FinishedAt    *time.Time
	CreatedAt     time.Time

	Destination *BackupDestination `gorm:"foreignKey:DestinationID"`
}

// SeedBackupSettings cria a linha singleton se faltar.
func SeedBackupSettings(db *gorm.DB) error {
	var row BackupSettings
	err := db.First(&row, 1).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&BackupSettings{
		ID:                 1,
		RetentionDays:      7,
		IncludeMongo:       true,
		IncludeMarketplace: true,
		IncludeGit:         true,
		IncludeSocial:      true,
	}).Error
}
